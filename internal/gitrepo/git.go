package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrInvalidPath = errors.New("invalid workspace-relative path")
var ErrNotRepository = errors.New("not a git repository")

type Client struct{}

type Status struct {
	Porcelain string `json:"porcelain"`
}

type Diff struct {
	Path string `json:"path,omitempty"`
	Diff string `json:"diff"`
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Status(ctx context.Context, root string) (Status, error) {
	output, err := runGit(ctx, root, "status", "--porcelain=v1")
	if err != nil {
		return Status{}, err
	}
	return Status{Porcelain: output}, nil
}

func (c *Client) RepositoryRoot(ctx context.Context, root string) (string, error) {
	output, err := runGit(ctx, root, "rev-parse", "--show-toplevel")
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
	cleanPath, err := cleanRelativePath(relPath)
	if err != nil {
		return Diff{}, err
	}
	args := []string{"diff", "--"}
	if cleanPath != "" {
		args = append(args, cleanPath)
	}
	output, err := runGit(ctx, root, args...)
	if err != nil {
		return Diff{}, err
	}
	return Diff{Path: cleanPath, Diff: output}, nil
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

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
		}
		return "", err
	}
	return stdout.String(), nil
}
