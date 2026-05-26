package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/auth"
)

func TestHealthCheckReturnsOK(t *testing.T) {
	server := newTestServer(t, t.TempDir())

	response := request(server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]string
	mustDecode(t, response, &body)
	if body["status"] != "ok" {
		t.Fatalf("unexpected health body: %+v", body)
	}
}

func TestHealthCheckReturnsUnavailableWhenDatabaseFails(t *testing.T) {
	server := newTestServerWithHealth(t, t.TempDir(), fakeHealthChecker{err: errors.New("down")})

	response := request(server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]map[string]any
	mustDecode(t, response, &body)
	if body["error"]["code"] != "database_unavailable" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

func TestAuthHandlersProtectWorkspaceRoutes(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newAuthenticatedTestServer(t, root, "secret")

	unauthorized := request(server, http.MethodGet, "/api/workspaces", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", unauthorized.Code, unauthorized.Body.String())
	}

	login := request(server, http.MethodPost, "/api/auth/login", `{"token":"secret"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", login.Code, login.Body.String())
	}
	var loginBody struct {
		Session auth.Session `json:"session"`
	}
	mustDecode(t, login, &loginBody)
	if !strings.HasPrefix(loginBody.Session.ID, "auth_") || loginBody.Session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected login body: %+v", loginBody)
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.CookieName {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}

	authorized := requestWithCookies(server, http.MethodGet, "/api/workspaces", "", cookies)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", authorized.Code, authorized.Body.String())
	}

	session := requestWithCookies(server, http.MethodGet, "/api/auth/session", "", cookies)
	if session.Code != http.StatusOK {
		t.Fatalf("expected session 200, got %d: %s", session.Code, session.Body.String())
	}
	var sessionBody struct {
		Session auth.Session `json:"session"`
	}
	mustDecode(t, session, &sessionBody)
	if sessionBody.Session.ID != loginBody.Session.ID {
		t.Fatalf("expected same session %q, got %+v", loginBody.Session.ID, sessionBody)
	}

	logout := requestWithCookies(server, http.MethodPost, "/api/auth/logout", "", cookies)
	if logout.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logout.Code, logout.Body.String())
	}
	var logoutBody map[string]string
	mustDecode(t, logout, &logoutBody)
	if logoutBody["status"] != "ok" {
		t.Fatalf("unexpected logout body: %+v", logoutBody)
	}
	if cleared := logout.Result().Cookies(); len(cleared) == 0 || cleared[0].Name != auth.CookieName || cleared[0].MaxAge >= 0 {
		t.Fatalf("expected cleared session cookie, got %+v", cleared)
	}

	afterLogout := requestWithCookies(server, http.MethodGet, "/api/auth/session", "", cookies)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("expected session 401 after logout, got %d: %s", afterLogout.Code, afterLogout.Body.String())
	}
}
