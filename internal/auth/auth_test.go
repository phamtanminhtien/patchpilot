package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

func TestServiceLoginValidateAndLogout(t *testing.T) {
	store, err := database.Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	service, err := NewService("secret", store)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	rawToken, session, err := service.Login(context.Background(), "secret", now)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if rawToken == "" || session.ID == "" {
		t.Fatalf("expected token and session, got %q %+v", rawToken, session)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	request.AddCookie(&http.Cookie{Name: CookieName, Value: rawToken})
	if _, err := service.ValidateRequest(context.Background(), request, now.Add(time.Minute)); err != nil {
		t.Fatalf("ValidateRequest returned error: %v", err)
	}
	if err := service.Logout(context.Background(), request); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if _, err := service.ValidateRequest(context.Background(), request, now.Add(2*time.Minute)); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession after logout, got %v", err)
	}
}
