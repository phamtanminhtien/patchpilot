package runner

import (
	"context"
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
		"git diff",
		"git log",
		"pnpm test",
		"pnpm --dir web build",
		"npm run lint",
		"yarn --dir web test",
		"bun run build",
		"go test ./...",
		"go build ./...",
		"python3 -m pytest",
		"cargo test",
		"make dev",
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

func TestClassifyBlocksCommandsOutsideSafeTable(t *testing.T) {
	for _, command := range []string{
		"node scripts/check.js",
		"pnpm exec tsc",
		"git show HEAD",
		"go test ./internal/runner",
		"python -m pytest",
		"python -c print",
		"python3 -c print",
		"node -e console.log",
		"git -c core.hooksPath=/tmp/hooks status",
	} {
		t.Run(command, func(t *testing.T) {
			decision, err := Classify(command)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}
			if decision.Level != SafetyBlocked {
				t.Fatalf("expected blocked, got %+v", decision)
			}
			if len(decision.Parts) == 0 || decision.Reason == "" {
				t.Fatalf("expected decision details, got %+v", decision)
			}
		})
	}
}

func TestClassifyBlocksUnsupportedExecutables(t *testing.T) {
	for _, command := range []string{
		"ruby scripts/check.rb",
		"sh -c date",
		"bash script.sh",
		"curl http://example.test",
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

func TestClassifyBlocksDestructivePathAndShellCommands(t *testing.T) {
	for _, command := range []string{
		"rm -rf dist",
		"rm -fr dist",
		"git reset --hard",
		"git clean -fd",
		"sudo make test",
		"su root",
		"chmod -R 777 .",
		"chown -R user .",
		"pnpm test && rm -rf dist",
		"pnpm test || true",
		"pnpm test; rm dist",
		"pnpm test | tee out.txt",
		"pnpm test > out.txt",
		"pnpm test < input.txt",
		"echo `date`",
		"echo $(date)",
		"pnpm test\nrm -rf dist",
		"pnpm --dir ../other test",
		"go test ../other",
		"go test /tmp/project",
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
	started := make(chan int, 1)

	err := runner.Start(RunSpec{
		ID:          "cmd_1",
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("testdata", "success"),
	}, Hooks{
		OnStarted: func(pid int) {
			started <- pid
		},
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

	pid := waitForStarted(t, started)
	if pid <= 0 {
		t.Fatalf("expected positive process pid, got %d", pid)
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

func TestRunnerStopAndWaitStopsRunningCommand(t *testing.T) {
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
	if !runner.StopAndWait(context.Background(), "cmd_1") {
		t.Fatal("expected StopAndWait to find running command")
	}

	result := waitForFinish(t, finished)
	if result.Status != "stopped" {
		t.Fatalf("expected stopped result, got %+v", result)
	}
	if runner.Stop("cmd_1") {
		t.Fatal("expected command to be removed after StopAndWait")
	}
}

func waitForStarted(t *testing.T, started <-chan int) int {
	t.Helper()
	select {
	case pid := <-started:
		return pid
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for command start")
		return 0
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
