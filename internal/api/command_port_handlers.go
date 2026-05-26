package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/ports"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type createCommandRequest struct {
	Command   string `json:"command"`
	Confirmed bool   `json:"confirmed"`
}

func (s *Server) createCommand(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req createCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	decision, err := runner.Classify(req.Command)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_command", "Command is required", nil)
		return
	}
	if decision.Level == runner.SafetyBlocked {
		writeError(w, http.StatusBadRequest, "blocked_command", decision.Reason, map[string]any{"decision": decision})
		return
	}
	if decision.Level == runner.SafetyNeedsConfirmation && !req.Confirmed {
		writeError(w, http.StatusConflict, "confirmation_required", "Command requires confirmation", map[string]any{"decision": decision})
		return
	}
	created, err := s.store.CreateCommand(r.Context(), database.CommandRecord{
		WorkspaceID: ws.ID,
		Command:     req.Command,
		Cwd:         ws.RootPath,
		Status:      "queued",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "command_create_failed", "Command could not be created", nil)
		return
	}
	if err := s.runner.Start(runner.RunSpec{
		ID:          created.ID,
		WorkspaceID: ws.ID,
		Command:     created.Command,
		Cwd:         created.Cwd,
	}, s.commandHooks(ws.ID, created.ID)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_command", "Command is invalid", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, commandResponseFromRecord(created))
}

func (s *Server) listProcesses(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	commands, err := s.store.ListCommands(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "process_list_failed", "Processes could not be listed", nil)
		return
	}
	response := make([]commandResponse, 0, len(commands))
	for _, command := range commands {
		response = append(response, commandResponseFromRecord(command))
	}
	page, nextCursor, ok := paginateItems(w, response, pagination, func(command commandResponse) string {
		return command.ID
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processes": page, "nextCursor": nextCursor})
}

func (s *Server) getProcess(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	command, err := s.store.GetCommand(r.Context(), ws.ID, r.PathValue("processId"))
	if err != nil {
		writeProcessError(w, err)
		return
	}
	output, err := s.store.ListCommandOutput(r.Context(), command.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "process_output_failed", "Process output could not be loaded", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"command": commandResponseFromRecord(command),
		"output":  outputResponsesFromRecords(output),
	})
}

func (s *Server) stopProcess(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	command, err := s.store.GetCommand(r.Context(), ws.ID, r.PathValue("processId"))
	if err != nil {
		writeProcessError(w, err)
		return
	}
	if command.Status == "running" || command.Status == "queued" {
		if stopped := s.runner.Stop(command.ID); !stopped {
			now := time.Now().UTC()
			command, err = s.store.FinishCommand(r.Context(), ws.ID, command.ID, "stopped", nil, now)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "process_stop_failed", "Process could not be stopped", nil)
				return
			}
			s.publishProcessExited(command)
		}
	}
	command, err = s.store.GetCommand(r.Context(), ws.ID, command.ID)
	if err != nil {
		writeProcessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commandResponseFromRecord(command))
}

func (s *Server) listPorts(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	s.refreshPortStates(r.Context(), ws.ID)
	records, err := s.store.ListPorts(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "port_list_failed", "Ports could not be listed", nil)
		return
	}
	response := make([]portResponse, 0, len(records))
	for _, record := range records {
		response = append(response, s.portResponseFromRecord(r, record))
	}
	page, nextCursor, ok := paginateItems(w, response, pagination, func(port portResponse) string {
		return strconv.Itoa(port.Port)
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ports": page, "nextCursor": nextCursor})
}

func (s *Server) exposePort(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	port, ok := portFromRequest(w, r)
	if !ok {
		return
	}
	if _, err := s.store.GetPort(r.Context(), ws.ID, port); err != nil {
		writePortError(w, err)
		return
	}
	if !ports.Reachable(r.Context(), port) {
		s.markPortClosed(r.Context(), ws.ID, port)
		writeError(w, http.StatusBadGateway, "port_unreachable", "Port is not accepting local connections", nil)
		return
	}
	exposedPath := fmt.Sprintf("/workspaces/%s/ports/%d/proxy/", ws.ID, port)
	record, err := s.store.ExposePort(r.Context(), ws.ID, port, exposedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "port_expose_failed", "Port could not be exposed", nil)
		return
	}
	response := s.portResponseFromRecord(r, record)
	s.events.Publish(events.Event{WorkspaceID: ws.ID, Type: "port.exposed", Payload: response})
	writeJSON(w, http.StatusOK, map[string]any{"port": response})
}

func (s *Server) proxyPort(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	port, ok := portFromRequest(w, r)
	if !ok {
		return
	}
	record, err := s.store.GetPort(r.Context(), ws.ID, port)
	if err != nil {
		writePortError(w, err)
		return
	}
	if record.Status != "exposed" {
		writeError(w, http.StatusConflict, "port_not_exposed", "Port is not exposed", nil)
		return
	}
	host, reachable := ports.ReachableHost(r.Context(), port)
	if !reachable {
		s.markPortClosed(r.Context(), ws.ID, port)
		writeError(w, http.StatusBadGateway, "port_unreachable", "Port is not accepting local connections", nil)
		return
	}
	prefix := fmt.Sprintf("/workspaces/%s/ports/%d/proxy", ws.ID, port)
	http.StripPrefix(prefix, ports.NewProxyForHost(host, port)).ServeHTTP(w, r)
}

func (s *Server) commandHooks(workspaceID, commandID string) runner.Hooks {
	scanCtx, stopScanning := context.WithCancel(context.Background())
	return runner.Hooks{
		OnStarted: func(pid int) {
			startedAt := time.Now().UTC()
			command, err := s.store.MarkCommandRunning(context.Background(), workspaceID, commandID, startedAt)
			if err == nil {
				s.publishProcessStarted(command)
			}
			go s.pollListeningPorts(scanCtx, workspaceID, commandID, pid)
		},
		OnOutput: func(stream, chunk string) {
			output, err := s.store.AppendCommandOutput(context.Background(), database.CommandOutputRecord{
				CommandID: commandID,
				Stream:    stream,
				Chunk:     chunk,
			}, 1024*1024)
			if err == nil {
				s.events.Publish(events.Event{
					WorkspaceID: workspaceID,
					Type:        "command.output",
					Payload:     outputResponseFromRecord(output),
				})
			}
		},
		OnFinished: func(result runner.FinishResult) {
			stopScanning()
			finishedAt := time.Now().UTC()
			command, err := s.store.FinishCommand(context.Background(), workspaceID, commandID, result.Status, result.ExitCode, finishedAt)
			if err == nil {
				s.publishProcessExited(command)
			}
		},
	}
}

func (s *Server) stopCommandForShutdown(ctx context.Context, command database.CommandRecord) error {
	if command.Status != "queued" && command.Status != "running" {
		return nil
	}
	if stopped := s.runner.StopAndWait(ctx, command.ID); stopped {
		updated, err := s.store.GetCommand(ctx, command.WorkspaceID, command.ID)
		if err != nil {
			return err
		}
		if updated.Status != "queued" && updated.Status != "running" {
			return nil
		}
		command = updated
	}
	finishedAt := time.Now().UTC()
	command, err := s.store.FinishCommand(ctx, command.WorkspaceID, command.ID, "stopped", nil, finishedAt)
	if err != nil {
		return err
	}
	s.publishProcessExited(command)
	return nil
}

func (s *Server) pollListeningPorts(ctx context.Context, workspaceID, commandID string, pid int) {
	s.detectListeningPorts(ctx, workspaceID, commandID, pid)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.detectListeningPorts(ctx, workspaceID, commandID, pid)
		}
	}
}

func (s *Server) detectListeningPorts(ctx context.Context, workspaceID, commandID string, pid int) {
	detectedPorts, err := s.ports.ListeningPorts(ctx, pid)
	if err != nil {
		return
	}
	for _, detectedPort := range detectedPorts {
		processID := commandID
		record, created, err := s.store.UpsertDetectedPort(ctx, database.PortRecord{
			WorkspaceID: workspaceID,
			ProcessID:   &processID,
			Port:        detectedPort,
			Status:      "detected",
		})
		if err != nil {
			continue
		}
		eventType := "port.opened"
		if !created && record.Status == "exposed" {
			eventType = "port.exposed"
		}
		s.events.Publish(events.Event{
			WorkspaceID: workspaceID,
			Type:        eventType,
			Payload:     s.portResponseFromRecord(nil, record),
		})
	}
}

func (s *Server) markPortClosed(ctx context.Context, workspaceID string, port int) {
	record, err := s.store.MarkPortClosed(ctx, workspaceID, port, time.Now().UTC())
	if err != nil {
		return
	}
	s.events.Publish(events.Event{
		WorkspaceID: workspaceID,
		Type:        "port.closed",
		Payload:     s.portResponseFromRecord(nil, record),
	})
}

func (s *Server) refreshPortStates(ctx context.Context, workspaceID string) {
	records, err := s.store.ListPorts(ctx, workspaceID)
	if err != nil {
		return
	}
	for _, record := range records {
		if ports.Reachable(ctx, record.Port) {
			if record.ExposedPath != nil {
				updated, err := s.store.ExposePort(ctx, workspaceID, record.Port, *record.ExposedPath)
				if err == nil && record.Status != updated.Status {
					s.events.Publish(events.Event{WorkspaceID: workspaceID, Type: "port.exposed", Payload: s.portResponseFromRecord(nil, updated)})
				}
				continue
			}
			updated, _, err := s.store.UpsertDetectedPort(ctx, database.PortRecord{
				WorkspaceID: workspaceID,
				ProcessID:   record.ProcessID,
				Port:        record.Port,
				Status:      "detected",
			})
			if err == nil && record.Status != updated.Status {
				s.events.Publish(events.Event{WorkspaceID: workspaceID, Type: "port.opened", Payload: s.portResponseFromRecord(nil, updated)})
			}
			continue
		}
		if record.Status == "exposed" || record.Status == "detected" {
			s.markPortClosed(ctx, workspaceID, record.Port)
		}
	}
}

func (s *Server) publishProcessStarted(command database.CommandRecord) {
	s.events.Publish(events.Event{
		WorkspaceID: command.WorkspaceID,
		Type:        "process.started",
		Payload:     commandResponseFromRecord(command),
	})
}

func (s *Server) publishProcessExited(command database.CommandRecord) {
	s.events.Publish(events.Event{
		WorkspaceID: command.WorkspaceID,
		Type:        "process.exited",
		Payload:     commandResponseFromRecord(command),
	})
}

func (s *Server) publishGitChanged(ctx context.Context, ws workspace.Workspace) {
	status, err := s.git.Status(ctx, ws.RootPath, gitrepo.StatusOptions{})
	if err != nil {
		return
	}
	s.events.Publish(events.Event{
		WorkspaceID: ws.ID,
		Type:        "git.changed",
		Payload:     status,
	})
}

func (s *Server) publishWorkspaceState(workspaceID, eventType string, ws workspace.Workspace) {
	s.events.Publish(events.Event{
		WorkspaceID: workspaceID,
		Type:        eventType,
		Payload:     ws,
	})
}
