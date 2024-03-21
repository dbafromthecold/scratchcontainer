package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// go run main.go run <cmd> <args>
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
	fmt.Printf("Running %v \n", os.Args[2:])

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET, // Added CLONE_NEWNET for network namespace
		Unshareflags: syscall.CLONE_NEWNS,
	}

	must(cmd.Run())
	setupPortForwarding() // This function will need to be defined to set up port forwarding
}

func child() {
	fmt.Printf("Running %v \n", os.Args[2:])

	cg()

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(syscall.Sethostname([]byte("container")))
	must(syscall.Chroot("/home/dbafromthecold/sqlserver"))
	must(os.Chdir("/"))
	must(syscall.Mount("proc", "proc", "proc", 0, ""))
	must(cmd.Run())

	must(syscall.Unmount("proc", 0))
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

func setupPortForwarding() {
	// Here you would use exec.Command to call iptables or an equivalent command to forward ports.
	// Example (you'll need to adjust the iptables command to fit your needs):
	// err := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "15789", "-j", "DNAT", "--to-destination", "IP_OF_NAMESPACE:1433").Run()
	// must(err)
	// Note: You'll need to replace IP_OF_NAMESPACE with the actual IP address of your network namespace.
}