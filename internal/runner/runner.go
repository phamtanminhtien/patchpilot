package runner

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var ErrEmptyCommand = errors.New("command is required")

type Command struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Runner struct {
	nextID atomic.Uint64
}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Queue(command string) (Command, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return Command{}, ErrEmptyCommand
	}
	return Command{
		ID:        fmt.Sprintf("cmd_%d", r.nextID.Add(1)),
		Command:   command,
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
	}, nil
}
