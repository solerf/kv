package handlers

import (
	"net/http"
	"strings"

	"kv/config"
	"kv/k8s"
	"kv/templates"
)

type Handler struct {
	mgr     *k8s.Manager
	entries []config.Entry
}

func New(mgr *k8s.Manager, entries []config.Entry) *Handler {
	return &Handler{
		mgr:     mgr,
		entries: entries,
	}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	contexts := make([]string, 0)
	seen := make(map[string]bool)
	for _, e := range h.entries {
		if !seen[e.Context] {
			contexts = append(contexts, e.Context)
			seen[e.Context] = true
		}
	}
	templates.Dashboard(contexts).Render(r.Context(), w)
}

func (h *Handler) Namespaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.URL.Query().Get("context")
	namespaces := make([]string, 0)
	for _, e := range h.entries {
		if e.Context == ctx {
			namespaces = append(namespaces, e.Namespace)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, namespaces)
}

func (h *Handler) Resources(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")

	tables, err := h.mgr.ListResources(r.Context(), kubeCtx, ns)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, tables)
}

func (h *Handler) Pods(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")

	pods, err := h.mgr.List[k8s.Pod](r.Context(), kubeCtx, ns)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, pods)
}

func (h *Handler) Describe(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	kind := r.URL.Query().Get("kind")
	name := r.URL.Query().Get("name")

	// kind to lower, otherwise it will not be fetched as UI sends it capitalized
	out, err := h.mgr.Describe(r.Context(), kubeCtx, strings.ToLower(kind), ns, name)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(out))
}
