package ports

import "context"

type ListenerScanner interface {
	ListeningPorts(ctx context.Context, rootPID int) ([]int, error)
}

func NewListenerScanner() ListenerScanner {
	return listenerScanner{}
}

func uniqueSortedPorts(ports []int) []int {
	seen := map[int]struct{}{}
	unique := make([]int, 0, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		unique = append(unique, port)
	}
	for i := 1; i < len(unique); i++ {
		for j := i; j > 0 && unique[j] < unique[j-1]; j-- {
			unique[j], unique[j-1] = unique[j-1], unique[j]
		}
	}
	return unique
}
