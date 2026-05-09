//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// setupCgroups creates simple memory and CPU cgroup limits for the current process.
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

	// Setup CPU cgroup. Some systems mount cpu and cpuacct together, while
	// others, including WSL, expose cpu as its own controller.
	cpu, err := findCPUController(cgroups)
	if err != nil {
		return err
	}
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

	// Enable auto-cleanup when the cgroup becomes empty.
	if err := os.WriteFile(filepath.Join(memoryPath, "notify_on_release"), []byte("1"), 0700); err != nil {
		log.Printf("Warning: failed to enable memory cgroup cleanup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cpuPath, "notify_on_release"), []byte("1"), 0700); err != nil {
		log.Printf("Warning: failed to enable cpu cgroup cleanup: %v", err)
	}

	return nil
}

func findCPUController(cgroups string) (string, error) {
	for _, controller := range []string{"cpu,cpuacct", "cpu"} {
		path := filepath.Join(cgroups, controller)
		if _, err := os.Stat(filepath.Join(path, "cpu.cfs_quota_us")); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("failed to find writable cpu cgroup controller")
}
