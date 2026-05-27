package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

func TestSettingsReturnsSafeDefaultsAndStatus(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	fixture.server.SetSettingsHome(home)
	fixture.server.SetLightModel("gpt-light")
	fixture.server.SetRuntimeConfigStatus(true, "https://api.openai.com/v1", 2, "json", true)

	request := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	recorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body settingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Preferences.Theme != "system" || body.Preferences.TerminalFontFamily == "" {
		t.Fatalf("unexpected preferences: %+v", body.Preferences)
	}
	if !body.ServerStatus.ProviderConfigured || body.ServerStatus.OpenAIBaseURLHost != "api.openai.com" || body.ServerStatus.LightModel != "gpt-light" {
		t.Fatalf("unexpected server status: %+v", body.ServerStatus)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("OPENAI_API_KEY")) {
		t.Fatalf("settings response exposed secret-like content: %s", recorder.Body.String())
	}
}

func TestPatchSettingsPreferencesPersistsNonSecretValues(t *testing.T) {
	home := t.TempDir()
	fixture := newServerFixture(t, t.TempDir(), fakeAgentProvider{})
	fixture.server.SetSettingsHome(home)

	body := bytes.NewBufferString(`{"theme":"dark","defaultModel":"gpt-5.4-mini","defaultReasoningEffort":"high","terminalFontFamily":"Mono"}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/settings/preferences", body)
	recorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	cfg, err := config.LoadUserConfig(home)
	if err != nil {
		t.Fatalf("LoadUserConfig returned error: %v", err)
	}
	if cfg.Preferences.Theme != "dark" || cfg.Preferences.DefaultModel != "gpt-5.4-mini" || cfg.Preferences.TerminalFontFamily != "Mono" {
		t.Fatalf("preferences were not persisted: %+v", cfg.Preferences)
	}
}

func TestSettingsRejectsUnknownPreferenceFields(t *testing.T) {
	fixture := newServerFixture(t, t.TempDir(), fakeAgentProvider{})
	fixture.server.SetSettingsHome(t.TempDir())

	request := httptest.NewRequest(http.MethodPatch, "/api/settings/preferences", bytes.NewBufferString(`{"openAIAPIKey":"secret"}`))
	recorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSettingsFontInstallServeAndDeleteGuardsInUse(t *testing.T) {
	home := t.TempDir()
	fixture := newServerFixture(t, t.TempDir(), fakeAgentProvider{})
	fixture.server.SetSettingsHome(home)

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	if err := writer.WriteField("family", "Patch Mono"); err != nil {
		t.Fatalf("WriteField returned error: %v", err)
	}
	part, err := writer.CreateFormFile("file", "patch-mono.woff2")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte("fake-font")); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/settings/fonts", &form)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		Font settingsFontResponse `json:"font"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if created.Font.ID == "" || created.Font.URL == "" {
		t.Fatalf("unexpected font response: %+v", created.Font)
	}
	if _, err := config.LoadUserConfig(home); err != nil {
		t.Fatalf("LoadUserConfig returned error: %v", err)
	}
	if _, err := filepath.Abs(filepath.Join(home, ".patchpilot", "fonts", created.Font.ID, created.Font.Filename)); err != nil {
		t.Fatalf("font path was not valid: %v", err)
	}

	patch := bytes.NewBufferString(`{"terminalFontFamily":"Patch Mono"}`)
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/preferences", patch)
	patchRecorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected preference patch 200, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/settings/fonts/"+created.Font.ID, nil)
	deleteRecorder := httptest.NewRecorder()
	fixture.server.Routes().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 for in-use font, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}
