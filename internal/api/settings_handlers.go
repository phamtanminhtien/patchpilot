package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

const (
	defaultAppFont       = "Inter, ui-sans-serif, system-ui, sans-serif"
	defaultMonoFont      = "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
	maxFontUploadBytes   = 2 << 20
	settingsDefaultModel = "gpt-5.5"
)

var (
	fontFamilyPattern = regexp.MustCompile(`^[A-Za-z0-9 ._,\-]+$`)
	fontExtensions    = map[string]struct{}{".woff2": {}, ".woff": {}, ".ttf": {}, ".otf": {}}
	settingsModels    = map[string]struct{}{"gpt-5.5": {}, "gpt-5.4": {}, "gpt-5.4-mini": {}}
	settingsEfforts   = map[string]struct{}{"low": {}, "medium": {}, "high": {}, "xhigh": {}}
	settingsThemes    = map[string]struct{}{"system": {}, "light": {}, "dark": {}}
)

type settingsResponse struct {
	Preferences  settingsPreferencesResponse `json:"preferences"`
	Fonts        []settingsFontResponse      `json:"fonts"`
	ServerStatus settingsServerStatus        `json:"serverStatus"`
}

type settingsPreferencesResponse struct {
	Theme                  string `json:"theme"`
	AppFontFamily          string `json:"appFontFamily"`
	CodeFontFamily         string `json:"codeFontFamily"`
	TerminalFontFamily     string `json:"terminalFontFamily"`
	DefaultModel           string `json:"defaultModel"`
	DefaultReasoningEffort string `json:"defaultReasoningEffort"`
}

type settingsFontResponse struct {
	ID        string `json:"id"`
	Family    string `json:"family"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mimeType,omitempty"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
}

type settingsServerStatus struct {
	ProviderConfigured  bool   `json:"providerConfigured"`
	OpenAIBaseURLHost   string `json:"openAIBaseUrlHost,omitempty"`
	LightModel          string `json:"lightModel"`
	AllowedRootsCount   int    `json:"allowedRootsCount"`
	LogFormat           string `json:"logFormat,omitempty"`
	StaticDirConfigured bool   `json:"staticDirConfigured"`
}

type patchSettingsPreferencesRequest struct {
	Theme                  *string `json:"theme"`
	AppFontFamily          *string `json:"appFontFamily"`
	CodeFontFamily         *string `json:"codeFontFamily"`
	TerminalFontFamily     *string `json:"terminalFontFamily"`
	DefaultModel           *string `json:"defaultModel"`
	DefaultReasoningEffort *string `json:"defaultReasoningEffort"`
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Settings could not be loaded", nil)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsResponse(cfg))
}

func (s *Server) patchSettingsPreferences(w http.ResponseWriter, r *http.Request) {
	var req patchSettingsPreferencesRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Settings could not be loaded", nil)
		return
	}
	if err := applySettingsPreferences(&cfg, req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_settings_preference", err.Error(), nil)
		return
	}
	if err := config.SaveUserConfig(s.userConfigHome(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Settings could not be saved", nil)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsResponse(cfg))
}

func (s *Server) listSettingsFonts(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Fonts could not be loaded", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fonts": s.fontResponses(cfg.Fonts)})
}

func (s *Server) createSettingsFont(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFontUploadBytes+(512<<10))
	if err := r.ParseMultipartForm(maxFontUploadBytes + (256 << 10)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_font_upload", "Font upload must be multipart form data under 2 MiB", nil)
		return
	}
	family := strings.TrimSpace(r.FormValue("family"))
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "font_file_required", "Font file is required", nil)
		return
	}
	defer file.Close()
	filename := filepath.Base(header.Filename)
	if family == "" {
		family = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	if err := validateFontFamily(family); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_font_family", err.Error(), nil)
		return
	}
	if err := validateFontFilename(filename); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_font_file", err.Error(), nil)
		return
	}
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Fonts could not be loaded", nil)
		return
	}
	for _, font := range cfg.Fonts {
		if strings.EqualFold(font.Family, family) {
			writeError(w, http.StatusConflict, "font_family_exists", "Font family is already installed", nil)
			return
		}
	}
	fontID, err := newFontID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "font_install_failed", "Font could not be installed", nil)
		return
	}
	fontDir := filepath.Join(s.fontRoot(), fontID)
	if err := os.MkdirAll(fontDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "font_install_failed", "Font could not be installed", nil)
		return
	}
	destination := filepath.Join(fontDir, filename)
	written, err := copyLimitedFile(destination, file, maxFontUploadBytes)
	if err != nil {
		_ = os.RemoveAll(fontDir)
		writeError(w, http.StatusBadRequest, "invalid_font_upload", err.Error(), nil)
		return
	}
	font := config.InstalledFont{
		ID:        fontID,
		Family:    family,
		Filename:  filename,
		MimeType:  header.Header.Get("Content-Type"),
		Size:      written,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	cfg.Fonts = append(cfg.Fonts, font)
	if err := config.SaveUserConfig(s.userConfigHome(), cfg); err != nil {
		_ = os.RemoveAll(fontDir)
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Font metadata could not be saved", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"font": s.fontResponse(font)})
}

func (s *Server) getSettingsFontFile(w http.ResponseWriter, r *http.Request) {
	fontID := r.PathValue("fontId")
	font, ok := s.installedFont(fontID)
	if !ok {
		writeError(w, http.StatusNotFound, "font_not_found", "Font was not found", nil)
		return
	}
	w.Header().Set("Content-Type", font.MimeType)
	http.ServeFile(w, r, filepath.Join(s.fontRoot(), font.ID, filepath.Base(font.Filename)))
}

func (s *Server) deleteSettingsFont(w http.ResponseWriter, r *http.Request) {
	fontID := r.PathValue("fontId")
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Fonts could not be loaded", nil)
		return
	}
	index := -1
	var font config.InstalledFont
	for i, candidate := range cfg.Fonts {
		if candidate.ID == fontID {
			index = i
			font = candidate
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, "font_not_found", "Font was not found", nil)
		return
	}
	if preferenceUsesFont(cfg.Preferences, font.Family) {
		writeError(w, http.StatusConflict, "font_in_use", "Font is selected by a font preference", nil)
		return
	}
	cfg.Fonts = append(cfg.Fonts[:index], cfg.Fonts[index+1:]...)
	if err := config.SaveUserConfig(s.userConfigHome(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "settings_config_failed", "Font metadata could not be saved", nil)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.fontRoot(), font.ID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) userConfigHome() string {
	if s.settingsHome != "" {
		return s.settingsHome
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func (s *Server) fontRoot() string {
	return filepath.Join(s.userConfigHome(), ".patchpilot", "fonts")
}

func (s *Server) settingsResponse(cfg config.UserConfig) settingsResponse {
	return settingsResponse{
		Preferences:  preferencesResponse(cfg.Preferences),
		Fonts:        s.fontResponses(cfg.Fonts),
		ServerStatus: s.serverStatus(),
	}
}

func (s *Server) serverStatus() settingsServerStatus {
	return settingsServerStatus{
		ProviderConfigured:  s.providerReady,
		OpenAIBaseURLHost:   safeURLHost(s.openAIBaseURL),
		LightModel:          defaultString(s.lightModel, defaultLightModel),
		AllowedRootsCount:   s.allowedRootCount,
		LogFormat:           s.logFormat,
		StaticDirConfigured: s.staticDirReady,
	}
}

func preferencesResponse(preferences config.SettingsPreferences) settingsPreferencesResponse {
	return settingsPreferencesResponse{
		Theme:                  defaultString(preferences.Theme, "system"),
		AppFontFamily:          defaultString(preferences.AppFontFamily, defaultAppFont),
		CodeFontFamily:         defaultString(preferences.CodeFontFamily, defaultMonoFont),
		TerminalFontFamily:     defaultString(preferences.TerminalFontFamily, defaultMonoFont),
		DefaultModel:           defaultString(preferences.DefaultModel, settingsDefaultModel),
		DefaultReasoningEffort: defaultString(preferences.DefaultReasoningEffort, "medium"),
	}
}

func (s *Server) fontResponses(fonts []config.InstalledFont) []settingsFontResponse {
	responses := make([]settingsFontResponse, 0, len(fonts))
	for _, font := range fonts {
		responses = append(responses, s.fontResponse(font))
	}
	return responses
}

func (s *Server) fontResponse(font config.InstalledFont) settingsFontResponse {
	return settingsFontResponse{
		ID:        font.ID,
		Family:    font.Family,
		Filename:  font.Filename,
		MimeType:  font.MimeType,
		Size:      font.Size,
		CreatedAt: font.CreatedAt,
		URL:       "/api/settings/fonts/" + url.PathEscape(font.ID) + "/file",
	}
}

func (s *Server) installedFont(fontID string) (config.InstalledFont, bool) {
	cfg, err := config.LoadUserConfig(s.userConfigHome())
	if err != nil {
		return config.InstalledFont{}, false
	}
	for _, font := range cfg.Fonts {
		if font.ID == fontID {
			return font, true
		}
	}
	return config.InstalledFont{}, false
}

func applySettingsPreferences(cfg *config.UserConfig, req patchSettingsPreferencesRequest) error {
	if req.Theme != nil {
		value := strings.TrimSpace(*req.Theme)
		if _, ok := settingsThemes[value]; !ok {
			return errMessage("Theme must be system, light, or dark")
		}
		cfg.Preferences.Theme = value
	}
	if req.AppFontFamily != nil {
		value, err := normalizedFontPreference(*req.AppFontFamily)
		if err != nil {
			return err
		}
		cfg.Preferences.AppFontFamily = value
	}
	if req.CodeFontFamily != nil {
		value, err := normalizedFontPreference(*req.CodeFontFamily)
		if err != nil {
			return err
		}
		cfg.Preferences.CodeFontFamily = value
	}
	if req.TerminalFontFamily != nil {
		value, err := normalizedFontPreference(*req.TerminalFontFamily)
		if err != nil {
			return err
		}
		cfg.Preferences.TerminalFontFamily = value
	}
	if req.DefaultModel != nil {
		value := strings.TrimSpace(*req.DefaultModel)
		if _, ok := settingsModels[value]; !ok {
			return errMessage("Default model is not supported")
		}
		cfg.Preferences.DefaultModel = value
	}
	if req.DefaultReasoningEffort != nil {
		value := strings.TrimSpace(*req.DefaultReasoningEffort)
		if _, ok := settingsEfforts[value]; !ok {
			return errMessage("Default reasoning effort is not supported")
		}
		cfg.Preferences.DefaultReasoningEffort = value
	}
	return nil
}

func normalizedFontPreference(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 200 || strings.ContainsAny(value, "{};<>\n\r") {
		return "", errMessage("Font family is not valid")
	}
	return value, nil
}

func validateFontFamily(value string) error {
	if value == "" || len(value) > 80 || !fontFamilyPattern.MatchString(value) {
		return errMessage("Font family must be 1-80 letters, numbers, spaces, dots, hyphens, underscores, or commas")
	}
	return nil
}

func validateFontFilename(filename string) error {
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "..") {
		return errMessage("Font filename is not valid")
	}
	if _, ok := fontExtensions[strings.ToLower(filepath.Ext(filename))]; !ok {
		return errMessage("Font file must be .woff2, .woff, .ttf, or .otf")
	}
	return nil
}

func copyLimitedFile(destination string, source io.Reader, limit int64) (int64, error) {
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
	}
	defer output.Close()
	reader := io.LimitReader(source, limit+1)
	written, err := io.Copy(output, reader)
	if err != nil {
		return written, err
	}
	if written == 0 {
		return written, errMessage("Font file is empty")
	}
	if written > limit {
		return written, errMessage("Font file must be under 2 MiB")
	}
	return written, nil
}

func newFontID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "font_" + hex.EncodeToString(bytes), nil
}

func preferenceUsesFont(preferences config.SettingsPreferences, family string) bool {
	return preferences.AppFontFamily == family || preferences.CodeFontFamily == family || preferences.TerminalFontFamily == family
}

func safeURLHost(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

type errMessage string

func (e errMessage) Error() string { return string(e) }
