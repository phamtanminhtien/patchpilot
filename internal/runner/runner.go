package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrEmptyCommand   = errors.New("command is required")
	ErrBlockedCommand = errors.New("command is blocked")
)

var safeCommandToken = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./~-]+$`)

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

type commandFactory func(context.Context) *exec.Cmd

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
	read, isRead := readCommand(invocation.Parts)
	if reason := blockedReason(command, invocation.Parts, isRead); reason != "" {
		decision.Level = SafetyBlocked
		decision.Reason = reason
		return decision, nil
	}
	if isRead {
		decision.Reason = read.reason
		if read.requiresApproval {
			decision.Level = SafetyNeedsConfirmation
			return decision, nil
		}
		decision.Level = SafetyAllowed
		return decision, nil
	}
	if _, ok := safeCommand(invocation.Parts); ok {
		decision.Level = SafetyAllowed
		decision.Reason = "Common project command"
		return decision, nil
	}
	decision.Level = SafetyBlocked
	decision.Reason = "Command is outside the explicit safe command table"
	return decision, nil
}

func (r *Runner) Start(spec RunSpec, hooks Hooks) error {
	invocation, err := parseInvocation(spec.Command)
	if err != nil {
		return err
	}
	_, isRead := readCommand(invocation.Parts)
	if reason := blockedReason(spec.Command, invocation.Parts, isRead); reason != "" {
		return fmt.Errorf("%w: %s", ErrBlockedCommand, reason)
	}
	if _, ok := safeCommand(invocation.Parts); !ok {
		return fmt.Errorf("%w: command is outside the explicit safe command table", ErrBlockedCommand)
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
	parts, err := splitCommand(command)
	if err != nil {
		return commandInvocation{}, err
	}
	if len(parts) == 0 {
		return commandInvocation{}, ErrEmptyCommand
	}
	return commandInvocation{Name: parts[0], Args: parts[1:], Parts: parts}, nil
}

func splitCommand(command string) ([]string, error) {
	parts := make([]string, 0)
	var current strings.Builder
	inQuote := false
	for _, r := range command {
		switch {
		case r == '\'':
			inQuote = !inQuote
		case !inQuote && (r == ' ' || r == '\t'):
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("%w: unterminated quote", ErrBlockedCommand)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts, nil
}

func commandContext(ctx context.Context, invocation commandInvocation) *exec.Cmd {
	if factory, ok := safeCommand(invocation.Parts); ok {
		return factory(ctx)
	}
	return exec.CommandContext(ctx, "false")
}

func blockedReason(command string, parts []string, allowReadPath bool) string {
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
		if filepath.IsAbs(part) && !allowReadPath {
			return "Absolute paths are not allowed in command arguments"
		}
		clean := filepath.Clean(part)
		if hasParentTraversal(clean) {
			return "Workspace escape paths are blocked"
		}
	}
	return ""
}

func supportedExecutable(name string) bool {
	switch name {
	case "git", "pnpm", "npm", "yarn", "bun", "go", "python", "python3", "pytest", "cargo", "make", "node", "sed", "cat":
		return true
	default:
		return false
	}
}

func safeCommand(parts []string) (commandFactory, bool) {
	if read, ok := readCommand(parts); ok {
		return read.factory, true
	}
	switch strings.Join(parts, " ") {
	case "git status":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "git", "status") }, true
	case "git diff":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "git", "diff") }, true
	case "git log":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "git", "log") }, true
	case "npm run test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "npm", "run", "test") }, true
	case "npm run lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "npm", "run", "lint") }, true
	case "npm run build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "npm", "run", "build") }, true
	case "npm run dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "npm", "run", "dev") }, true
	case "pnpm test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "test") }, true
	case "pnpm lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "lint") }, true
	case "pnpm build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "build") }, true
	case "pnpm dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "dev") }, true
	case "pnpm --dir web test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "--dir", "web", "test") }, true
	case "pnpm --dir web lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "--dir", "web", "lint") }, true
	case "pnpm --dir web build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "--dir", "web", "build") }, true
	case "pnpm --dir web dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pnpm", "--dir", "web", "dev") }, true
	case "yarn test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "test") }, true
	case "yarn lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "lint") }, true
	case "yarn build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "build") }, true
	case "yarn dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "dev") }, true
	case "yarn --dir web test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "--dir", "web", "test") }, true
	case "yarn --dir web lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "--dir", "web", "lint") }, true
	case "yarn --dir web build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "--dir", "web", "build") }, true
	case "yarn --dir web dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "yarn", "--dir", "web", "dev") }, true
	case "bun test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "bun", "test") }, true
	case "bun run test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "bun", "run", "test") }, true
	case "bun run lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "bun", "run", "lint") }, true
	case "bun run build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "bun", "run", "build") }, true
	case "bun run dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "bun", "run", "dev") }, true
	case "go test ./...":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "go", "test", "./...") }, true
	case "go build ./...":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "go", "build", "./...") }, true
	case "pytest":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "pytest") }, true
	case "python3 -m pytest":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "python3", "-m", "pytest") }, true
	case "cargo test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "cargo", "test") }, true
	case "cargo build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "cargo", "build") }, true
	case "make test":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "make", "test") }, true
	case "make lint":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "make", "lint") }, true
	case "make build":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "make", "build") }, true
	case "make dev":
		return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "make", "dev") }, true
	default:
		return nil, false
	}
}

type readCommandSpec struct {
	factory          commandFactory
	path             string
	requiresApproval bool
	reason           string
}

func readCommand(parts []string) (readCommandSpec, bool) {
	switch {
	case len(parts) == 2 && parts[0] == "cat":
		path := parts[1]
		if !validReadPath(path) {
			return readCommandSpec{}, false
		}
		return buildReadCommandSpec(path, func(resolved string) commandFactory {
			return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "cat", resolved) }
		})
	case len(parts) == 4 && parts[0] == "sed" && parts[1] == "-n":
		if !validSedPrintRange(parts[2]) {
			return readCommandSpec{}, false
		}
		path := parts[3]
		if !validReadPath(path) {
			return readCommandSpec{}, false
		}
		return buildReadCommandSpec(path, func(resolved string) commandFactory {
			return func(ctx context.Context) *exec.Cmd {
				return exec.CommandContext(ctx, "sed", "-n", parts[2], resolved)
			}
		})
	default:
		return readCommandSpec{}, false
	}
}

func buildReadCommandSpec(path string, factory func(string) commandFactory) (readCommandSpec, bool) {
	resolved := expandHomePath(path)
	spec := readCommandSpec{
		factory: factory(resolved),
		path:    path,
		reason:  "Workspace file read command",
	}
	switch {
	case isSkillRootReadPath(path):
		spec.reason = "Skill file read command"
	case isOutsideWorkspaceReadPath(path):
		spec.requiresApproval = true
		spec.reason = "Outside-workspace file read requires approval"
	case isSecretLikeReadPath(path):
		spec.requiresApproval = true
		spec.reason = "Secret-like file read requires approval"
	}
	return spec, true
}

func validReadPath(path string) bool {
	if strings.TrimSpace(path) == "" || strings.HasSuffix(path, "/") {
		return false
	}
	if path == "." {
		return false
	}
	if hasParentTraversal(path) || hasParentTraversal(filepath.Clean(path)) {
		return false
	}
	return true
}

func hasParentTraversal(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}

func validSedPrintRange(arg string) bool {
	if !strings.HasSuffix(arg, "p") {
		return false
	}
	ranges := strings.Split(strings.TrimSuffix(arg, "p"), ",")
	if len(ranges) != 2 {
		return false
	}
	start, err := strconv.Atoi(ranges[0])
	if err != nil || start < 1 {
		return false
	}
	end, err := strconv.Atoi(ranges[1])
	if err != nil || end < start {
		return false
	}
	return true
}

func isSecretLikeReadPath(path string) bool {
	name := filepath.Base(filepath.ToSlash(filepath.Clean(path)))
	if name == ".env" || strings.HasPrefix(name, ".env.") || name == ".npmrc" || name == ".pypirc" || name == ".netrc" || name == "id_rsa" || name == "id_ed25519" {
		return true
	}
	return strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key")
}

func isOutsideWorkspaceReadPath(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "~/")
}

func isSkillRootReadPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "~/.patchpilot/skills/") || strings.HasPrefix(clean, "~/.agents/skills/") {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}
	abs := filepath.ToSlash(filepath.Clean(path))
	patchpilotRoot := filepath.ToSlash(filepath.Join(home, ".patchpilot", "skills")) + "/"
	agentsRoot := filepath.ToSlash(filepath.Join(home, ".agents", "skills")) + "/"
	return strings.HasPrefix(abs, patchpilotRoot) || strings.HasPrefix(abs, agentsRoot)
}

func expandHomePath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	return filepath.Join(home, path[2:])
}
