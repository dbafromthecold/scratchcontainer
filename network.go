//go:build linux

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// portForwarder accepts incoming host connections and forwards them to the container.
type portForwarder struct {
	listener net.Listener
	quit     chan struct{}
}

// startPortForward listens on the requested host port and forwards traffic to the container.
func startPortForward(containerIP, publish string) (*portForwarder, error) {
	hostPort, containerPort, err := parsePublish(publish)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", hostPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on host port %d: %w", hostPort, err)
	}

	pf := &portForwarder{listener: listener, quit: make(chan struct{})}
	containerAddr := fmt.Sprintf("%s:%d", containerIP, containerPort)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-pf.quit:
					return
				default:
					log.Printf("warning: accept error: %v", err)
					continue
				}
			}

			go func(c net.Conn) {
				defer c.Close()
				target, err := net.Dial("tcp", containerAddr)
				if err != nil {
					log.Printf("warning: failed to dial container %s: %v", containerAddr, err)
					return
				}
				defer target.Close()

				go io.Copy(c, target)
				io.Copy(target, c)
			}(conn)
		}
	}()

	return pf, nil
}

func (pf *portForwarder) Close() {
	close(pf.quit)
	pf.listener.Close()
}

// parsePublish validates the publish argument and returns the two ports.
func parsePublish(publish string) (int, int, error) {
	parts := strings.Split(publish, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("publish format must be hostPort:containerPort")
	}

	hostPort, err := strconv.Atoi(parts[0])
	if err != nil || hostPort <= 0 || hostPort > 65535 {
		return 0, 0, fmt.Errorf("invalid host port: %s", parts[0])
	}

	containerPort, err := strconv.Atoi(parts[1])
	if err != nil || containerPort <= 0 || containerPort > 65535 {
		return 0, 0, fmt.Errorf("invalid container port: %s", parts[1])
	}

	return hostPort, containerPort, nil
}

// setupNetwork creates a veth pair and configures networking for the child netns.
func setupNetwork(pid int, publish string) (string, string, string, error) {
	hostPort, _, err := parsePublish(publish)
	if err != nil {
		return "", "", "", err
	}

	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return "", "", "", fmt.Errorf("ip command not found: %w", err)
	}

	// Host and container IP are chosen from a private /24 subnet based on PID.
	hostIface := fmt.Sprintf("veth%d", pid)
	containerIP := fmt.Sprintf("172.22.%d.2", pid%240+1)
	hostIP := fmt.Sprintf("172.22.%d.1", pid%240+1)

	if err := runCommand(ipPath, "link", "add", hostIface, "type", "veth", "peer", "name", "eth0"); err != nil {
		return "", "", "", fmt.Errorf("failed to create veth pair: %w", err)
	}

	if err := runCommand(ipPath, "link", "set", "eth0", "netns", strconv.Itoa(pid)); err != nil {
		deleteInterface(hostIface)
		return "", "", "", fmt.Errorf("failed to move container veth into netns: %w", err)
	}

	if err := runCommand(ipPath, "addr", "add", fmt.Sprintf("%s/24", hostIP), "dev", hostIface); err != nil {
		deleteInterface(hostIface)
		return "", "", "", fmt.Errorf("failed to assign host IP: %w", err)
	}

	if err := runCommand(ipPath, "link", "set", hostIface, "up"); err != nil {
		deleteInterface(hostIface)
		return "", "", "", fmt.Errorf("failed to bring up host veth: %w", err)
	}

	if err := configureContainerInterface(pid, hostIP, containerIP); err != nil {
		deleteInterface(hostIface)
		return "", "", "", err
	}

	log.Printf("Published host port %d to container %s:%d", hostPort, containerIP, hostPort)
	return hostIP, containerIP, hostIface, nil
}

// configureContainerInterface enters the container network namespace and sets up eth0 there.
func configureContainerInterface(pid int, hostIP, containerIP string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	currentNS, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return fmt.Errorf("failed to open current netns: %w", err)
	}
	defer currentNS.Close()

	childNS, err := os.Open(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return fmt.Errorf("failed to open child netns: %w", err)
	}
	defer childNS.Close()

	if err := unix.Setns(int(childNS.Fd()), unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to enter child netns: %w", err)
	}
	defer func() {
		if err := unix.Setns(int(currentNS.Fd()), unix.CLONE_NEWNET); err != nil {
			log.Printf("warning: failed to restore host netns: %v", err)
		}
	}()

	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return fmt.Errorf("ip command not found in child netns: %w", err)
	}

	if err := runCommand(ipPath, "link", "set", "dev", "lo", "up"); err != nil {
		return fmt.Errorf("failed to bring up loopback: %w", err)
	}

	if err := runCommand(ipPath, "addr", "add", fmt.Sprintf("%s/24", containerIP), "dev", "eth0"); err != nil {
		return fmt.Errorf("failed to assign container IP: %w", err)
	}

	if err := runCommand(ipPath, "link", "set", "dev", "eth0", "up"); err != nil {
		return fmt.Errorf("failed to bring up container eth0: %w", err)
	}

	if err := runCommand(ipPath, "route", "add", "default", "via", hostIP); err != nil {
		return fmt.Errorf("failed to add default route in container: %w", err)
	}

	return nil
}

// deleteInterface removes the host veth side when the container exits.
func deleteInterface(iface string) error {
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return err
	}
	return runCommand(ipPath, "link", "delete", iface)
}
