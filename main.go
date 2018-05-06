package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"github.com/containerd/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
	fmt.Printf("Running %v \n", os.Args[3:])
	shares := uint64(100)
	mem, _ := strconv.ParseInt(os.Args[2], 10, 64)
	memlimit := int64(mem*1024*1024)
	fmt.Println("memlimit: ",memlimit," : in mb: ",mem)
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath("/test"), &specs.LinuxResources{
    	CPU: &specs.LinuxCPU{
        	Shares: &shares,
		Cpus:   "0",
		Mems:	"0",	
    	},
	Memory: &specs.LinuxMemory{
		Limit: &memlimit,
	},
	})
	if err != nil {
		fmt.Println("Error while creating new cgroup: ",err)
		os.Exit(1)
	}
	defer control.Delete()
	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET| syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	//must(syscall.Mount("/tmp", "/mytemp", "tmpfs", 0, ""))
	//must(cmd.Run())
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error: ",err)
	}
	pid := cmd.Process.Pid
	if err := control.Add(cgroups.Process{Pid:pid}); err != nil {
		fmt.Printf("Error while adding process : %d to cgroups: %s\n",pid,err)
	}
	fmt.Println("Waiting for command to finish...",pid)
	err = cmd.Wait()
	if err!=nil {
	if exiterr, ok := err.(*exec.ExitError); ok {
            // The program has exited with an exit code != 0

            // This works on both Unix and Windows. Although package
            // syscall is generally platform dependent, WaitStatus is
            // defined for both Unix and Windows and in both cases has
            // an ExitStatus() method with the same signature.
            if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
                fmt.Printf("Exit Status: %d\n", status.ExitStatus())
            }
        }
	}
	fmt.Println("Command finished with error: ", err)
}

func child() {
	fmt.Printf("Running %v \n", os.Args[2:])

	cg()

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(syscall.Sethostname([]byte("container")))
	must(syscall.Chroot("/"))
	must(os.Chdir("/"))
	must(syscall.Mount("proc", "proc", "proc", 0, ""))
	must(syscall.Mount("thing", "mytemp", "tmpfs", 0, ""))

	must(cmd.Run())

	must(syscall.Unmount("proc", 0))
	must(syscall.Unmount("thing", 0))
}

func cg() {
	cgroups := "/sys/fs/cgroup/"
	pids := filepath.Join(cgroups, "pids")
	os.Mkdir(filepath.Join(pids, "liz"), 0755)
	must(ioutil.WriteFile(filepath.Join(pids, "liz/pids.max"), []byte("20"), 0700))
	// Removes the new cgroup in place after the container exits
	must(ioutil.WriteFile(filepath.Join(pids, "liz/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(pids, "liz/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

