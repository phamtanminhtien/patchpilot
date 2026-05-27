package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/api"
	"github.com/phamtanminhtien/patchpilot/internal/auth"
	"github.com/phamtanminhtien/patchpilot/internal/config"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/logging"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
	"go.uber.org/zap"
)

type gracefulShutdowner interface {
	Shutdown(context.Context, string) error
}

type httpShutdowner interface {
	Shutdown(context.Context) error
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger, err := logging.New(cfg.LogFormat)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	if err := run(cfg, logger); err != nil {
		logger.Error("patchpilot stopped", zap.Error(err))
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *zap.Logger) error {
	store, err := database.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("close database", zap.Error(err))
		}
	}()

	gitClient := gitrepo.NewClient()
	fileService := filestore.NewService()
	run := runner.NewRunner()
	hub := events.NewHub()
	workspaces, err := workspace.NewManager(cfg.AllowedRoots, store, gitClient)
	if err != nil {
		return err
	}
	authService, err := auth.NewService(cfg.AdminToken, store)
	if err != nil {
		return err
	}
	agentManager := agent.NewManager(store, fileService, gitClient, run, hub, agent.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL))

	server := api.NewServerWithAuth(workspaces, fileService, gitClient, run, store, hub, agentManager, authService, store)
	server.SetBackendAddr(cfg.Addr)
	server.SetLightModel(cfg.LightModel)
	server.SetSettingsHome(cfg.HomeDir)
	server.SetRuntimeConfigStatus(cfg.OpenAIAPIKey != "", cfg.OpenAIBaseURL, len(cfg.AllowedRoots), cfg.LogFormat, cfg.StaticDir != "")
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.RoutesWithStatic(cfg.StaticDir),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("patchpilot listening", zap.String("addr", httpServer.Addr), zap.String("db_path", cfg.DBPath))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-serverErrors:
		return err
	case signal := <-signals:
		logger.Info("shutdown signal received", zap.String("signal", signal.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdownServices(ctx, server, httpServer, logger); err != nil {
		return err
	}

	if err := <-serverErrors; err != nil {
		return err
	}
	logger.Info("patchpilot shutdown complete")
	return nil
}

func shutdownServices(ctx context.Context, cleaner gracefulShutdowner, server httpShutdowner, logger *zap.Logger) error {
	if cleaner != nil {
		if err := cleaner.Shutdown(ctx, "backend shutdown"); err != nil {
			logger.Error("shutdown cleanup failed", zap.Error(err))
		}
	}
	return server.Shutdown(ctx)
}
