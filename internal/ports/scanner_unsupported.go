//go:build !darwin && !linux

package ports

import "context"

type listenerScanner struct{}

func (listenerScanner) ListeningPorts(context.Context, int) ([]int, error) {
	return nil, nil
}
