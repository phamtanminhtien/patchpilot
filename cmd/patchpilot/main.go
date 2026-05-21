package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/api"
	"github.com/phamtanminhtien/patchpilot/internal/config"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/logging"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
	"go.uber.org/zap"
)

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
	workspaces, err := workspace.NewManager(cfg.AllowedRoots, store, gitClient)
	if err != nil {
		return err
	}

	server := api.NewServer(workspaces, filestore.NewService(), gitClient, runner.NewRunner(), store)
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
	if err := httpServer.Shutdown(ctx); err != nil {
		return err
	}

	if err := <-serverErrors; err != nil {
		return err
	}
	logger.Info("patchpilot shutdown complete")
	return nil
}
