// Two halves of the containers lesson.
//  1. A container is a process: new namespaces plus a cgroup limit.
//     Starting one needs root on Linux, so main prints the plan and
//     only runs it with -run.
//  2. Image layers are cached by a hash of the parent layer plus the
//     step and its input files. main replays both Dockerfiles.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// startContained runs cmd as a "container": its own hostname, process ids
// and mounts (namespaces), and at most memMax bytes (cgroup v2).
func startContained(name string, memMax int64, cmd string, args ...string) error {
	cg := filepath.Join("/sys/fs/cgroup", name)
	if err := os.MkdirAll(cg, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cg, "memory.max"), []byte(fmt.Sprint(memMax)), 0o644); err != nil { // 268435456 = 256 MiB
		return err
	}
	c := exec.Command(cmd, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | // own hostname
			syscall.CLONE_NEWPID | // own process ids
			syscall.CLONE_NEWNS, // own mounts
	}
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Start(); err != nil { // a fork, no kernel boots
		return err
	}
	pid := []byte(fmt.Sprint(c.Process.Pid))
	if err := os.WriteFile(filepath.Join(cg, "cgroup.procs"), pid, 0o644); err != nil {
		return err
	}
	return c.Wait() // every syscall goes to the one host kernel
}

// startVM boots a microVM with Firecracker: the hypervisor gives it 2 CPUs
// and 4 GiB, and a whole guest kernel boots before the service starts.
func startVM(kernel, rootfs string) error {
	cfg := fmt.Sprintf(`{"machine-config": {"vcpu_count": 2, "mem_size_mib": 4096},
  "boot-source": {"kernel_image_path": %q, "boot_args": "init=/indexer"},
  "drives": [{"drive_id": "root", "path_on_host": %q,
    "is_root_device": true, "is_read_only": false}]}`, kernel, rootfs)
	if err := os.WriteFile("vm.json", []byte(cfg), 0o644); err != nil {
		return err
	}
	vm := exec.Command("firecracker", "--no-api", "--config-file", "vm.json")
	vm.Stdout, vm.Stderr = os.Stdout, os.Stderr
	return vm.Run() // a guest kernel boots, then init, then the service
}

// Step is one Dockerfile line and the files it reads.
type Step struct {
	Line   string
	Inputs []string
}

// build returns how many steps ran. A step's key is the hash of the parent
// key, the line and the content of its inputs. A known key is a cache hit.
func build(steps []Step, files map[string]string, cache map[[32]byte]bool) int {
	ran := 0
	var parent [32]byte
	for _, s := range steps {
		h := sha256.New()
		h.Write(parent[:])
		h.Write([]byte(s.Line))
		for _, f := range s.Inputs {
			h.Write([]byte(files[f]))
		}
		var key [32]byte
		copy(key[:], h.Sum(nil))
		if cache[key] {
			fmt.Printf("    cache hit  %s\n", s.Line)
		} else {
			fmt.Printf("    RUN        %s\n", s.Line)
			cache[key] = true
			ran++
		}
		parent = key // a change here changes every key above it
	}
	return ran
}

func main() {
	run := flag.Bool("run", false, "really start a contained shell (root, Linux)")
	boot := flag.Bool("vm", false, "really boot a microVM (needs firecracker, vmlinux and rootfs.ext4)")
	flag.Parse()
	fmt.Println("A VM is a machine:")
	fmt.Println("  firecracker: 2 vCPUs, 4096 MiB, its own guest kernel boots first")
	if *boot {
		if err := startVM("vmlinux", "rootfs.ext4"); err != nil {
			fmt.Println("  boot failed:", err)
		}
	}
	fmt.Println("A container is a process:")
	fmt.Println("  clone flags: NEWUTS NEWPID NEWNS, cgroup memory.max = 268435456 (256 MiB)")
	if *run {
		if err := startContained("indexer", 256<<20, "/bin/sh", "-c", "echo pid $$ in its own namespace"); err != nil {
			fmt.Println("  start failed:", err)
		}
	}

	files := map[string]string{"go.mod": "module indexer", "go.sum": "h1:abc", "main.go": "v1"}
	good := []Step{
		{"FROM golang", nil},
		{"COPY go.mod go.sum", []string{"go.mod", "go.sum"}},
		{"RUN go mod download", nil},
		{"COPY . .", []string{"go.mod", "go.sum", "main.go"}},
		{"RUN go build", nil},
	}
	bad := []Step{
		{"FROM golang", nil},
		{"COPY . .", []string{"go.mod", "go.sum", "main.go"}},
		{"RUN go mod download", nil},
		{"RUN go vet", nil},
		{"RUN go build", nil},
	}
	for _, c := range []struct {
		name  string
		steps []Step
	}{{"lock files first", good}, {"all code first", bad}} {
		cache := map[[32]byte]bool{}
		files["main.go"] = "v1"
		fmt.Printf("%s, first build: %d of 5 run\n", c.name, build(c.steps, files, cache))
		files["main.go"] = "v2: one line fixed"
		fmt.Printf("%s, rebuild after a code change: %d of 5 run\n", c.name, build(c.steps, files, cache))
	}
}
