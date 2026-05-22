package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

const (
	CookieName = "patchpilot_session"
	ttl        = 30 * 24 * time.Hour
)

var (
	ErrInvalidToken = errors.New("invalid admin token")
	ErrNoSession    = errors.New("session is missing")
)

type Service struct {
	adminToken string
	store      *database.Store
}

type Session struct {
	ID         string    `json:"id"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

func NewService(adminToken string, store *database.Store) (*Service, error) {
	adminToken = strings.TrimSpace(adminToken)
	if adminToken == "" {
		return nil, errors.New("PATCHPILOT_ADMIN_TOKEN is required")
	}
	if store == nil {
		return nil, errors.New("auth store is required")
	}
	return &Service{adminToken: adminToken, store: store}, nil
}

func (s *Service) Login(ctx context.Context, token string, now time.Time) (string, Session, error) {
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		return "", Session{}, ErrInvalidToken
	}
	raw, err := randomToken()
	if err != nil {
		return "", Session{}, err
	}
	expiresAt := now.UTC().Add(ttl)
	record, err := s.store.CreateAuthSession(ctx, database.AuthSessionRecord{
		SessionHash: hashToken(raw),
		CreatedAt:   now.UTC(),
		LastSeenAt:  now.UTC(),
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return "", Session{}, err
	}
	return raw, sessionFromRecord(record), nil
}

func (s *Service) ValidateRequest(ctx context.Context, r *http.Request, now time.Time) (Session, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return Session{}, ErrNoSession
	}
	record, err := s.store.GetAuthSessionByHash(ctx, hashToken(cookie.Value), now)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Session{}, ErrNoSession
		}
		return Session{}, err
	}
	return sessionFromRecord(record), nil
}

func (s *Service) Logout(ctx context.Context, r *http.Request) error {
	cookie, err := r.Cookie(CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil
	}
	return s.store.DeleteAuthSessionByHash(ctx, hashToken(cookie.Value))
}

func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func SecureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func sessionFromRecord(record database.AuthSessionRecord) Session {
	return Session{
		ID:         record.ID,
		ExpiresAt:  record.ExpiresAt,
		LastSeenAt: record.LastSeenAt,
	}
}
