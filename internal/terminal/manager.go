package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/phamtanminhtien/patchpilot/internal/database"
)

const (
	DefaultRows = 24
	DefaultCols = 80
	MaxReplay   = 1024 * 1024
)

var ErrSessionClosed = errors.New("terminal session is closed")

type Store interface {
	CreateTerminalSession(context.Context, database.TerminalSessionRecord) (database.TerminalSessionRecord, error)
	GetTerminalSession(context.Context, string, string) (database.TerminalSessionRecord, error)
	UpdateTerminalSession(context.Context, string, string, map[string]any) (database.TerminalSessionRecord, error)
	CloseTerminalSession(context.Context, string, string, string, *int, time.Time) (database.TerminalSessionRecord, error)
}

type Manager struct {
	store    Store
	onClosed func(database.TerminalSessionRecord)

	mu       sync.Mutex
	sessions map[string]*runtimeSession
}

type runtimeSession struct {
	record      database.TerminalSessionRecord
	cmd         *exec.Cmd
	ptyFile     *os.File
	replay      []byte
	subscribers map[chan Event]struct{}
	closed      bool
	mu          sync.Mutex
}

type Event struct {
	Type     string
	Data     string
	ExitCode *int
}

type CreateOptions struct {
	Title string
	Rows  int
	Cols  int
}

func NewManager(store Store, onClosed func(database.TerminalSessionRecord)) *Manager {
	return &Manager{store: store, onClosed: onClosed, sessions: map[string]*runtimeSession{}}
}

func (m *Manager) Create(ctx context.Context, workspaceID, cwd string, options CreateOptions) (database.TerminalSessionRecord, error) {
	rows, cols := normalizeSize(options.Rows, options.Cols)
	title := strings.TrimSpace(options.Title)
	if title == "" {
		title = "Terminal"
	}
	shell, err := shellPath()
	if err != nil {
		return database.TerminalSessionRecord{}, err
	}
	cmd := exec.CommandContext(context.Background(), shell)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return database.TerminalSessionRecord{}, err
	}
	pid := cmd.Process.Pid
	record, err := m.store.CreateTerminalSession(ctx, database.TerminalSessionRecord{
		WorkspaceID: workspaceID,
		Title:       title,
		Cwd:         cwd,
		Status:      "open",
		PID:         &pid,
		Rows:        rows,
		Cols:        cols,
	})
	if err != nil {
		_ = ptyFile.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return database.TerminalSessionRecord{}, err
	}
	runtime := &runtimeSession{record: record, cmd: cmd, ptyFile: ptyFile, subscribers: map[chan Event]struct{}{}}
	m.mu.Lock()
	m.sessions[record.ID] = runtime
	m.mu.Unlock()
	go m.readLoop(runtime)
	go m.waitLoop(runtime)
	return record, nil
}

func (m *Manager) Patch(ctx context.Context, workspaceID, sessionID string, title *string, rows, cols *int) (database.TerminalSessionRecord, error) {
	updates := map[string]any{}
	if title != nil {
		updates["title"] = strings.TrimSpace(*title)
	}
	if rows != nil || cols != nil {
		record, err := m.store.GetTerminalSession(ctx, workspaceID, sessionID)
		if err != nil {
			return database.TerminalSessionRecord{}, err
		}
		nextRows := record.Rows
		nextCols := record.Cols
		if rows != nil {
			nextRows = *rows
		}
		if cols != nil {
			nextCols = *cols
		}
		nextRows, nextCols = normalizeSize(nextRows, nextCols)
		updates["rows"] = nextRows
		updates["cols"] = nextCols
		_ = m.Resize(workspaceID, sessionID, nextRows, nextCols)
	}
	if len(updates) == 0 {
		return m.store.GetTerminalSession(ctx, workspaceID, sessionID)
	}
	return m.store.UpdateTerminalSession(ctx, workspaceID, sessionID, updates)
}

func (m *Manager) Subscribe(workspaceID, sessionID string) (database.TerminalSessionRecord, []byte, <-chan Event, func(), error) {
	m.mu.Lock()
	runtime := m.sessions[sessionID]
	m.mu.Unlock()
	if runtime == nil || runtime.record.WorkspaceID != workspaceID {
		record, err := m.store.GetTerminalSession(context.Background(), workspaceID, sessionID)
		if err != nil {
			return database.TerminalSessionRecord{}, nil, nil, nil, err
		}
		return record, nil, nil, nil, ErrSessionClosed
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.closed {
		return runtime.record, append([]byte(nil), runtime.replay...), nil, nil, ErrSessionClosed
	}
	ch := make(chan Event, 64)
	runtime.subscribers[ch] = struct{}{}
	replay := append([]byte(nil), runtime.replay...)
	unsubscribe := func() {
		runtime.mu.Lock()
		if _, ok := runtime.subscribers[ch]; ok {
			delete(runtime.subscribers, ch)
			close(ch)
		}
		runtime.mu.Unlock()
	}
	return runtime.record, replay, ch, unsubscribe, nil
}

func (m *Manager) WriteInput(workspaceID, sessionID, input string) error {
	runtime, err := m.runtime(workspaceID, sessionID)
	if err != nil {
		return err
	}
	_, err = runtime.ptyFile.Write([]byte(input))
	return err
}

func (m *Manager) Resize(workspaceID, sessionID string, rows, cols int) error {
	runtime, err := m.runtime(workspaceID, sessionID)
	if err != nil {
		return err
	}
	rows, cols = normalizeSize(rows, cols)
	return pty.Setsize(runtime.ptyFile, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (m *Manager) Close(ctx context.Context, workspaceID, sessionID string) (database.TerminalSessionRecord, error) {
	m.mu.Lock()
	runtime := m.sessions[sessionID]
	m.mu.Unlock()
	if runtime == nil || runtime.record.WorkspaceID != workspaceID {
		record, err := m.store.GetTerminalSession(ctx, workspaceID, sessionID)
		if err != nil {
			return database.TerminalSessionRecord{}, err
		}
		return record, nil
	}
	_ = runtime.ptyFile.Close()
	if runtime.cmd.Process != nil {
		_ = runtime.cmd.Process.Kill()
	}
	return m.finish(runtime, "closed", nil)
}

func (m *Manager) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	runtimes := make([]*runtimeSession, 0, len(m.sessions))
	for _, runtime := range m.sessions {
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	for _, runtime := range runtimes {
		_, err := m.Close(ctx, runtime.record.WorkspaceID, runtime.record.ID)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return err
		}
	}
	return nil
}

func (m *Manager) runtime(workspaceID, sessionID string) (*runtimeSession, error) {
	m.mu.Lock()
	runtime := m.sessions[sessionID]
	m.mu.Unlock()
	if runtime == nil || runtime.record.WorkspaceID != workspaceID {
		return nil, ErrSessionClosed
	}
	runtime.mu.Lock()
	closed := runtime.closed
	runtime.mu.Unlock()
	if closed {
		return nil, ErrSessionClosed
	}
	return runtime, nil
}

func (m *Manager) readLoop(runtime *runtimeSession) {
	buf := make([]byte, 8192)
	for {
		n, err := runtime.ptyFile.Read(buf)
		if n > 0 {
			runtime.appendOutput(string(buf[:n]))
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				runtime.broadcast(Event{Type: "error", Data: err.Error()})
			}
			return
		}
	}
}

func (m *Manager) waitLoop(runtime *runtimeSession) {
	err := runtime.cmd.Wait()
	status := "closed"
	if err != nil {
		status = "failed"
	}
	var exitCode *int
	if runtime.cmd.ProcessState != nil {
		code := runtime.cmd.ProcessState.ExitCode()
		exitCode = &code
		if status == "failed" && code >= 0 {
			status = "closed"
		}
	}
	_, _ = m.finish(runtime, status, exitCode)
}

func (m *Manager) finish(runtime *runtimeSession, status string, exitCode *int) (database.TerminalSessionRecord, error) {
	runtime.mu.Lock()
	if runtime.closed {
		record := runtime.record
		runtime.mu.Unlock()
		return record, nil
	}
	runtime.closed = true
	runtime.mu.Unlock()

	_ = runtime.ptyFile.Close()
	record, err := m.store.CloseTerminalSession(context.Background(), runtime.record.WorkspaceID, runtime.record.ID, status, exitCode, time.Now().UTC())
	if err == nil {
		runtime.record = record
	}
	runtime.closeSubscribers(exitCode)
	m.mu.Lock()
	delete(m.sessions, runtime.record.ID)
	m.mu.Unlock()
	if err == nil && m.onClosed != nil {
		m.onClosed(record)
	}
	return record, err
}

func (r *runtimeSession) appendOutput(chunk string) {
	r.mu.Lock()
	r.replay = append(r.replay, []byte(chunk)...)
	if len(r.replay) > MaxReplay {
		r.replay = append([]byte(nil), r.replay[len(r.replay)-MaxReplay:]...)
	}
	r.mu.Unlock()
	r.broadcast(Event{Type: "output", Data: chunk})
}

func (r *runtimeSession) broadcast(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for subscriber := range r.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (r *runtimeSession) closeSubscribers(exitCode *int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for subscriber := range r.subscribers {
		select {
		case subscriber <- Event{Type: "closed", ExitCode: exitCode}:
		default:
		}
		close(subscriber)
		delete(r.subscribers, subscriber)
	}
}

func normalizeSize(rows, cols int) (int, int) {
	if rows <= 0 {
		rows = DefaultRows
	}
	if cols <= 0 {
		cols = DefaultCols
	}
	return rows, cols
}

func shellPath() (string, error) {
	candidates := []string{strings.TrimSpace(os.Getenv("SHELL")), "/bin/zsh", "/bin/bash", "/bin/sh"}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no supported shell found")
}
