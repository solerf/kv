package handlers

import (
	"net/http"
	"strings"

	"sigs.k8s.io/yaml"

	"kv/config"
	"kv/k8s"
	"kv/templates"
)

// deterministicDNSName returns the value of the annotation whose key ends with
// "deterministic-dns-name" (regardless of its domain prefix), or "" if absent.
func deterministicDNSName(annotations map[string]string) string {
	for key, value := range annotations {
		if strings.HasSuffix(key, "deterministic-dns-name") {
			return value
		}
	}
	return ""
}

type Handler struct {
	clients map[string]*k8s.Client
	entries []config.Entry
}

func New(clients map[string]*k8s.Client, entries []config.Entry) *Handler {
	return &Handler{
		clients: clients,
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

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	type resourceTable struct {
		Kind  string              `json:"kind"`
		Items []map[string]string `json:"items"`
	}

	kindsToFetch := []string{"services", "deployments", "ingresses", "statefulsets", "daemonsets", "jobs", "cronjobs"}
	var tables []resourceTable

	for _, kind := range kindsToFetch {
		items, err := client.List(r.Context(), kind, ns)
		if err != nil {
			continue
		}
		rows := make([]map[string]string, 0, len(items))
		for _, item := range items {
			row := map[string]string{
				"name":      item.GetName(),
				"namespace": item.GetNamespace(),
				"age":       item.GetCreationTimestamp().Time.Format("2006-01-02 15:04"),
				"status":    k8s.ExtractStatus(item),
			}

			// For ingresses, extract deterministicDNS tag if present
			if kind == "ingresses" {
				if dnsValue := deterministicDNSName(item.GetAnnotations()); dnsValue != "" {
					row["deterministicDNS"] = dnsValue
				}
			}

			rows = append(rows, row)
		}
		if len(rows) > 0 {
			tables = append(tables, resourceTable{Kind: kind, Items: rows})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, tables)
}

func (h *Handler) Pods(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	items, err := client.List(r.Context(), "pods", ns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type podInfo struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Age    string `json:"age"`
		IP     string `json:"ip"`
		Node   string `json:"node"`
		Image  string `json:"image"`
	}

	pods := make([]podInfo, 0, len(items))
	for _, item := range items {
		ip, _, _ := k8s.NestedString(item.Object, "status", "podIP")
		node, _, _ := k8s.NestedString(item.Object, "spec", "nodeName")
		image := k8s.ExtractMainContainerImage(item)
		pods = append(pods, podInfo{
			Name:   item.GetName(),
			Status: k8s.ExtractStatus(item),
			Age:    item.GetCreationTimestamp().Time.Format("2006-01-02 15:04"),
			IP:     ip,
			Node:   node,
			Image:  image,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, pods)
}

func (h *Handler) Describe(w http.ResponseWriter, r *http.Request) {
	kubeCtx := r.URL.Query().Get("context")
	ns := r.URL.Query().Get("namespace")
	kind := r.URL.Query().Get("kind")
	name := r.URL.Query().Get("name")

	client, ok := h.clients[kubeCtx]
	if !ok {
		http.Error(w, "unknown context", http.StatusBadRequest)
		return
	}

	obj, err := client.Get(r.Context(), kind, ns, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out, err := yaml.Marshal(obj.Object)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(out)
}
