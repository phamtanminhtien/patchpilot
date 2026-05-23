package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrEmptyCommitMessage = errors.New("commit message is required")
	ErrEmptyPathList      = errors.New("at least one path is required")
	ErrInvalidPath        = errors.New("invalid workspace-relative path")
	ErrInvalidRoot        = errors.New("invalid workspace root")
	ErrGitFailed          = errors.New("git command failed")
	ErrNotRepository      = errors.New("not a git repository")
)

type Client struct{}

type Status struct {
	Porcelain string `json:"porcelain"`
}

type StatusOptions struct {
	Ignored          bool     `json:"ignored"`
	Untracked        string   `json:"untracked"`         // "all", "normal", "no"
	IgnoreSubmodules string   `json:"ignore_submodules"` // "none", "untracked", "dirty", "all"
	Paths            []string `json:"paths"`
}

type Diff struct {
	Path string `json:"path,omitempty"`
	Diff string `json:"diff"`
}

type Commit struct {
	Hash string `json:"hash"`
}

type ApplyDirection string

const (
	ApplyForward ApplyDirection = "forward"
	ApplyReverse ApplyDirection = "reverse"
)

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Status(ctx context.Context, root string, opts StatusOptions) (Status, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Status{}, err
	}
	args := []string{"status", "--porcelain=v1"}

	untracked := opts.Untracked
	if untracked == "" {
		untracked = "all"
	}
	switch untracked {
	case "all", "normal", "no":
		args = append(args, "--untracked-files="+untracked)
	default:
		return Status{}, fmt.Errorf("invalid untracked option: %s", untracked)
	}

	if opts.Ignored {
		args = append(args, "--ignored")
	}

	if opts.IgnoreSubmodules != "" {
		switch opts.IgnoreSubmodules {
		case "none", "untracked", "dirty", "all":
			args = append(args, "--ignore-submodules="+opts.IgnoreSubmodules)
		default:
			return Status{}, fmt.Errorf("invalid ignore_submodules option: %s", opts.IgnoreSubmodules)
		}
	}

	args = append(args, "--")

	if len(opts.Paths) > 0 {
		cleaned, err := cleanRelativePaths(opts.Paths)
		if err != nil {
			return Status{}, err
		}
		args = append(args, cleaned...)
	}

	output, err := runGit(ctx, root, args...)
	if err != nil {
		return Status{}, err
	}

	lines := strings.Split(output, "\n")
	filteredLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if shouldFilterStatusLine(line) {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	const maxLines = 1000
	if len(filteredLines) > maxLines {
		truncated := make([]string, maxLines)
		copy(truncated, filteredLines[:maxLines])
		truncated = append(truncated, "!! (status output truncated: too many files)")
		filteredLines = truncated
	}

	return Status{Porcelain: strings.Join(filteredLines, "\n")}, nil
}

func shouldFilterStatusLine(line string) bool {
	if len(line) < 4 {
		return false
	}
	status := line[:2]
	if status != "??" && status != "!!" {
		return false
	}
	pathPart := line[3:]
	if len(pathPart) >= 2 && pathPart[0] == '"' && pathPart[len(pathPart)-1] == '"' {
		pathPart = pathPart[1 : len(pathPart)-1]
	}
	pathPart = filepath.ToSlash(pathPart)
	segments := strings.Split(pathPart, "/")
	for _, seg := range segments {
		segLower := strings.ToLower(seg)
		switch segLower {
		case "node_modules", ".git", ".pnpm-store", ".pnpm", "build", "dist", ".next", ".nuxt", ".docusaurus", ".svelte-kit", "tmp", "temp", ".cache", "coverage", ".nyc_output", "bower_components", ".yarn", ".cargo", ".idea", ".vscode":
			return true
		}
	}
	return false
}

func (c *Client) RepositoryRoot(ctx context.Context, root string) (string, error) {
	output, err := runRawGit(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotRepository
	}
	repositoryRoot, err := filepath.EvalSymlinks(filepath.Clean(strings.TrimSpace(output)))
	if err != nil {
		return "", err
	}
	return repositoryRoot, nil
}

func (c *Client) Diff(ctx context.Context, root, relPath string) (Diff, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Diff{}, err
	}
	cleanPath, err := cleanRelativePath(relPath)
	if err != nil {
		return Diff{}, err
	}
	args := []string{"diff"}
	if repositoryHasHead(ctx, root) {
		args = append(args, "HEAD")
	} else {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	if cleanPath != "" {
		args = append(args, cleanPath)
	}
	output, err := runGit(ctx, root, args...)
	if err != nil {
		return Diff{}, err
	}
	if output == "" && cleanPath != "" {
		untrackedDiff, err := c.untrackedDiff(ctx, root, cleanPath)
		if err != nil {
			return Diff{}, err
		}
		output = untrackedDiff
	}
	return Diff{Path: cleanPath, Diff: output}, nil
}

func (c *Client) Stage(ctx context.Context, root string, relPaths []string) (Status, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Status{}, err
	}
	cleanPaths, err := cleanRelativePaths(relPaths)
	if err != nil {
		return Status{}, err
	}
	args := append([]string{"add", "--"}, cleanPaths...)
	if _, err := runGit(ctx, root, args...); err != nil {
		return Status{}, err
	}
	return c.Status(ctx, root, StatusOptions{})
}

func (c *Client) Unstage(ctx context.Context, root string, relPaths []string) (Status, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Status{}, err
	}
	cleanPaths, err := cleanRelativePaths(relPaths)
	if err != nil {
		return Status{}, err
	}
	args := append([]string{"reset", "--"}, cleanPaths...)
	if _, err := runGit(ctx, root, args...); err != nil {
		return Status{}, err
	}
	return c.Status(ctx, root, StatusOptions{})
}

func (c *Client) Discard(ctx context.Context, root string, relPaths []string) (Status, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Status{}, err
	}
	cleanPaths, err := cleanRelativePaths(relPaths)
	if err != nil {
		return Status{}, err
	}
	for _, cleanPath := range cleanPaths {
		if err := c.discardPath(ctx, root, cleanPath); err != nil {
			return Status{}, err
		}
	}
	return c.Status(ctx, root, StatusOptions{})
}

func (c *Client) Commit(ctx context.Context, root, message string, relPaths []string) (Commit, error) {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return Commit{}, err
	}
	if strings.TrimSpace(message) == "" {
		return Commit{}, ErrEmptyCommitMessage
	}
	cleanPaths, err := cleanRelativePaths(relPaths)
	if err != nil {
		return Commit{}, err
	}
	addArgs := append([]string{"add", "--"}, cleanPaths...)
	if _, err := runGit(ctx, root, addArgs...); err != nil {
		return Commit{}, err
	}
	commitArgs := append([]string{"commit", "-m", message, "--"}, cleanPaths...)
	if _, err := runGit(ctx, root, commitArgs...); err != nil {
		return Commit{}, err
	}
	hash, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Commit{}, err
	}
	return Commit{Hash: strings.TrimSpace(hash)}, nil
}

func (c *Client) ApplyPatch(ctx context.Context, root, diff string, direction ApplyDirection) error {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return ErrInvalidPath
	}
	checkArgs := []string{"apply", "--check"}
	applyArgs := []string{"apply"}
	if direction == ApplyReverse {
		checkArgs = append(checkArgs, "--reverse")
		applyArgs = append(applyArgs, "--reverse")
	}
	if _, err := runGitWithInput(ctx, root, diff, checkArgs...); err != nil {
		return err
	}
	_, err := runGitWithInput(ctx, root, diff, applyArgs...)
	return err
}

func (c *Client) CheckPatch(ctx context.Context, root, diff string) error {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return ErrInvalidPath
	}
	_, err := runGitWithInput(ctx, root, diff, "apply", "--check")
	return err
}

func (c *Client) discardPath(ctx context.Context, root, cleanPath string) error {
	if isUntrackedPath(ctx, root, cleanPath) {
		return removeWorkspacePath(root, cleanPath)
	}
	_, err := runGit(ctx, root, "checkout", "--", cleanPath)
	return err
}

func (c *Client) untrackedDiff(ctx context.Context, root, cleanPath string) (string, error) {
	output, err := runGit(ctx, root, "ls-files", "--others", "--exclude-standard", "--", cleanPath)
	if err != nil {
		return "", err
	}
	if !pathListed(output, cleanPath) {
		return "", nil
	}
	diff, err := runGitAllowExit(ctx, root, 1, "diff", "--no-index", "--", os.DevNull, cleanPath)
	if err != nil {
		return "", err
	}
	return diff, nil
}

func isUntrackedPath(ctx context.Context, root, cleanPath string) bool {
	output, err := runGit(ctx, root, "ls-files", "--others", "--exclude-standard", "--", cleanPath)
	if err != nil {
		return false
	}
	return pathListed(output, cleanPath) || pathPrefixListed(output, cleanPath)
}

func validateRepositoryRoot(ctx context.Context, root string) error {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return ErrInvalidRoot
	}
	cleanRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidRoot, root)
	}
	output, err := runRawGit(ctx, cleanRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return ErrNotRepository
	}
	repositoryRoot, err := filepath.EvalSymlinks(filepath.Clean(strings.TrimSpace(output)))
	if err != nil {
		return err
	}
	if repositoryRoot != cleanRoot {
		return ErrNotRepository
	}
	return nil
}

func repositoryHasHead(ctx context.Context, root string) bool {
	_, err := runGit(ctx, root, "rev-parse", "--verify", "HEAD")
	return err == nil
}

func cleanRelativePath(relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", nil
	}
	if filepath.IsAbs(relPath) {
		return "", ErrInvalidPath
	}
	clean := filepath.Clean(relPath)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return filepath.ToSlash(clean), nil
}

func cleanRelativePaths(relPaths []string) ([]string, error) {
	if len(relPaths) == 0 {
		return nil, ErrEmptyPathList
	}
	seen := map[string]struct{}{}
	cleanPaths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		if strings.TrimSpace(relPath) == "" {
			return nil, ErrInvalidPath
		}
		cleanPath, err := cleanRelativePath(relPath)
		if err != nil {
			return nil, err
		}
		if cleanPath == "" {
			return nil, ErrInvalidPath
		}
		if _, ok := seen[cleanPath]; ok {
			continue
		}
		seen[cleanPath] = struct{}{}
		cleanPaths = append(cleanPaths, cleanPath)
	}
	if len(cleanPaths) == 0 {
		return nil, ErrEmptyPathList
	}
	return cleanPaths, nil
}

func removeWorkspacePath(root, cleanPath string) error {
	target := filepath.Join(root, filepath.FromSlash(cleanPath))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if absTarget != absRoot && !strings.HasPrefix(absTarget, absRoot+string(filepath.Separator)) {
		return ErrInvalidPath
	}
	return os.RemoveAll(absTarget)
}

func pathListed(output, cleanPath string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == cleanPath {
			return true
		}
	}
	return false
}

func pathPrefixListed(output, cleanPath string) bool {
	prefix := strings.TrimSuffix(cleanPath, "/") + "/"
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	output, _, err := runGitCommand(ctx, root, args...)
	return output, err
}

func runRawGit(ctx context.Context, root string, args ...string) (string, error) {
	return runGit(ctx, root, args...)
}

func runGitAllowExit(ctx context.Context, root string, allowedExitCode int, args ...string) (string, error) {
	output, exitCode, err := runGitCommand(ctx, root, args...)
	if err == nil || exitCode == allowedExitCode {
		return output, nil
	}
	return "", err
}

func runGitCommand(ctx context.Context, root string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if stderr.Len() > 0 {
			return stdout.String(), exitCode, fmt.Errorf("%w: git %v: %w: %s", ErrGitFailed, args, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), exitCode, fmt.Errorf("%w: git %v: %w", ErrGitFailed, args, err)
	}
	return stdout.String(), 0, nil
}

func runGitWithInput(ctx context.Context, root, input string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return stdout.String(), fmt.Errorf("%w: git %v: %w: %s", ErrGitFailed, args, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), fmt.Errorf("%w: git %v: %w", ErrGitFailed, args, err)
	}
	return stdout.String(), nil
}
