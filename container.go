//go:build linux

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// run is executed in the parent process. It creates a new child process
// with new namespaces, configures optional port publishing, and waits for it.
func run() error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	publish := fs.String("publish", "", "publish host:container port mapping")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("usage: scratchcontainer run [-publish hostPort:containerPort] <rootfs> <command> [args...]")
	}

	rootfs := fs.Arg(0)
	command := fs.Args()[1:]

	// Create a pipe for parent-to-child synchronization. The child waits until
	// the parent has configured networking before continuing.
	syncReader, syncWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create sync pipe: %w", err)
	}
	defer syncWriter.Close()

	cmd := exec.Command("/proc/self/exe", append([]string{"child", rootfs}, command...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass the read side of the pipe into the child process as fd 3.
	cmd.ExtraFiles = []*os.File{syncReader}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start child process: %w", err)
	}

	syncReader.Close()

	var forwarder *portForwarder
	var cleanup func()
	if *publish != "" {
		// Set up networking and host port publishing before allowing the child to continue.
		_, containerIP, hostIface, err := setupNetwork(cmd.Process.Pid, *publish)
		if err != nil {
			cmd.Process.Kill()
			return err
		}

		cleanup = func() {
			if err := deleteInterface(hostIface); err != nil {
				log.Printf("warning: failed to delete host veth %s: %v", hostIface, err)
			}
		}

		forwarder, err = startPortForward(containerIP, *publish)
		if err != nil {
			cleanup()
			cmd.Process.Kill()
			return err
		}
	}

	// Signal the child process that networking is configured and it can proceed.
	if _, err := syncWriter.Write([]byte{1}); err != nil {
		if forwarder != nil {
			forwarder.Close()
		}
		if cleanup != nil {
			cleanup()
		}
		return fmt.Errorf("failed to signal child: %w", err)
	}

	// Wait for the child process to finish and then clean up resources.
	if err := cmd.Wait(); err != nil {
		if forwarder != nil {
			forwarder.Close()
		}
		if cleanup != nil {
			cleanup()
		}
		return err
	}

	if forwarder != nil {
		forwarder.Close()
	}
	if cleanup != nil {
		cleanup()
	}

	return nil
}

// child is executed inside the newly-created namespaces.
// It sets up cgroups, hostname, chroot, and starts the requested command.
func child() error {
	if err := waitForParent(); err != nil {
		return err
	}

	if len(os.Args) < 4 {
		return fmt.Errorf("usage: scratchcontainer child <rootfs> <command> [args...]")
	}

	rootfs := os.Args[2]
	command := os.Args[3:]

	fmt.Printf("Running %v in container\n", command)

	// Apply control group limits for this process tree.
	if err := setupCgroups(); err != nil {
		return fmt.Errorf("failed to setup cgroups: %w", err)
	}

	// Set a simple hostname inside the new UTS namespace.
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Change root filesystem into the provided rootfs path.
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("failed to chroot to %s: %w", rootfs, err)
	}

	// After chroot, change directory to the new root so relative paths work.
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir to root: %w", err)
	}

	// Mount proc inside the container so /proc exposes process information.
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}
	defer func() {
		if err := syscall.Unmount("proc", 0); err != nil {
			log.Printf("Warning: failed to unmount proc: %v", err)
		}
	}()

	// Execute the requested command within the container context.
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}