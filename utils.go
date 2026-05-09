//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// waitForParent blocks the child until the parent has configured networking.
func waitForParent() error {
	syncFile := os.NewFile(uintptr(3), "syncpipe")
	if syncFile == nil {
		return fmt.Errorf("failed to access sync pipe")
	}
	defer syncFile.Close()

	buf := make([]byte, 1)
	n, err := syncFile.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to wait for network setup: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("sync pipe closed before network setup completed")
	}

	return nil
}

// runCommand executes an external command and returns combined stdout/stderr.
func runCommand(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", path, strings.Join(args, " "), err, bytes.TrimSpace(output))
	}
	return nil
}