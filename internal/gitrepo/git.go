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

	output, err := runGitStatus(ctx, root, args)
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
	output, err := runGitRevParseTopLevel(ctx, root)
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
	output, err := runGitDiff(ctx, root, repositoryHasHead(ctx, root), cleanPath)
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
	if _, err := runGitAddPaths(ctx, root, cleanPaths); err != nil {
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
	if _, err := runGitResetPaths(ctx, root, cleanPaths); err != nil {
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
	if _, err := runGitAddPaths(ctx, root, cleanPaths); err != nil {
		return Commit{}, err
	}
	if _, err := runGitCommitPaths(ctx, root, message, cleanPaths); err != nil {
		return Commit{}, err
	}
	hash, err := runGitRevParseHead(ctx, root)
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
	if _, err := runGitApplyCheck(ctx, root, diff, direction); err != nil {
		return err
	}
	_, err := runGitApply(ctx, root, diff, direction)
	return err
}

func (c *Client) CheckPatch(ctx context.Context, root, diff string) error {
	if err := validateRepositoryRoot(ctx, root); err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return ErrInvalidPath
	}
	_, err := runGitApplyCheck(ctx, root, diff, ApplyForward)
	return err
}

func (c *Client) discardPath(ctx context.Context, root, cleanPath string) error {
	if isUntrackedPath(ctx, root, cleanPath) {
		return removeWorkspacePath(root, cleanPath)
	}
	_, err := runGitCheckoutPath(ctx, root, cleanPath)
	return err
}

func (c *Client) untrackedDiff(ctx context.Context, root, cleanPath string) (string, error) {
	output, err := runGitLsUntracked(ctx, root, cleanPath)
	if err != nil {
		return "", err
	}
	if !pathListed(output, cleanPath) {
		return "", nil
	}
	diff, err := runGitNoIndexDiff(ctx, root, cleanPath)
	if err != nil {
		return "", err
	}
	return diff, nil
}

func isUntrackedPath(ctx context.Context, root, cleanPath string) bool {
	output, err := runGitLsUntracked(ctx, root, cleanPath)
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
	output, err := runGitRevParseTopLevel(ctx, cleanRoot)
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
	_, err := runGitRevParseVerifyHead(ctx, root)
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

func runGitStatus(ctx context.Context, root string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitRevParseTopLevel(ctx context.Context, root string) (string, error) {
	args := []string{"rev-parse", "--show-toplevel"}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitRevParseHead(ctx context.Context, root string) (string, error) {
	args := []string{"rev-parse", "HEAD"}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitRevParseVerifyHead(ctx context.Context, root string) (string, error) {
	args := []string{"rev-parse", "--verify", "HEAD"}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD")
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitDiff(ctx context.Context, root string, hasHead bool, cleanPath string) (string, error) {
	if hasHead {
		args := []string{"diff", "HEAD", "--"}
		if cleanPath != "" {
			args = append(args, cleanPath)
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		output, _, err := runPreparedGit(cmd, root, args, "")
		return output, err
	}
	args := []string{"diff", "--cached", "--"}
	if cleanPath != "" {
		args = append(args, cleanPath)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitAddPaths(ctx context.Context, root string, cleanPaths []string) (string, error) {
	args := append([]string{"add", "--"}, cleanPaths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitResetPaths(ctx context.Context, root string, cleanPaths []string) (string, error) {
	args := append([]string{"reset", "--"}, cleanPaths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitCommitPaths(ctx context.Context, root, message string, cleanPaths []string) (string, error) {
	args := append([]string{"commit", "-F", "-", "--"}, cleanPaths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	output, _, err := runPreparedGit(cmd, root, args, message)
	return output, err
}

func runGitApplyCheck(ctx context.Context, root, diff string, direction ApplyDirection) (string, error) {
	if direction == ApplyReverse {
		args := []string{"apply", "--check", "--reverse"}
		cmd := exec.CommandContext(ctx, "git", "apply", "--check", "--reverse")
		output, _, err := runPreparedGit(cmd, root, args, diff)
		return output, err
	}
	args := []string{"apply", "--check"}
	cmd := exec.CommandContext(ctx, "git", "apply", "--check")
	output, _, err := runPreparedGit(cmd, root, args, diff)
	return output, err
}

func runGitApply(ctx context.Context, root, diff string, direction ApplyDirection) (string, error) {
	if direction == ApplyReverse {
		args := []string{"apply", "--reverse"}
		cmd := exec.CommandContext(ctx, "git", "apply", "--reverse")
		output, _, err := runPreparedGit(cmd, root, args, diff)
		return output, err
	}
	args := []string{"apply"}
	cmd := exec.CommandContext(ctx, "git", "apply")
	output, _, err := runPreparedGit(cmd, root, args, diff)
	return output, err
}

func runGitCheckoutPath(ctx context.Context, root, cleanPath string) (string, error) {
	args := []string{"checkout", "--", cleanPath}
	cmd := exec.CommandContext(ctx, "git", "checkout", "--", cleanPath)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitLsUntracked(ctx context.Context, root, cleanPath string) (string, error) {
	args := []string{"ls-files", "--others", "--exclude-standard", "--", cleanPath}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard", "--", cleanPath)
	output, _, err := runPreparedGit(cmd, root, args, "")
	return output, err
}

func runGitNoIndexDiff(ctx context.Context, root, cleanPath string) (string, error) {
	args := []string{"diff", "--no-index", "--", os.DevNull, cleanPath}
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--", os.DevNull, cleanPath)
	output, exitCode, err := runPreparedGit(cmd, root, args, "")
	if err == nil || exitCode == 1 {
		return output, nil
	}
	return "", err
}

func runPreparedGit(cmd *exec.Cmd, root string, args []string, input string) (string, int, error) {
	cmd.Dir = root
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

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
