package filestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidPath = errors.New("invalid workspace-relative path")
	ErrOutsideRoot = errors.New("path escapes workspace root")
	ErrNotTextFile = errors.New("file is not a readable text file")
)

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) List(root, relPath string) ([]Entry, error) {
	abs, cleanRel, err := safePath(root, relPath)
	if err != nil {
		return nil, err
	}

	infos, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		fileInfo, err := info.Info()
		if err != nil {
			return nil, err
		}
		entryPath := filepath.ToSlash(filepath.Join(cleanRel, info.Name()))
		if cleanRel == "." {
			entryPath = info.Name()
		}
		entries = append(entries, Entry{
			Name:  info.Name(),
			Path:  entryPath,
			IsDir: info.IsDir(),
			Size:  fileInfo.Size(),
		})
	}
	return entries, nil
}

func (s *Service) Read(root, relPath string) (File, error) {
	abs, cleanRel, err := safePath(root, relPath)
	if err != nil {
		return File{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return File{}, err
	}
	if info.IsDir() {
		return File{}, ErrNotTextFile
	}

	content, err := os.ReadFile(abs)
	if err != nil {
		return File{}, err
	}
	if !isText(content) {
		return File{}, ErrNotTextFile
	}
	return File{Path: filepath.ToSlash(cleanRel), Content: string(content)}, nil
}

func safePath(root, relPath string) (string, string, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return "", "", ErrInvalidPath
	}
	if filepath.IsAbs(relPath) {
		return "", "", ErrInvalidPath
	}
	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || cleanRel == string(filepath.Separator) {
		cleanRel = "."
	}
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", "", ErrOutsideRoot
	}

	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidPath, root)
	}
	target := filepath.Join(rootReal, cleanRel)
	targetReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootReal, targetReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", ErrOutsideRoot
	}

	return targetReal, cleanRel, nil
}

func isText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}
