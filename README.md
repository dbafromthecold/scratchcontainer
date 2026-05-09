### Building a container from scratch

Repository to build a container in Go - demo repo for me to learn

This repo is using Liz Rice's containers from scratch code...
https://www.youtube.com/watch?v=_TsSmSu57Zo

## Features

- Linux namespace isolation (UTS, PID, Mount, Network)
- Control group resource limits (CPU and memory)
- Chroot filesystem isolation
- Proc filesystem mounting
- Configurable root filesystem

## Usage

```bash
# Build the container runtime
go build -o scratchcontainer

# Run a command in a container
./scratchcontainer run /path/to/rootfs /bin/bash

# Example with a simple command
./scratchcontainer run /tmp/rootfs echo "Hello from container"
```

## Requirements

- Linux kernel with namespace and cgroup support
- Root filesystem prepared at the specified path
- Run as root (for namespace operations)

## Security Note

This is a basic educational implementation. For production use, consider established container runtimes like runc or containerd.


