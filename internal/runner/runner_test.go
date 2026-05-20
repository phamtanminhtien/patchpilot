package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestQueueRejectsEmptyCommand(t *testing.T) {
	runner := NewRunner()

	_, err := runner.Queue("  ")
	if !errors.Is(err, ErrEmptyCommand) {
		t.Fatalf("expected ErrEmptyCommand, got %v", err)
	}
}

func TestQueueReturnsQueuedCommand(t *testing.T) {
	runner := NewRunner()

	command, err := runner.Queue("go test ./...")
	if err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}
	if !strings.HasPrefix(command.ID, "cmd_") {
		t.Fatalf("expected cmd_ ID, got %q", command.ID)
	}
	if command.Command != "go test ./..." || command.Status != "queued" {
		t.Fatalf("unexpected command: %+v", command)
	}
}
