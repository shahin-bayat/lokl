package runner

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// SignalTree sends sig to all descendants of pid in the OS process tree.
// Reaches grandchild processes that reset their own process group (e.g. node spawned by pnpm).
func SignalTree(pid int, sig syscall.Signal) {
	out, err := exec.Command("ps", "-o", "pid=,ppid=", "-ax").Output()
	if err != nil {
		return
	}
	children := make(map[int][]int)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		p, err1 := strconv.Atoi(fields[0])
		pp, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		children[pp] = append(children[pp], p)
	}
	queue := []int{pid}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			_ = syscall.Kill(child, sig)
			queue = append(queue, child)
		}
	}
}
