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
	"kv/static"
)

type Config struct {
	Addr    string
	Manager *k8s.Manager
	Entries []config.Entry
}

func New(ctx context.Context, cfg Config) *http.Server {
	h := handlers.New(cfg.Manager, cfg.Entries)

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
	r.Get("/api/pod/info", h.PodInfo)
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
	return srv
}

func staticHandler() http.Handler {
	// Allow override for development
	if dir := os.Getenv("KV_STATIC_DIR"); dir != "" {
		return http.StripPrefix("/static/", http.FileServer(http.Dir(dir)))
	}

	// Use embedded filesystem for production
	return http.StripPrefix("/static/", http.FileServer(http.FS(static.FS)))
}
