package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// Sentinel errors returned by Manager. ErrContextUnavailable wraps the
// underlying client-creation failure.
var (
	ErrUnknownContext     = errors.New("unknown context")
	ErrContextUnavailable = errors.New("context unavailable")
	ErrNoContexts         = errors.New("no contexts configured")
	ErrUnsupportedKind    = errors.New("unsupported resource kind")
)

// clientFactory lazily builds (and memoizes) the client for one context.
type clientFactory = func() (*client, error)

// Manager is the public facade over one lazily-built client per context; a
// context that is never opened costs only a small thunk.
type Manager struct {
	kubeconfig string
	logger     *slog.Logger
	// clients maps each context to its lazy factory. Keys are fixed at
	// construction; the only write swaps a failed factory, which sync.Map handles.
	clients sync.Map // map[string]clientFactory
	// created records the clients actually built, so Close skips unused contexts.
	created sync.Map // map[string]*client
}

// NewManager registers a lazy factory per configured context without connecting
// to any of them. It errors only when no context is configured.
func NewManager(kubeconfig string, contexts []string, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	m := &Manager{kubeconfig: kubeconfig, logger: logger}

	var n int
	for _, kubeContext := range contexts {
		if _, exists := m.clients.Load(kubeContext); exists {
			continue
		}
		m.clients.Store(kubeContext, m.newFactory(kubeContext))
		n++
	}

	if n == 0 {
		return nil, ErrNoContexts
	}

	return m, nil
}

// newFactory returns a memoizing factory: sync.OnceValues builds the client at
// most once and records it so Close can shut it down.
func (m *Manager) newFactory(kubeCtx string) clientFactory {
	return sync.OnceValues(func() (*client, error) {
		c, err := newClient(m.kubeconfig, kubeCtx)
		if err != nil {
			return nil, err
		}
		m.created.Store(kubeCtx, c)
		return c, nil
	})
}

// Close releases every client that was actually created; unused contexts are
// left untouched.
func (m *Manager) Close() {
	m.created.Range(func(_, v any) bool {
		v.(*client).close()
		return true
	})
}

// CheckContext reports whether the context can be served (nil if so), building
// its client once as a side effect so a later call reuses it.
func (m *Manager) CheckContext(kubeCtx string) error {
	_, err := m.client(kubeCtx)
	return err
}

// client returns the client for a context, building it once. On a build failure
// it swaps in a fresh factory so the next call retries instead of caching the error.
func (m *Manager) client(kubeCtx string) (*client, error) {
	v, ok := m.clients.Load(kubeCtx)
	if !ok {
		return nil, ErrUnknownContext
	}

	c, err := v.(clientFactory)()
	if err != nil {
		m.clients.Store(kubeCtx, m.newFactory(kubeCtx))
		return nil, fmt.Errorf("%w: %w", ErrContextUnavailable, err)
	}
	return c, nil
}

func (m *Manager) listGVR(ctx context.Context, kubeCtx string, gvr schema.GroupVersionResource, namespace string) ([]unstructured.Unstructured, error) {
	c, err := m.client(kubeCtx)
	if err != nil {
		return nil, err
	}
	return c.listGVR(ctx, gvr, namespace)
}

func (m *Manager) getGVR(ctx context.Context, kubeCtx string, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	c, err := m.client(kubeCtx)
	if err != nil {
		return nil, err
	}
	return c.getGVR(ctx, gvr, namespace, name)
}

// Describe returns the requested object rendered as YAML.
func (m *Manager) Describe(ctx context.Context, kubeCtx, kind, namespace, name string) (string, error) {
	c, err := m.client(kubeCtx)
	if err != nil {
		return "", err
	}
	gvr, ok := resourceGVR[kind]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedKind, kind)
	}
	obj, err := c.getGVR(ctx, gvr, namespace, name)
	if err != nil {
		return "", fmt.Errorf("getting %s/%s: %w", kind, name, err)
	}
	out, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (m *Manager) StreamLogs(ctx context.Context, kubeCtx, namespace, name string, follow bool) (io.ReadCloser, error) {
	c, err := m.client(kubeCtx)
	if err != nil {
		return nil, err
	}
	return c.streamLogs(ctx, namespace, name, follow)
}

func (m *Manager) DeletePod(ctx context.Context, kubeCtx, namespace, name string) error {
	c, err := m.client(kubeCtx)
	if err != nil {
		return err
	}
	return c.deletePod(ctx, namespace, name)
}

// resourceModel is satisfied by *T: it knows its GVR and fills itself from an
// unstructured object. The *T constraint lets List/Get infer the pointer type.
type resourceModel[T any] interface {
	*T
	gvr() schema.GroupVersionResource
	from(unstructured.Unstructured)
}

// List fetches all objects of model type T in a namespace as typed models.
func (m *Manager) List[T any, PT resourceModel[T]](ctx context.Context, kubeCtx, namespace string) ([]T, error) {
	var probe PT = new(T)
	items, err := m.listGVR(ctx, kubeCtx, probe.gvr(), namespace)
	if err != nil {
		return nil, err
	}
	out := make([]T, len(items))
	for i := range items {
		PT(&out[i]).from(items[i])
	}
	return out, nil
}

// Get fetches a single object of model type T by name as a typed model.
func (m *Manager) Get[T any, PT resourceModel[T]](ctx context.Context, kubeCtx, namespace, name string) (T, error) {
	var out T
	var p PT = &out
	obj, err := m.getGVR(ctx, kubeCtx, p.gvr(), namespace, name)
	if err != nil {
		return out, err
	}
	p.from(*obj)
	return out, nil
}

// listWorkers bounds the concurrency of ListResources.
const listWorkers = 3

// ResourceTable is one kind's worth of rows for the dashboard.
type ResourceTable struct {
	Kind  string `json:"kind"`
	Items any    `json:"items"`
}

// ListResources fetches every dashboard kind for a namespace through a bounded
// worker pool, in a stable order. Failed kinds are skipped (logged by severity,
// see logListError) and empty kinds omitted; a bad context returns an error.
func (m *Manager) ListResources(ctx context.Context, kubeCtx, namespace string) ([]ResourceTable, error) {
	// Build the client up front so a bad context errors here, not as empty results.
	if err := m.CheckContext(kubeCtx); err != nil {
		return nil, err
	}

	// One fetch per kind, in the order the dashboard renders them.
	fetches := []struct {
		kind string
		run  func() (any, int, error)
	}{
		{"Services", func() (any, int, error) { v, err := m.List[Service](ctx, kubeCtx, namespace); return v, len(v), err }},
		{"Deployments", func() (any, int, error) {
			v, err := m.List[Deployment](ctx, kubeCtx, namespace)
			return v, len(v), err
		}},
		{"Ingresses", func() (any, int, error) { v, err := m.List[Ingress](ctx, kubeCtx, namespace); return v, len(v), err }},
		{"StatefulSets", func() (any, int, error) {
			v, err := m.List[StatefulSet](ctx, kubeCtx, namespace)
			return v, len(v), err
		}},
		{"DaemonSets", func() (any, int, error) { v, err := m.List[DaemonSet](ctx, kubeCtx, namespace); return v, len(v), err }},
		{"Jobs", func() (any, int, error) { v, err := m.List[Job](ctx, kubeCtx, namespace); return v, len(v), err }},
		{"CronJobs", func() (any, int, error) { v, err := m.List[CronJob](ctx, kubeCtx, namespace); return v, len(v), err }},
	}

	out := make([]ResourceTable, len(fetches))
	jobs := make(chan int)
	var wg sync.WaitGroup

	for range listWorkers {
		wg.Go(func() {
			// Each index is written by exactly one worker, so out[i] is race-free.
			for i := range jobs {
				fetch := fetches[i]
				items, count, err := fetch.run()
				if err != nil {
					m.logListError(fetch.kind, err)
					continue
				}
				if count == 0 {
					continue
				}
				out[i] = ResourceTable{Kind: fetch.kind, Items: items}
			}
		})
	}

	for i := range fetches {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Drop kinds that never loaded (still zero-value).
	return slices.DeleteFunc(out, func(t ResourceTable) bool {
		return t.Kind == ""
	}), nil
}

// logListError logs a per-kind fetch failure by severity: absent kinds and
// permission denials are normal (debug), a canceled request is silent, and real
// faults (5xx, throttling, unreachable) warn.
func (m *Manager) logListError(kind string, err error) {
	switch {
	case errors.Is(err, ErrCanceled):
		// Client went away; nothing to report.
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrForbidden), errors.Is(err, ErrUnauthorized):
		m.logger.Debug("skipping resource kind", "kind", kind, "reason", err)
	default:
		m.logger.Warn("listing resource kind failed", "kind", kind, "err", err)
	}
}
