//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// scratchcontainer - A basic container runtime demo
// Usage: scratchcontainer run <rootfs> <command> [args...]
func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: scratchcontainer run <rootfs> <command> [args...]")
		fmt.Println("Example: scratchcontainer run /path/to/rootfs /bin/bash")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if err := run(); err != nil {
			log.Fatalf("Error running container: %v", err)
		}
	case "child":
		if err := child(); err != nil {
			log.Fatalf("Error in child process: %v", err)
		}
	default:
		fmt.Println("Usage: scratchcontainer run <rootfs> <command> [args...]")
		os.Exit(1)
	}
}

func run() error {
	rootfs := os.Args[2]
	command := os.Args[3:]
	
	fmt.Printf("Running %v in container with rootfs: %s\n", command, rootfs)

	cmd := exec.Command("/proc/self/exe", append([]string{"child", rootfs}, command...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	return cmd.Run()
}

func child() error {
	rootfs := os.Args[2]
	command := os.Args[3:]
	
	fmt.Printf("Running %v in container\n", command)

	// Setup cgroups
	if err := setupCgroups(); err != nil {
		return fmt.Errorf("failed to setup cgroups: %w", err)
	}

	// Set hostname
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Change root
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("failed to chroot to %s: %w", rootfs, err)
	}

	// Change directory to new root
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir to root: %w", err)
	}

	// Mount proc
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}
	defer func() {
		if err := syscall.Unmount("proc", 0); err != nil {
			log.Printf("Warning: failed to unmount proc: %v", err)
		}
	}()

	// Execute the command
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func setupCgroups() error {
	cgroups := "/sys/fs/cgroup/"
	pid := os.Getpid()
	containerName := "scratchcontainer"

	// Setup memory cgroup
	memory := filepath.Join(cgroups, "memory")
	memoryPath := filepath.Join(memory, containerName)
	
	if err := os.MkdirAll(memoryPath, 0755); err != nil {
		return fmt.Errorf("failed to create memory cgroup: %w", err)
	}

	// Set memory limit (2GB)
	if err := os.WriteFile(filepath.Join(memoryPath, "memory.limit_in_bytes"), []byte("2147483648"), 0700); err != nil {
		return fmt.Errorf("failed to set memory limit: %w", err)
	}

	// Setup CPU cgroup
	cpu := filepath.Join(cgroups, "cpu,cpuacct")
	cpuPath := filepath.Join(cpu, containerName)
	
	if err := os.MkdirAll(cpuPath, 0755); err != nil {
		return fmt.Errorf("failed to create cpu cgroup: %w", err)
	}

	// Set CPU quota (20% of CPU)
	if err := os.WriteFile(filepath.Join(cpuPath, "cpu.cfs_quota_us"), []byte("200000"), 0700); err != nil {
		return fmt.Errorf("failed to set cpu quota: %w", err)
	}

	// Add process to cgroups
	pidStr := strconv.Itoa(pid)
	
	if err := os.WriteFile(filepath.Join(memoryPath, "cgroup.procs"), []byte(pidStr), 0700); err != nil {
		return fmt.Errorf("failed to add process to memory cgroup: %w", err)
	}

	if err := os.WriteFile(filepath.Join(cpuPath, "cgroup.procs"), []byte(pidStr), 0700); err != nil {
		return fmt.Errorf("failed to add process to cpu cgroup: %w", err)
	}

	// Enable auto-cleanup
	if err := os.WriteFile(filepath.Join(memoryPath, "notify_on_release"), []byte("1"), 0700); err != nil {
		log.Printf("Warning: failed to enable memory cgroup cleanup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cpuPath, "notify_on_release"), []byte("1"), 0700); err != nil {
		log.Printf("Warning: failed to enable cpu cgroup cleanup: %v", err)
	}

	return nil
}
