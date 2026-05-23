package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var ErrEmptyCommand = errors.New("command is required")

type SafetyLevel string

const (
	SafetyAllowed           SafetyLevel = "allowed"
	SafetyNeedsConfirmation SafetyLevel = "needs_confirmation"
	SafetyBlocked           SafetyLevel = "blocked"
)

type SafetyDecision struct {
	Level  SafetyLevel `json:"level"`
	Reason string      `json:"reason"`
	Parts  []string    `json:"parts"`
}

type RunSpec struct {
	ID          string
	WorkspaceID string
	Command     string
	Cwd         string
}

type FinishResult struct {
	ExitCode *int
	Status   string
}

type Hooks struct {
	OnOutput   func(stream, chunk string)
	OnStarted  func(pid int)
	OnFinished func(FinishResult)
}

type Runner struct {
	mu     sync.Mutex
	active map[string]*activeProcess
}

type activeProcess struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func NewRunner() *Runner {
	return &Runner{active: map[string]*activeProcess{}}
}

func Classify(command string) (SafetyDecision, error) {
	parts, err := parse(command)
	if err != nil {
		return SafetyDecision{}, err
	}
	decision := SafetyDecision{
		Level: SafetyNeedsConfirmation,
		Parts: parts,
	}
	if reason := blockedReason(command, parts); reason != "" {
		decision.Level = SafetyBlocked
		decision.Reason = reason
		return decision, nil
	}
	if allowed(parts) {
		decision.Level = SafetyAllowed
		decision.Reason = "Common project command"
		return decision, nil
	}
	decision.Reason = "Command is outside the common project command allowlist"
	return decision, nil
}

func (r *Runner) Start(spec RunSpec, hooks Hooks) error {
	parts, err := parse(spec.Command)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.active[spec.ID] = &activeProcess{cancel: cancel, done: make(chan struct{})}
	r.mu.Unlock()

	go r.run(ctx, spec.ID, spec.Cwd, parts, hooks)
	return nil
}

func (r *Runner) Stop(commandID string) bool {
	r.mu.Lock()
	process, ok := r.active[commandID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	process.cancel()
	return true
}

func (r *Runner) StopAndWait(ctx context.Context, commandID string) bool {
	r.mu.Lock()
	process, ok := r.active[commandID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	process.cancel()
	select {
	case <-process.done:
		return true
	case <-ctx.Done():
		return true
	}
}

func (r *Runner) run(ctx context.Context, commandID, cwd string, parts []string, hooks Hooks) {
	defer func() {
		r.mu.Lock()
		process := r.active[commandID]
		delete(r.active, commandID)
		r.mu.Unlock()
		if process != nil {
			close(process.done)
		}
	}()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = cwd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if hooks.OnFinished != nil {
			hooks.OnFinished(FinishResult{Status: "failed"})
		}
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		if hooks.OnFinished != nil {
			hooks.OnFinished(FinishResult{Status: "failed"})
		}
		return
	}
	if err := cmd.Start(); err != nil {
		status := "failed"
		if ctx.Err() != nil {
			status = "stopped"
		}
		if hooks.OnFinished != nil {
			hooks.OnFinished(FinishResult{Status: status})
		}
		return
	}
	if hooks.OnStarted != nil {
		hooks.OnStarted(cmd.Process.Pid)
	}

	var readers sync.WaitGroup
	readers.Add(2)
	go streamOutput(&readers, stdout, "stdout", hooks.OnOutput)
	go streamOutput(&readers, stderr, "stderr", hooks.OnOutput)
	waitErr := cmd.Wait()
	readers.Wait()

	result := FinishResult{Status: "exited"}
	if waitErr != nil {
		if ctx.Err() != nil {
			result.Status = "stopped"
		} else {
			result.Status = "failed"
		}
	}
	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		result.ExitCode = &code
	}
	if hooks.OnFinished != nil {
		hooks.OnFinished(result)
	}
}

func streamOutput(wg *sync.WaitGroup, reader io.Reader, stream string, onOutput func(stream, chunk string)) {
	defer wg.Done()
	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 && onOutput != nil {
			onOutput(stream, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func parse(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, ErrEmptyCommand
	}
	return strings.Fields(command), nil
}

func blockedReason(command string, parts []string) string {
	for _, token := range []string{"&&", "||", ";", "|", ">", "<", "`", "$(", "\n", "\r"} {
		if strings.Contains(command, token) {
			return fmt.Sprintf("Shell syntax %q is not allowed", token)
		}
	}
	if filepath.IsAbs(parts[0]) {
		return "Absolute command paths are not allowed"
	}
	if len(parts) > 0 {
		switch parts[0] {
		case "sudo", "su":
			return "Privilege escalation commands are blocked"
		case "rm":
			for _, part := range parts[1:] {
				if strings.Contains(part, "r") && strings.Contains(part, "f") && strings.HasPrefix(part, "-") {
					return "Recursive forced removal is blocked"
				}
			}
		case "chmod", "chown":
			for _, part := range parts[1:] {
				if part == "-R" || strings.HasPrefix(part, "-R") {
					return "Recursive permission changes are blocked"
				}
			}
		case "git":
			if len(parts) >= 2 && parts[1] == "clean" {
				return "git clean is blocked"
			}
			if len(parts) >= 3 && parts[1] == "reset" && parts[2] == "--hard" {
				return "git reset --hard is blocked"
			}
		}
	}
	for _, part := range parts[1:] {
		if filepath.IsAbs(part) {
			return "Absolute paths are not allowed in command arguments"
		}
		clean := filepath.Clean(part)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "Workspace escape paths are blocked"
		}
	}
	return ""
}

func allowed(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "git":
		return len(parts) == 2 && (parts[1] == "status" || parts[1] == "diff" || parts[1] == "log")
	case "npm":
		return len(parts) == 3 && parts[1] == "run" && projectScript(parts[2])
	case "pnpm", "yarn":
		if len(parts) == 2 && projectScript(parts[1]) {
			return true
		}
		return len(parts) == 4 && parts[1] == "--dir" && safeRelativeDir(parts[2]) && projectScript(parts[3])
	case "bun":
		return (len(parts) == 2 && parts[1] == "test") || (len(parts) == 3 && parts[1] == "run" && projectScript(parts[2]))
	case "go":
		if len(parts) == 3 && parts[1] == "build" && parts[2] == "./..." {
			return true
		}
		return len(parts) == 3 && parts[1] == "test" && (parts[2] == "./..." || strings.HasPrefix(parts[2], "./"))
	case "pytest":
		return len(parts) == 1
	case "python", "python3":
		return len(parts) == 3 && parts[1] == "-m" && parts[2] == "pytest"
	case "cargo":
		return len(parts) == 2 && (parts[1] == "test" || parts[1] == "build")
	case "make":
		return len(parts) == 2 && projectScript(parts[1])
	default:
		return false
	}
}

func projectScript(script string) bool {
	return script == "test" || script == "lint" || script == "build" || script == "dev"
}

func safeRelativeDir(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
