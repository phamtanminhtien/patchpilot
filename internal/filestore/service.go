package filestore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidPath  = errors.New("invalid workspace-relative path")
	ErrOutsideRoot  = errors.New("path escapes workspace root")
	ErrIgnoredPath  = errors.New("path is ignored")
	ErrNotTextFile  = errors.New("file is not a readable text file")
	ErrFileTooLarge = errors.New("file exceeds max readable size")
)

const MaxReadableFileSize int64 = 1 << 20

type Entry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	IsDir      bool      `json:"isDir"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SearchResult struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Line    int    `json:"line,omitempty"`
	Preview string `json:"preview,omitempty"`
}

type IndexEntry struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
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
	if isIgnoredPath(cleanRel) {
		return nil, ErrIgnoredPath
	}

	infos, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		if shouldSkipEntry(info) {
			continue
		}
		fileInfo, err := info.Info()
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && fileInfo.Size() > MaxReadableFileSize {
			continue
		}
		entryPath := filepath.ToSlash(filepath.Join(cleanRel, info.Name()))
		if cleanRel == "." {
			entryPath = info.Name()
		}
		entries = append(entries, Entry{
			Name:       info.Name(),
			Path:       entryPath,
			IsDir:      info.IsDir(),
			Size:       fileInfo.Size(),
			ModifiedAt: fileInfo.ModTime().UTC(),
		})
	}
	return entries, nil
}

func (s *Service) Index(root string) ([]IndexEntry, error) {
	abs, _, err := safePath(root, ".")
	if err != nil {
		return nil, err
	}

	entries := make([]IndexEntry, 0)
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == abs {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipEntry(entry) || isIgnoredPath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxReadableFileSize {
			return nil
		}
		entries = append(entries, IndexEntry{
			Path:       rel,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func (s *Service) Read(root, relPath string) (File, error) {
	abs, cleanRel, err := safePath(root, relPath)
	if err != nil {
		return File{}, err
	}
	if isIgnoredPath(cleanRel) {
		return File{}, ErrIgnoredPath
	}
	info, err := os.Stat(abs)
	if err != nil {
		return File{}, err
	}
	if info.IsDir() {
		return File{}, ErrNotTextFile
	}
	if info.Size() > MaxReadableFileSize {
		return File{}, ErrFileTooLarge
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

func (s *Service) Search(root, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	abs, _, err := safePath(root, ".")
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	results := make([]SearchResult, 0)
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == abs {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipEntry(entry) || isIgnoredPath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if info.Size() > MaxReadableFileSize {
			return nil
		}
		nameMatches := strings.Contains(strings.ToLower(entry.Name()), lowerQuery)
		if nameMatches {
			results = append(results, SearchResult{Path: rel, Kind: "filename"})
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isText(content) {
			return nil
		}
		if line, preview, ok := contentMatch(content, lowerQuery); ok {
			results = append(results, SearchResult{Path: rel, Kind: "content", Line: line, Preview: preview})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Path == results[j].Path {
			return results[i].Kind < results[j].Kind
		}
		return results[i].Path < results[j].Path
	})
	return results, nil
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

func shouldSkipEntry(entry fs.DirEntry) bool {
	if entry.Type()&fs.ModeSymlink != 0 {
		return true
	}
	return isIgnoredName(entry.Name())
}

func isIgnoredName(name string) bool {
	return name == ".git" || name == "node_modules" || name == "build"
}

func isIgnoredPath(relPath string) bool {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." {
		return false
	}
	for _, part := range strings.Split(relPath, "/") {
		if isIgnoredName(part) {
			return true
		}
	}
	return false
}

func isText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}

func contentMatch(content []byte, lowerQuery string) (int, string, bool) {
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			return index + 1, strings.TrimSpace(line), true
		}
	}
	return 0, "", false
}
