package logging

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewSupportsJSONFormat(t *testing.T) {
	logger, err := New("json")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestNewSupportsConsoleFormat(t *testing.T) {
	logger, err := New("console")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestConsoleEncodersUseANSIColors(t *testing.T) {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncodeTime = colorTimeEncoder
	cfg.EncodeCaller = colorCallerEncoder

	encoder := zapcore.NewConsoleEncoder(cfg)
	buffer, err := encoder.EncodeEntry(zapcore.Entry{
		Caller:  zapcore.EntryCaller{Defined: true, File: "cmd/patchpilot/main.go", Line: 1},
		Level:   zapcore.InfoLevel,
		Message: "started",
		Time:    time.Date(2026, 5, 21, 10, 30, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("EncodeEntry returned error: %v", err)
	}
	line := buffer.String()
	if count := strings.Count(line, "\x1b["); count < 3 {
		t.Fatalf("expected ANSI escapes for time, level, and caller, got %q", line)
	}
}
