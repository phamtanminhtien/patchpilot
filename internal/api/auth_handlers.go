package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/auth"
)

type loginRequest struct {
	Token string `json:"token"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	rawToken, session, err := s.auth.Login(r.Context(), req.Token, time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "invalid_auth_token", "Invalid admin token", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "login_failed", "Login failed", nil)
		return
	}
	auth.SetSessionCookie(w, rawToken, session.ExpiresAt, auth.SecureCookie(r))
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	session, err := s.auth.ValidateRequest(r.Context(), r, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	if err := s.auth.Logout(r.Context(), r); err != nil {
		writeError(w, http.StatusInternalServerError, "logout_failed", "Logout failed", nil)
		return
	}
	auth.ClearSessionCookie(w, auth.SecureCookie(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
