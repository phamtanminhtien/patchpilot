package runner

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassifyRejectsEmptyCommand(t *testing.T) {
	_, err := Classify("  ")
	if !errors.Is(err, ErrEmptyCommand) {
		t.Fatalf("expected ErrEmptyCommand, got %v", err)
	}
}

func TestClassifyAllowsCommonProjectCommands(t *testing.T) {
	for _, command := range []string{
		"git status",
		"pnpm test",
		"pnpm --dir web build",
		"npm run lint",
		"go test ./...",
		"python3 -m pytest",
	} {
		t.Run(command, func(t *testing.T) {
			decision, err := Classify(command)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}
			if decision.Level != SafetyAllowed {
				t.Fatalf("expected allowed, got %+v", decision)
			}
		})
	}
}

func TestClassifyRequiresConfirmationForUnknownCommands(t *testing.T) {
	decision, err := Classify("node scripts/check.js")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if decision.Level != SafetyNeedsConfirmation {
		t.Fatalf("expected needs_confirmation, got %+v", decision)
	}
}

func TestClassifyBlocksDestructiveAndShellCommands(t *testing.T) {
	for _, command := range []string{
		"rm -rf dist",
		"git reset --hard",
		"git clean -fd",
		"sudo make test",
		"chmod -R 777 .",
		"pnpm test && rm -rf dist",
		"pnpm --dir ../other test",
		"/bin/ls",
	} {
		t.Run(command, func(t *testing.T) {
			decision, err := Classify(command)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}
			if decision.Level != SafetyBlocked {
				t.Fatalf("expected blocked, got %+v", decision)
			}
		})
	}
}

func TestRunnerStreamsStdoutStderrAndExitCode(t *testing.T) {
	runner := NewRunner()
	var chunks []string
	finished := make(chan FinishResult, 1)

	err := runner.Start(RunSpec{
		ID:          "cmd_1",
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("testdata", "success"),
	}, Hooks{
		OnOutput: func(stream, chunk string) {
			chunks = append(chunks, stream+":"+chunk)
		},
		OnFinished: func(result FinishResult) {
			finished <- result
		},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	result := waitForFinish(t, finished)
	if result.Status != "exited" || result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !containsChunk(chunks, "stdout:", "ok") {
		t.Fatalf("expected stdout chunk, got %+v", chunks)
	}
}

func TestRunnerReportsFailureExitCode(t *testing.T) {
	runner := NewRunner()
	finished := make(chan FinishResult, 1)

	err := runner.Start(RunSpec{
		ID:          "cmd_1",
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("testdata", "failure"),
	}, Hooks{
		OnFinished: func(result FinishResult) {
			finished <- result
		},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	result := waitForFinish(t, finished)
	if result.Status != "failed" || result.ExitCode == nil || *result.ExitCode == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunnerStopsRunningCommand(t *testing.T) {
	runner := NewRunner()
	finished := make(chan FinishResult, 1)

	err := runner.Start(RunSpec{
		ID:          "cmd_1",
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("testdata", "slow"),
	}, Hooks{
		OnFinished: func(result FinishResult) {
			finished <- result
		},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !runner.Stop("cmd_1") {
		t.Fatal("expected Stop to find running command")
	}

	result := waitForFinish(t, finished)
	if result.Status != "stopped" {
		t.Fatalf("expected stopped result, got %+v", result)
	}
}

func waitForFinish(t *testing.T, finished <-chan FinishResult) FinishResult {
	t.Helper()
	select {
	case result := <-finished:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for command")
		return FinishResult{}
	}
}

func containsChunk(chunks []string, prefix, text string) bool {
	for _, chunk := range chunks {
		if strings.HasPrefix(chunk, prefix) && strings.Contains(chunk, text) {
			return true
		}
	}
	return false
}
