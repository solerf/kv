package server

import (
	"context"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"kv/config"
	"kv/handlers"
	"kv/k8s"
)

type Config struct {
	Addr       string
	Kubeconfig string
	Entries    []config.Entry
}

func New(ctx context.Context, cfg Config) (*http.Server, error) {
	clients := make(map[string]*k8s.Client)
	for _, entry := range cfg.Entries {
		if _, exists := clients[entry.Context]; exists {
			continue
		}
		client, err := k8s.NewClient(cfg.Kubeconfig, entry.Context)
		if err != nil {
			return nil, err
		}
		clients[entry.Context] = client
	}

	h := handlers.New(clients, cfg.Entries)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", staticHandler())
	r.Get("/", h.Dashboard)
	r.Get("/api/namespaces", h.Namespaces)
	r.Get("/api/resources", h.Resources)
	r.Get("/api/pods", h.Pods)
	r.Get("/api/describe", h.Describe)
	r.Get("/pod", h.PodDetail)
	r.Get("/api/pod/metrics", h.PodMetrics)
	r.Get("/api/pod/logs", h.PodLogs)
	r.Delete("/api/pod", h.PodDelete)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	return srv, nil
}

func staticHandler() http.Handler {
	dir := "static"
	if d := os.Getenv("KV_STATIC_DIR"); d != "" {
		dir = d
	}
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(dir)))
	return fs
}
