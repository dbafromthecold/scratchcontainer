package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("help")
	}
}

func run() {
	fmt.Printf("Running %v as parent\n", os.Args[2:])

	setupNetworkNamespace()

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	must(cmd.Run())
}

func child() {
	fmt.Printf("Running %v as child\n", os.Args[2:])

	cg()

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(syscall.Sethostname([]byte("container")))
	must(syscall.Chroot("/your/new/root"))
	must(os.Chdir("/"))
	must(syscall.Mount("proc", "proc", "proc", 0, ""))

	setupVethForNamespace()

	must(cmd.Run())

	must(syscall.Unmount("proc", 0))
}

func setupNetworkNamespace() {
    // Create a new network namespace
    err := exec.Command("ip", "netns", "add", "dbafromthecold-ns").Run()
    must(err)

    // TODO - add check to make sure namespace does not already exist
}


func setupVethForNamespace() {

    // Create a veth pair: one end (veth-host) stays in the host namespace,
    // the other end (veth-ns) is moved to "dbafromthecold-ns".
    must(exec.Command("ip", "link", "add", "veth-host", "type", "veth", "peer", "name", "veth-ns").Run())

    // Move veth-ns to the namespace
    must(exec.Command("ip", "link", "set", "veth-ns", "netns", "dbafromthecold-ns").Run())

    // Assign an IP to veth-host in the host namespace
    must(exec.Command("ip", "addr", "add", "192.168.1.1/24", "dev", "veth-host").Run())
    must(exec.Command("ip", "link", "set", "veth-host", "up").Run())

    // Assign an IP to veth-ns in "mynamespace" and bring it up
    must(exec.Command("ip", "netns", "exec", "dbafromthecold-ns", "ip", "addr", "add", "192.168.1.2/24", "dev", "veth-ns").Run())
    must(exec.Command("ip", "netns", "exec", "dbafromthecold-ns", "ip", "link", "set", "veth-ns", "up").Run())
    must(exec.Command("ip", "netns", "exec", "dbafromthecold-ns", "ip", "route", "add", "default", "via", "192.168.1.1").Run())
}

func setupPortForwarding() {
	// Forward traffic from port 15789 on the host to port 1433 in the namespace
	must(exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "15789", "-j", "DNAT", "--to-destination", "192.168.1.2:1433").Run())
	must(exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE").Run())
}


func cg() {
	cgroups := "/sys/fs/cgroup/"
	memory := filepath.Join(cgroups, "memory")
	os.Mkdir(filepath.Join(memory, "sqlserver"), 0755)
	must(ioutil.WriteFile(filepath.Join(memory, "sqlserver/memory.limit_in_bytes"), []byte("2147483648"), 0700))

	cpu := filepath.Join(cgroups, "cpu,cpuacct")
	os.Mkdir(filepath.Join(cpu, "sqlserver"), 0755)
	must(ioutil.WriteFile(filepath.Join(cpu, "sqlserver/cpu.cfs_quota_us"), []byte("200000"), 0700))

	// Removes the new cgroup in place after the container exits
	must(ioutil.WriteFile(filepath.Join(memory, "sqlserver/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(memory, "sqlserver/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))

	must(ioutil.WriteFile(filepath.Join(cpu, "sqlserver/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(cpu, "sqlserver/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}