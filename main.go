package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"kv/config"
	"kv/server"
)

type CLI struct {
	Kubeconfig string `help:"Path to kubeconfig file." type:"path" short:"k" env:"KUBECONFIG"`
	Config     string `help:"Path to kv.context file." type:"path" short:"c" default:"~/kv.context"`
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
		Addr:       ":8888",
		Kubeconfig: cli.Kubeconfig,
		Entries:    cfg.Entries,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("kv listening on http://0.0.0.0:8888\n")

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	if err = srv.ListenAndServe(); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
