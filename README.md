⁸# ScratchContainer

A minimal, educational container runtime written in Go that demonstrates the core concepts of Linux containerization from scratch.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black)](https://www.linux.org/)

## Overview

ScratchContainer is an educational project that implements a basic container runtime using only standard Go libraries and Linux system calls. It serves as a learning tool to understand how containers work under the hood, demonstrating key concepts like:

- **Linux Namespaces**: Process isolation (UTS, PID, Mount, Network)
- **Control Groups (cgroups)**: Resource limits and constraints
- **Chroot**: Filesystem isolation
- **Virtual Networking**: Veth pairs and port forwarding
- **Process Management**: Parent-child process coordination

This is **not** a production-ready container runtime. It's designed for educational purposes to help developers understand container internals.

## Architecture

The runtime consists of two main execution modes:

1. **Parent Process (`run`)**: Sets up namespaces, networking, and resource limits
2. **Child Process (`child`)**: Executes within the isolated environment

Key components:
- `main.go`: Entry point and CLI handling
- `container.go`: Container lifecycle management
- `network.go`: Virtual networking and port forwarding
- `cgroups.go`: Resource control groups
- `utils.go`: Utility functions

## Features

- ✅ Linux namespace isolation (UTS, PID, Mount, Network)
- ✅ Control group resource limits (CPU: 20%, Memory: 2GB)
- ✅ Chroot filesystem isolation
- ✅ Proc filesystem mounting
- ✅ Virtual Ethernet (veth) networking
- ✅ Host port publishing to containers
- ✅ Process tree isolation

## Requirements

- **Operating System**: Linux (namespace and cgroup support required)
- **Go Version**: 1.21 or later
- **Permissions**: Must run as root for namespace operations
- **Root Filesystem**: A prepared root filesystem directory

## Quick Start

### 1. Build the Runtime

```bash
go build -o scratchcontainer .
```

### 2. Prepare a Root Filesystem

Create a minimal root filesystem (this is just an example):

```bash
mkdir -p /tmp/rootfs/{bin,proc}
cp /bin/bash /tmp/rootfs/bin/
cp /bin/ls /tmp/rootfs/bin/
# Add other necessary binaries and libraries...
```

### 3. Run a Container

```bash
# Basic command execution
sudo ./scratchcontainer run /tmp/rootfs echo "Hello from container!"

# Interactive shell
sudo ./scratchcontainer run /tmp/rootfs /bin/bash

# With port forwarding (host:15789 → container:1433)
sudo ./scratchcontainer run -publish 15789:1433 /tmp/rootfs /bin/bash
```

## Usage

```
Usage: scratchcontainer run [-publish hostPort:containerPort] <rootfs> <command> [args...]

Options:
  -publish string
        Publish host:container port mapping (e.g., 8080:80)

Arguments:
  <rootfs>    Path to the container root filesystem
  <command>   Command to execute inside the container
  [args...]   Additional arguments for the command
```

## Examples

### Basic Container Execution
```bash
sudo ./scratchcontainer run /tmp/rootfs /bin/ls -la /
```

### Networked Container with Port Forwarding
```bash
# Start a simple HTTP server in the container
sudo ./scratchcontainer run -publish 8080:80 /tmp/rootfs python3 -m http.server 80
```

### Resource-Constrained Container
The container automatically applies:
- CPU limit: 20% of host CPU
- Memory limit: 2GB
- Automatic cleanup on exit

## Technical Details

### Namespace Isolation
- **UTS**: Isolated hostname
- **PID**: Separate process ID space
- **Mount**: Isolated filesystem view
- **Network**: Virtual network stack

### Networking Implementation
- Creates veth pair for host-container communication
- Assigns private IP addresses (172.22.x.x/24 subnet)
- Configures routing and NAT
- Supports TCP port forwarding

### Resource Management
- Uses cgroup v1 for CPU and memory limits
- Automatic cgroup cleanup
- Process tree isolation

## Security Considerations

⚠️ **Educational Purpose Only**

This implementation is for learning and demonstration. It lacks many security features found in production container runtimes:

- No user namespace mapping
- No seccomp filtering
- No AppArmor/SELinux integration
- No image layer management
- No orchestration integration

**Do not use in production environments.**

## Learning Resources

- [Liz Rice - Containers from Scratch](https://www.youtube.com/watch?v=_TsSmSu57Zo)
- [Linux Namespaces Man Pages](https://man7.org/linux/man-pages/man7/namespaces.7.html)
- [Control Groups Documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/index.html)
- [Docker Internals](https://docs.docker.com/engine/docker-overview/)

## Contributing

This is an educational project. Feel free to:
- Submit issues for bugs or improvements
- Create pull requests for enhancements
- Use it as a reference for learning container internals

## Licence

This project is for educational purposes. See individual source files for licensing information.


