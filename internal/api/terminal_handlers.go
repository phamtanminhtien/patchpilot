package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	terminalsvc "github.com/phamtanminhtien/patchpilot/internal/terminal"
	"nhooyr.io/websocket"
)

type createTerminalSessionRequest struct {
	Title string `json:"title"`
	Rows  int    `json:"rows"`
	Cols  int    `json:"cols"`
}

type patchTerminalSessionRequest struct {
	Title *string `json:"title"`
	Rows  *int    `json:"rows"`
	Cols  *int    `json:"cols"`
}

type terminalClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Rows int    `json:"rows"`
	Cols int    `json:"cols"`
}

type terminalServerMessage struct {
	Type     string                   `json:"type"`
	Data     string                   `json:"data,omitempty"`
	Session  *terminalSessionResponse `json:"session,omitempty"`
	ExitCode *int                     `json:"exitCode,omitempty"`
	Message  string                   `json:"message,omitempty"`
}

func (s *Server) listTerminalSessions(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	sessions, err := s.store.ListTerminalSessions(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_session_list_failed", "Terminal sessions could not be listed", nil)
		return
	}
	response := terminalSessionResponsesFromRecords(sessions)
	page, nextCursor, ok := paginateItems(w, response, pagination, func(session terminalSessionResponse) string {
		return session.ID
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": page, "nextCursor": nextCursor})
}

func (s *Server) createTerminalSession(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req createTerminalSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	session, err := s.terminal.Create(r.Context(), ws.ID, ws.RootPath, terminalsvc.CreateOptions{Title: req.Title, Rows: req.Rows, Cols: req.Cols})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_session_create_failed", "Terminal session could not be created", nil)
		return
	}
	if session.PID != nil {
		s.startTerminalPortScan(ws.ID, session.ID, *session.PID)
	}
	response := terminalSessionResponseFromRecord(session)
	s.events.Publish(events.Event{WorkspaceID: ws.ID, Type: "terminal.session.created", Payload: response})
	writeJSON(w, http.StatusCreated, map[string]any{"session": response})
}

func (s *Server) patchTerminalSession(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req patchTerminalSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	session, err := s.terminal.Patch(r.Context(), ws.ID, r.PathValue("sessionId"), req.Title, req.Rows, req.Cols)
	if err != nil {
		writeTerminalError(w, err)
		return
	}
	response := terminalSessionResponseFromRecord(session)
	s.events.Publish(events.Event{WorkspaceID: ws.ID, Type: "terminal.session.updated", Payload: response})
	writeJSON(w, http.StatusOK, map[string]any{"session": response})
}

func (s *Server) closeTerminalSession(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	session, err := s.terminal.Close(r.Context(), ws.ID, r.PathValue("sessionId"))
	if err != nil {
		writeTerminalError(w, err)
		return
	}
	response := terminalSessionResponseFromRecord(session)
	writeJSON(w, http.StatusOK, map[string]any{"session": response})
}

func (s *Server) terminalSocket(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	session, replay, output, unsubscribe, err := s.terminal.Subscribe(ws.ID, r.PathValue("sessionId"))
	if err != nil {
		writeTerminalError(w, err)
		return
	}
	defer unsubscribe()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "terminal socket closed")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	writer := &terminalWSWriter{conn: conn}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		_ = writer.Write(ctx, terminalServerMessage{Type: "ready", Session: responsePtr(terminalSessionResponseFromRecord(session))})
		if len(replay) > 0 {
			_ = writer.Write(ctx, terminalServerMessage{Type: "output", Data: string(replay)})
		}
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-output:
				if !ok {
					return
				}
				message := terminalServerMessage{Type: event.Type, Data: event.Data, ExitCode: event.ExitCode}
				if event.Type == "error" {
					message.Message = event.Data
					message.Data = ""
				}
				if err := writer.Write(ctx, message); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		_, reader, err := conn.Reader(ctx)
		if err != nil {
			cancel()
			break
		}
		var message terminalClientMessage
		if err := json.NewDecoder(reader).Decode(&message); err != nil {
			_ = writer.Write(ctx, terminalServerMessage{Type: "error", Message: "Invalid terminal message"})
			continue
		}
		switch message.Type {
		case "input":
			if err := s.terminal.WriteInput(ws.ID, session.ID, message.Data); err != nil {
				_ = writer.Write(ctx, terminalServerMessage{Type: "error", Message: "Terminal input failed"})
			}
		case "resize":
			updated, err := s.terminal.Patch(r.Context(), ws.ID, session.ID, nil, &message.Rows, &message.Cols)
			if err != nil {
				_ = writer.Write(ctx, terminalServerMessage{Type: "error", Message: "Terminal resize failed"})
				continue
			}
			s.events.Publish(events.Event{WorkspaceID: ws.ID, Type: "terminal.session.updated", Payload: terminalSessionResponseFromRecord(updated)})
		case "ping":
			_ = writer.Write(ctx, terminalServerMessage{Type: "ready", Session: responsePtr(terminalSessionResponseFromRecord(session))})
		default:
			_ = writer.Write(ctx, terminalServerMessage{Type: "error", Message: "Unsupported terminal message"})
		}
	}
	<-writerDone
}

type terminalWSWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *terminalWSWriter) Write(ctx context.Context, message terminalServerMessage) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	return wsjsonWrite(ctx, w.conn, message)
}

func wsjsonWrite(ctx context.Context, conn *websocket.Conn, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, payload)
}

func responsePtr(response terminalSessionResponse) *terminalSessionResponse {
	return &response
}

func writeTerminalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, "terminal_session_not_found", "Terminal session was not found", nil)
	case errors.Is(err, terminalsvc.ErrSessionClosed):
		writeError(w, http.StatusConflict, "terminal_session_closed", "Terminal session is closed", nil)
	default:
		writeError(w, http.StatusInternalServerError, "terminal_session_failed", "Terminal session request failed", nil)
	}
}

func (s *Server) startTerminalPortScan(workspaceID, sessionID string, pid int) {
	ctx, cancel := context.WithCancel(context.Background())
	s.terminalScansMu.Lock()
	if previous := s.terminalScans[sessionID]; previous != nil {
		previous()
	}
	s.terminalScans[sessionID] = cancel
	s.terminalScansMu.Unlock()
	go s.pollListeningPorts(ctx, workspaceID, sessionID, pid)
}

func (s *Server) onTerminalClosed(session database.TerminalSessionRecord) {
	s.terminalScansMu.Lock()
	if cancel := s.terminalScans[session.ID]; cancel != nil {
		cancel()
		delete(s.terminalScans, session.ID)
	}
	s.terminalScansMu.Unlock()
	s.events.Publish(events.Event{WorkspaceID: session.WorkspaceID, Type: "terminal.session.closed", Payload: terminalSessionResponseFromRecord(session)})
}
