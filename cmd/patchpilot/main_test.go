package main

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

type fakeCleaner struct {
	calls  *[]string
	err    error
	reason string
}

func (f *fakeCleaner) Shutdown(_ context.Context, reason string) error {
	*f.calls = append(*f.calls, "cleaner")
	f.reason = reason
	return f.err
}

type fakeHTTPServer struct {
	calls *[]string
	err   error
}

func (f *fakeHTTPServer) Shutdown(context.Context) error {
	*f.calls = append(*f.calls, "http")
	return f.err
}

func TestShutdownServicesRunsCleanupBeforeHTTPShutdown(t *testing.T) {
	var calls []string
	cleaner := &fakeCleaner{calls: &calls}
	httpServer := &fakeHTTPServer{calls: &calls}

	if err := shutdownServices(context.Background(), cleaner, httpServer, zap.NewNop()); err != nil {
		t.Fatalf("shutdownServices returned error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "cleaner" || calls[1] != "http" {
		t.Fatalf("expected cleanup before http shutdown, got %+v", calls)
	}
	if cleaner.reason != "backend shutdown" {
		t.Fatalf("expected fixed shutdown reason, got %q", cleaner.reason)
	}
}

func TestShutdownServicesStillStopsHTTPWhenCleanupFails(t *testing.T) {
	var calls []string
	cleaner := &fakeCleaner{calls: &calls, err: errors.New("cleanup failed")}
	httpServer := &fakeHTTPServer{calls: &calls}

	if err := shutdownServices(context.Background(), cleaner, httpServer, zap.NewNop()); err != nil {
		t.Fatalf("shutdownServices returned error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "cleaner" || calls[1] != "http" {
		t.Fatalf("expected best-effort cleanup then http shutdown, got %+v", calls)
	}
}
