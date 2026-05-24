package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	ErrEmptyCommand   = errors.New("command is required")
	ErrBlockedCommand = errors.New("command is blocked")
)

var safeCommandToken = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

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

type commandInvocation struct {
	Name  string
	Args  []string
	Parts []string
}

func NewRunner() *Runner {
	return &Runner{active: map[string]*activeProcess{}}
}

func Classify(command string) (SafetyDecision, error) {
	invocation, err := parseInvocation(command)
	if err != nil {
		return SafetyDecision{}, err
	}
	decision := SafetyDecision{
		Level: SafetyNeedsConfirmation,
		Parts: invocation.Parts,
	}
	if reason := blockedReason(command, invocation.Parts); reason != "" {
		decision.Level = SafetyBlocked
		decision.Reason = reason
		return decision, nil
	}
	if allowed(invocation.Parts) {
		decision.Level = SafetyAllowed
		decision.Reason = "Common project command"
		return decision, nil
	}
	decision.Reason = "Command is outside the common project command allowlist"
	return decision, nil
}

func (r *Runner) Start(spec RunSpec, hooks Hooks) error {
	invocation, err := parseInvocation(spec.Command)
	if err != nil {
		return err
	}
	if reason := blockedReason(spec.Command, invocation.Parts); reason != "" {
		return fmt.Errorf("%w: %s", ErrBlockedCommand, reason)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.active[spec.ID] = &activeProcess{cancel: cancel, done: make(chan struct{})}
	r.mu.Unlock()

	go r.run(ctx, spec.ID, spec.Cwd, invocation, hooks)
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

func (r *Runner) run(ctx context.Context, commandID, cwd string, invocation commandInvocation, hooks Hooks) {
	defer func() {
		r.mu.Lock()
		process := r.active[commandID]
		delete(r.active, commandID)
		r.mu.Unlock()
		if process != nil {
			close(process.done)
		}
	}()

	cmd := commandContext(ctx, invocation)
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

func parseInvocation(command string) (commandInvocation, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return commandInvocation{}, ErrEmptyCommand
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return commandInvocation{}, ErrEmptyCommand
	}
	return commandInvocation{Name: parts[0], Args: parts[1:], Parts: parts}, nil
}

func commandContext(ctx context.Context, invocation commandInvocation) *exec.Cmd {
	switch invocation.Name {
	case "git":
		return exec.CommandContext(ctx, "git", invocation.Args...)
	case "pnpm":
		return exec.CommandContext(ctx, "pnpm", invocation.Args...)
	case "npm":
		return exec.CommandContext(ctx, "npm", invocation.Args...)
	case "yarn":
		return exec.CommandContext(ctx, "yarn", invocation.Args...)
	case "bun":
		return exec.CommandContext(ctx, "bun", invocation.Args...)
	case "go":
		return exec.CommandContext(ctx, "go", invocation.Args...)
	case "python":
		return exec.CommandContext(ctx, "python", invocation.Args...)
	case "python3":
		return exec.CommandContext(ctx, "python3", invocation.Args...)
	case "pytest":
		return exec.CommandContext(ctx, "pytest", invocation.Args...)
	case "cargo":
		return exec.CommandContext(ctx, "cargo", invocation.Args...)
	case "make":
		return exec.CommandContext(ctx, "make", invocation.Args...)
	case "node":
		return exec.CommandContext(ctx, "node", invocation.Args...)
	default:
		// parseInvocation and blockedReason prevent this before execution.
		return exec.CommandContext(ctx, "false")
	}
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
	if !safeCommandToken.MatchString(parts[0]) {
		return "Unsupported command token characters are blocked"
	}
	if !supportedExecutable(parts[0]) {
		return "Unsupported command executable is blocked"
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
		if !safeCommandToken.MatchString(part) {
			return "Unsupported command token characters are blocked"
		}
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

func supportedExecutable(name string) bool {
	switch name {
	case "git", "pnpm", "npm", "yarn", "bun", "go", "python", "python3", "pytest", "cargo", "make", "node":
		return true
	default:
		return false
	}
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
