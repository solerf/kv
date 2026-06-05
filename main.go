package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"

	"kv/config"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(cli.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	srv, err := server.New(ctx, server.Config{
		Addr:       cli.Addr,
		Kubeconfig: cli.Kubeconfig,
		Entries:    cfg.Entries,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("kv listening on http://localhost%s\n", cli.Addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err = srv.ListenAndServe(); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
