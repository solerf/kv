package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"kv/templates"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) PodDetail(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	templates.PodDetail(kubeCtx, ns, name).Render(r.Context(), w)
}

func (h *Handler) PodMetrics(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	metrics, err := client.GetPodMetrics(r.Context(), ns, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	stream, err := client.StreamLogs(r.Context(), ns, name, follow)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	if err := client.DeletePod(r.Context(), ns, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"deleted"}`))
}
