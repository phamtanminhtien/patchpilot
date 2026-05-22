//go:build darwin

package ports

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

type listenerScanner struct{}

func (listenerScanner) ListeningPorts(ctx context.Context, rootPID int) ([]int, error) {
	pids, err := processTreePIDs(ctx, rootPID)
	if err != nil {
		return nil, err
	}
	if len(pids) == 0 {
		return nil, nil
	}
	pidArgs := make([]string, 0, len(pids))
	for _, pid := range pids {
		pidArgs = append(pidArgs, strconv.Itoa(pid))
	}
	args := []string{"-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-F", "n", "-p", strings.Join(pidArgs, ",")}
	output, err := exec.CommandContext(ctx, "lsof", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return nil, nil
		}
		return nil, err
	}
	return uniqueSortedPorts(parseLsofPorts(string(output))), nil
}

func processTreePIDs(ctx context.Context, rootPID int) ([]int, error) {
	if rootPID <= 0 {
		return nil, nil
	}
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil, err
	}
	children := map[int][]int{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, ppidErr := strconv.Atoi(fields[1])
		if pidErr != nil || ppidErr != nil {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}
	return collectProcessTree(rootPID, children), nil
}

func collectProcessTree(rootPID int, children map[int][]int) []int {
	seen := map[int]struct{}{}
	queue := []int{rootPID}
	pids := make([]int, 0, 4)
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		pids = append(pids, pid)
		queue = append(queue, children[pid]...)
	}
	return pids
}

func parseLsofPorts(output string) []int {
	var ports []int
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		port, ok := portFromAddress(strings.TrimPrefix(line, "n"))
		if ok {
			ports = append(ports, port)
		}
	}
	return ports
}

func portFromAddress(address string) (int, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return 0, false
	}
	address = strings.TrimSpace(strings.TrimPrefix(address, "TCP"))
	if idx := strings.IndexByte(address, ' '); idx >= 0 {
		address = address[:idx]
	}
	idx := strings.LastIndexByte(address, ':')
	if idx < 0 || idx == len(address)-1 {
		return 0, false
	}
	port, err := strconv.Atoi(address[idx+1:])
	if err != nil {
		return 0, false
	}
	return port, port >= 1 && port <= 65535
}
