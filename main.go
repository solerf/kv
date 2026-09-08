package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"

	"kv/config"
	"kv/k8s"
	"kv/server"
)

type CLI struct {
	Kubeconfig string `help:"Path to kubeconfig file." type:"path" short:"k" env:"KUBECONFIG"`
	Config     string `help:"Path to kv.context file." type:"path" short:"c" default:"~/kv.context"`
	Addr       string `help:"Address to listen on." short:"a" default:":8989"`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("kv"),
		kong.Description("View Kubernetes resources in UI."),
	)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(cli.Config)
	if err != nil {
		logger.Error("loading config", "err", err)
		os.Exit(1)
	}

	contexts := make([]string, 0, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		contexts = append(contexts, entry.Context)
	}

	mgr, err := k8s.NewManager(cli.Kubeconfig, contexts, logger)
	if err != nil {
		logger.Error("creating manager", "err", err)
		os.Exit(1)
	}

	srv := server.New(ctx, server.Config{
		Addr:    cli.Addr,
		Manager: mgr,
		Entries: cfg.Entries,
	})

	logger.Info("listening", "addr", "http://localhost"+cli.Addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Stop serving first (drains in-flight requests), then release the
		// manager's cluster connections.
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
		}
		mgr.Close()
	}()

	if err = srv.ListenAndServe(); err != nil && ctx.Err() == nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
