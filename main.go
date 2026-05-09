//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
)

// scratchcontainer is a minimal educational container runtime.
// It demonstrates Linux namespaces, cgroups, chroot, proc mounting,
// and optional host port publishing into a container.
//
// Usage:
//   scratchcontainer run [-publish hostPort:containerPort] <rootfs> <command> [args...]
func main() {
	// The first argument selects the process mode: run or child.
	if len(os.Args) < 2 {
		usage()
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
		usage()
		os.Exit(1)        }
}

func usage() {
        fmt.Println("Usage: scratchcontainer run [-publish hostPort:containerPort] <rootfs> <command> [args...]")
        fmt.Println("Example: scratchcontainer run -publish 15789:1433 /path/to/rootfs /bin/bash")
}