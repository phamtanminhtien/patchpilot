package filestore

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidPath      = errors.New("invalid workspace-relative path")
	ErrOutsideRoot      = errors.New("path escapes workspace root")
	ErrIgnoredPath      = errors.New("path is ignored")
	ErrNotTextFile      = errors.New("file is not a readable text file")
	ErrFileTooLarge     = errors.New("file exceeds max readable size")
	ErrInvalidLineRange = errors.New("invalid line range")
	ErrSecretPath       = errors.New("secret-like paths cannot be written")
	ErrSymlinkPath      = errors.New("symlink paths cannot be written")
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

type ReadOptions struct {
	StartLine int
	EndLine   int
}

type SearchOptions struct {
	ExcludePatterns []string
	IncludePatterns []string
	Path            string
}

type IndexEntry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Dir         string    `json:"dir"`
	Extension   string    `json:"extension"`
	Kind        string    `json:"kind"`
	IndexStatus string    `json:"indexStatus"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modifiedAt"`
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
	if hasDotGitPath(cleanRel) {
		return nil, ErrIgnoredPath
	}

	infos, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		if shouldSkipListEntry(info) {
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
	if entries, ok, err := gitFileIndex(abs); ok || err != nil {
		if err != nil {
			return nil, err
		}
		appendAncestorDirectoryMarkers(abs, entries)
		if err := appendSkippedDirectoryMarkers(abs, entries); err != nil {
			return nil, err
		}
		return sortedIndexEntries(entries), nil
	}

	entries := make(map[string]IndexEntry)
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
		if entry.Type()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if isIgnoredPath(rel) || isHeavyIndexName(entry.Name()) {
			if entry.IsDir() {
				if marker, ok := skippedDirectoryIndexEntry(path, rel); ok {
					entries[rel] = marker
				}
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
		entries[rel] = fileIndexEntry(rel, info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sortedIndexEntries(entries), nil
}

func (s *Service) IndexFile(root, relPath string) (IndexEntry, bool, error) {
	abs, cleanRel, err := safePath(root, relPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return IndexEntry{}, false, nil
		}
		return IndexEntry{}, false, err
	}
	cleanRel = filepath.ToSlash(cleanRel)
	if isIgnoredPath(cleanRel) {
		return IndexEntry{}, false, nil
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return IndexEntry{}, false, nil
		}
		return IndexEntry{}, false, err
	}
	if info.IsDir() || info.Mode()&fs.ModeSymlink != 0 || info.Size() > MaxReadableFileSize {
		return IndexEntry{}, false, nil
	}
	return fileIndexEntry(cleanRel, info), true, nil
}

func (s *Service) Read(root, relPath string) (File, error) {
	return s.ReadWithOptions(root, relPath, ReadOptions{})
}

func (s *Service) ReadWithOptions(root, relPath string, opts ReadOptions) (File, error) {
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
	text, err := applyLineRange(string(content), opts)
	if err != nil {
		return File{}, err
	}
	return File{Path: filepath.ToSlash(cleanRel), Content: text}, nil
}

func (s *Service) Write(root, relPath, content string) (File, error) {
	abs, cleanRel, err := safePath(root, relPath)
	if err != nil {
		return File{}, err
	}
	if isIgnoredPath(cleanRel) {
		return File{}, ErrIgnoredPath
	}
	if isSecretPath(cleanRel) {
		return File{}, ErrSecretPath
	}
	rootDir, err := os.OpenRoot(root)
	if err != nil {
		return File{}, fmt.Errorf("%w: %s", ErrInvalidPath, root)
	}
	defer rootDir.Close()
	if hasSymlinkPath(rootDir, cleanRel) {
		return File{}, ErrSymlinkPath
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return File{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return File{}, ErrSymlinkPath
	}
	if info.IsDir() {
		return File{}, ErrNotTextFile
	}
	if info.Size() > MaxReadableFileSize || int64(len([]byte(content))) > MaxReadableFileSize {
		return File{}, ErrFileTooLarge
	}
	existing, err := os.ReadFile(abs)
	if err != nil {
		return File{}, err
	}
	if !isText(existing) || !isText([]byte(content)) {
		return File{}, ErrNotTextFile
	}
	if err := os.WriteFile(abs, []byte(content), info.Mode().Perm()); err != nil {
		return File{}, err
	}
	return File{Path: filepath.ToSlash(cleanRel), Content: content}, nil
}

func (s *Service) Search(root, query string) ([]SearchResult, error) {
	return s.SearchWithOptions(root, query, SearchOptions{})
}

func (s *Service) SearchWithOptions(root, query string, opts SearchOptions) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	scope := strings.TrimSpace(opts.Path)
	if scope == "" {
		scope = "."
	}
	abs, cleanScope, err := safePath(root, scope)
	if err != nil {
		return nil, err
	}
	if isIgnoredPath(cleanScope) {
		return nil, ErrIgnoredPath
	}
	rootAbs, _, err := safePath(root, ".")
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	results := make([]SearchResult, 0)
	scopeInfo, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !scopeInfo.IsDir() {
		if !shouldSearchFile(cleanScope, filepath.Base(cleanScope), opts) {
			return results, nil
		}
		if scopeInfo.Size() > MaxReadableFileSize {
			return results, nil
		}
		if err := appendSearchFile(&results, abs, filepath.ToSlash(cleanScope), filepath.Base(cleanScope), lowerQuery); err != nil {
			return nil, err
		}
		sortSearchResults(results)
		return results, nil
	}
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == abs {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipEntry(entry) || isIgnoredPath(rel) || isSearchExcluded(rel, opts.ExcludePatterns) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && !shouldSearchFile(rel, entry.Name(), opts) {
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
		return appendSearchFile(&results, path, rel, entry.Name(), lowerQuery)
	})
	if err != nil {
		return nil, err
	}
	sortSearchResults(results)
	return results, nil
}

func applyLineRange(content string, opts ReadOptions) (string, error) {
	start := opts.StartLine
	end := opts.EndLine
	if start == 0 {
		start = 1
	}
	if start < 1 || end < 0 || (end != 0 && start > end) {
		return "", ErrInvalidLineRange
	}
	if start == 1 && end == 0 {
		return content, nil
	}
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if start > len(lines) {
		return "", nil
	}
	startIndex := start - 1
	endIndex := len(lines)
	if end != 0 && end < endIndex {
		endIndex = end
	}
	return strings.Join(lines[startIndex:endIndex], ""), nil
}

func appendSearchFile(results *[]SearchResult, absPath, relPath, name, lowerQuery string) error {
	if strings.Contains(strings.ToLower(name), lowerQuery) {
		*results = append(*results, SearchResult{Path: relPath, Kind: "filename"})
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	if !isText(content) {
		return nil
	}
	if line, preview, ok := contentMatch(content, lowerQuery); ok {
		*results = append(*results, SearchResult{Path: relPath, Kind: "content", Line: line, Preview: preview})
	}
	return nil
}

func shouldSearchFile(rel, name string, opts SearchOptions) bool {
	if isSearchExcluded(rel, opts.ExcludePatterns) {
		return false
	}
	if len(opts.IncludePatterns) == 0 {
		return true
	}
	return matchesAnySearchPattern(rel, opts.IncludePatterns) || matchesAnySearchPattern(name, opts.IncludePatterns)
}

func isSearchExcluded(rel string, patterns []string) bool {
	return matchesAnySearchPattern(rel, patterns)
}

func matchesAnySearchPattern(rel string, patterns []string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return false
	}
	for _, pattern := range patterns {
		if searchPatternMatches(pattern, rel) {
			return true
		}
	}
	return false
}

func searchPatternMatches(pattern, rel string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "/") {
		for _, segment := range strings.Split(rel, "/") {
			if segmentGlobMatches(pattern, segment) {
				return true
			}
		}
		return false
	}
	return pathSegmentsMatch(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func pathSegmentsMatch(patternParts, relParts []string) bool {
	if len(patternParts) == 0 {
		return len(relParts) == 0
	}
	if patternParts[0] == "**" {
		return pathSegmentsMatch(patternParts[1:], relParts) ||
			(len(relParts) > 0 && pathSegmentsMatch(patternParts, relParts[1:]))
	}
	if len(relParts) == 0 || !segmentGlobMatches(patternParts[0], relParts[0]) {
		return false
	}
	return pathSegmentsMatch(patternParts[1:], relParts[1:])
}

func segmentGlobMatches(pattern, value string) bool {
	return segmentGlobMatchesFrom(pattern, value, 0, 0)
}

func segmentGlobMatchesFrom(pattern, value string, patternIndex, valueIndex int) bool {
	for patternIndex < len(pattern) {
		if pattern[patternIndex] == '*' {
			for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
				patternIndex++
			}
			if patternIndex == len(pattern) {
				return true
			}
			for next := valueIndex; next <= len(value); next++ {
				if segmentGlobMatchesFrom(pattern, value, patternIndex, next) {
					return true
				}
			}
			return false
		}
		if valueIndex >= len(value) || pattern[patternIndex] != value[valueIndex] {
			return false
		}
		patternIndex++
		valueIndex++
	}
	return valueIndex == len(value)
}

func sortSearchResults(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Path == results[j].Path {
			return results[i].Kind < results[j].Kind
		}
		return results[i].Path < results[j].Path
	})
}

func gitFileIndex(root string) (map[string]IndexEntry, bool, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z")
	output, err := cmd.Output()
	if err != nil {
		return nil, false, nil
	}
	entries := make(map[string]IndexEntry)
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := filepath.ToSlash(string(raw))
		if isIgnoredPath(rel) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(abs)
		if err != nil || info.IsDir() || info.Mode()&fs.ModeSymlink != 0 || info.Size() > MaxReadableFileSize {
			continue
		}
		entries[rel] = fileIndexEntry(rel, info)
	}
	return entries, true, nil
}

func appendAncestorDirectoryMarkers(root string, entries map[string]IndexEntry) {
	for rel := range entries {
		dir := pathDir(rel)
		for dir != "" {
			if _, exists := entries[dir]; !exists {
				if marker, ok := directoryIndexEntry(filepath.Join(root, filepath.FromSlash(dir)), dir); ok {
					entries[dir] = marker
				}
			}
			dir = pathDir(dir)
		}
	}
}

func directoryIndexEntry(abs, rel string) (IndexEntry, bool) {
	info, err := os.Lstat(abs)
	if err != nil {
		return IndexEntry{}, false
	}
	return IndexEntry{
		Path:        rel,
		Name:        pathBase(rel),
		Dir:         pathDir(rel),
		Extension:   "",
		Kind:        "folder",
		IndexStatus: "indexed",
		Size:        0,
		ModifiedAt:  info.ModTime().UTC(),
	}, true
}

func appendSkippedDirectoryMarkers(root string, entries map[string]IndexEntry) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || !entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		if isIgnoredPath(rel) || isHeavyIndexName(entry.Name()) {
			if marker, ok := skippedDirectoryIndexEntry(path, rel); ok {
				entries[rel] = marker
			}
			return filepath.SkipDir
		}
		return nil
	})
}

func fileIndexEntry(rel string, info fs.FileInfo) IndexEntry {
	name := pathBase(rel)
	return IndexEntry{
		Path:        rel,
		Name:        name,
		Dir:         pathDir(rel),
		Extension:   strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
		Kind:        "file",
		IndexStatus: "indexed",
		Size:        info.Size(),
		ModifiedAt:  info.ModTime().UTC(),
	}
}

func skippedDirectoryIndexEntry(abs, rel string) (IndexEntry, bool) {
	info, err := os.Lstat(abs)
	if err != nil {
		return IndexEntry{}, false
	}
	return IndexEntry{
		Path:        rel,
		Name:        pathBase(rel),
		Dir:         pathDir(rel),
		Extension:   "",
		Kind:        "folder",
		IndexStatus: "skipped",
		Size:        0,
		ModifiedAt:  info.ModTime().UTC(),
	}, true
}

func sortedIndexEntries(entries map[string]IndexEntry) []IndexEntry {
	list := make([]IndexEntry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Path < list[j].Path
	})
	return list
}

func pathBase(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return parts[len(parts)-1]
}

func pathDir(rel string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.ToSlash(rel)))
	if dir == "." {
		return ""
	}
	return dir
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

func shouldSkipListEntry(entry fs.DirEntry) bool {
	if entry.Type()&fs.ModeSymlink != 0 {
		return true
	}
	return entry.Name() == ".git"
}

func isIgnoredName(name string) bool {
	return name == ".git" || isHeavyIndexName(name)
}

func isHeavyIndexName(name string) bool {
	switch name {
	case "node_modules", ".pnpm", ".yarn", ".next", ".nuxt", "dist", "build", "coverage", ".cache", ".turbo", ".vite":
		return true
	default:
		return false
	}
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

func hasDotGitPath(relPath string) bool {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." {
		return false
	}
	for _, part := range strings.Split(relPath, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

func isSecretPath(relPath string) bool {
	name := filepath.Base(filepath.ToSlash(filepath.Clean(relPath)))
	if name == ".env" || strings.HasPrefix(name, ".env.") || name == ".npmrc" || name == ".pypirc" || name == ".netrc" || name == "id_rsa" || name == "id_ed25519" {
		return true
	}
	return strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key")
}

func hasSymlinkPath(root *os.Root, relPath string) bool {
	cleanRel, err := filepath.Localize(filepath.ToSlash(filepath.Clean(relPath)))
	if err != nil {
		return true
	}
	if cleanRel == "." {
		return false
	}
	for _, part := range strings.Split(cleanRel, string(filepath.Separator)) {
		info, err := root.Lstat(part)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return true
		}
		if info.IsDir() {
			nextRoot, err := root.OpenRoot(part)
			if err != nil {
				return true
			}
			defer nextRoot.Close()
			root = nextRoot
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
