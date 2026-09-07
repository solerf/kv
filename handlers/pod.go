package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"kv/k8s"
	"kv/templates"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	json.NewEncoder(w).Encode(v)
}

// writeErr maps a Manager error to an HTTP status: an unknown context is a
// client error (400), anything else is treated as a server error (500).
func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, k8s.ErrUnknownContext) {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (h *Handler) PodDetail(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	templates.PodDetail(kubeCtx, ns, name).Render(r.Context(), w)
}

func (h *Handler) PodInfo(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	pod, err := h.mgr.Get[k8s.Pod](r.Context(), kubeCtx, ns, name)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, pod)
}

func (h *Handler) PodMetrics(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	metrics, err := h.mgr.PodMetrics(r.Context(), kubeCtx, ns, name)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, metrics)
}

func (h *Handler) PodLogs(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	follow := r.URL.Query().Get("follow") == "true"

	stream, err := h.mgr.StreamLogs(r.Context(), kubeCtx, ns, name, follow)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, canFlush := w.(http.Flusher)

	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				w.Write([]byte("\n[error reading logs: " + err.Error() + "]\n"))
			}
			break
		}
	}
}

func (h *Handler) PodDelete(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	if err := h.mgr.DeletePod(r.Context(), kubeCtx, ns, name); err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"deleted"}`))
}
