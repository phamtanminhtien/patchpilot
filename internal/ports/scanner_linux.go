//go:build linux

package ports

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type listenerScanner struct{}

func (listenerScanner) ListeningPorts(ctx context.Context, rootPID int) ([]int, error) {
	pids, err := processTreePIDs(ctx, rootPID)
	if err != nil {
		return nil, err
	}
	inodes := map[string]struct{}{}
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, inode := range socketInodesForPID(pid) {
			inodes[inode] = struct{}{}
		}
	}
	if len(inodes) == 0 {
		return nil, nil
	}
	ports, err := listeningPortsForInodes(inodes)
	if err != nil {
		return nil, err
	}
	return uniqueSortedPorts(ports), nil
}

func processTreePIDs(ctx context.Context, rootPID int) ([]int, error) {
	if rootPID <= 0 {
		return nil, nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	children := map[int][]int{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		ppid, ok := ppidForPID(pid)
		if !ok {
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

func ppidForPID(pid int) (int, bool) {
	content, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, false
	}
	stat := string(content)
	end := strings.LastIndexByte(stat, ')')
	if end < 0 || end+2 >= len(stat) {
		return 0, false
	}
	fields := strings.Fields(stat[end+2:])
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	return ppid, err == nil
}

func socketInodesForPID(pid int) []string {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	inodes := make([]string, 0, len(entries))
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
			continue
		}
		inodes = append(inodes, strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"))
	}
	return inodes
}

func listeningPortsForInodes(inodes map[string]struct{}) ([]int, error) {
	var ports []int
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		filePorts, err := listeningPortsFromProcNet(path, inodes)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		ports = append(ports, filePorts...)
	}
	return ports, nil
}

func listeningPortsFromProcNet(path string, inodes map[string]struct{}) ([]int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ports []int
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[3] != "0A" {
			continue
		}
		if _, ok := inodes[fields[9]]; !ok {
			continue
		}
		if port, ok := portFromProcAddress(fields[1]); ok {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

func portFromProcAddress(address string) (int, bool) {
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return 0, false
	}
	rawIP, err := hex.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	if len(rawIP) == net.IPv4len && !isAllowedIPv4(rawIP) {
		return 0, false
	}
	port64, err := strconv.ParseInt(parts[1], 16, 32)
	if err != nil {
		return 0, false
	}
	port := int(port64)
	return port, port >= 1 && port <= 65535
}

func isAllowedIPv4(raw []byte) bool {
	return raw[0] == 0 || raw[0] == 1 && raw[1] == 0 && raw[2] == 0 && raw[3] == 127
}
