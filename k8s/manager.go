package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// ErrUnknownContext is returned when the requested kube context is not
// configured. ErrContextUnavailable is returned when a configured context's
// client cannot be created (e.g. the context is missing from the kubeconfig).
var (
	ErrUnknownContext     = errors.New("unknown context")
	ErrContextUnavailable = errors.New("context unavailable")
)

// clientFactory lazily builds (and memoizes) the client for one context.
type clientFactory = func() (*client, error)

// Manager is the single public facade the rest of the app depends on. It is
// created by main and injected into the server. Each configured context gets a
// lazy factory at construction; the client itself is built on first use, so a
// context that is never opened costs only a small thunk.
type Manager struct {
	kubeconfig string
	// clients maps a configured context to its lazy factory. Keys are fixed at
	// construction and never added or removed, so reads need no lock; the only
	// write is swapping a failed factory for a fresh one on the error path,
	// which sync.Map handles.
	clients sync.Map // map[string]clientFactory
	// created records the clients that were actually built, so Close can shut
	// them down without instantiating the ones that were never used.
	created sync.Map // map[string]*client
}

// NewManager registers a lazy factory per configured context without connecting
// to any of them. It errors only when no context is configured.
func NewManager(kubeconfig string, contexts []string) (*Manager, error) {
	m := &Manager{kubeconfig: kubeconfig}

	var n int
	for _, kubeContext := range contexts {
		if _, exists := m.clients.Load(kubeContext); exists {
			continue
		}
		m.clients.Store(kubeContext, m.newFactory(kubeContext))
		n++
	}

	if n == 0 {
		return nil, fmt.Errorf("no contexts configured")
	}

	return m, nil
}

// newFactory returns a memoizing factory for one context. sync.OnceValues runs
// newClient at most once and serializes concurrent first callers. A client that
// is built successfully is recorded so Close can shut it down.
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

// Close releases the connections of every client that was actually created.
// It is safe to call once during shutdown; unused contexts are left untouched.
func (m *Manager) Close() {
	m.created.Range(func(_, v any) bool {
		v.(*client).close()
		return true
	})
}

// CheckContext reports whether the context can be served: nil if a client is
// available, ErrUnknownContext if the context is not configured, or a wrapped
// ErrContextUnavailable if the client cannot be created. It builds the client
// (once) as a side effect, so a later call reuses it.
func (m *Manager) CheckContext(kubeCtx string) error {
	_, err := m.client(kubeCtx)
	return err
}

// client returns the client for a configured context, building it on first use.
// An unconfigured context returns ErrUnknownContext; a configured context whose
// client cannot be built returns an error wrapping ErrContextUnavailable. On a
// creation failure the memoized error is replaced with a fresh factory so the
// next call retries rather than returning the same error forever.
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
		return "", fmt.Errorf("unsupported resource kind: %s", kind)
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

// resourceModel is satisfied by *T for each model type T: it knows its own GVR
// and how to populate itself from an unstructured object. The *T constraint
// lets List/Get infer the pointer type from the value type at the call site.
type resourceModel[T any] interface {
	*T
	gvr() schema.GroupVersionResource
	from(unstructured.Unstructured)
}

// List fetches all objects of model type T in a namespace, returning typed
// models. Usage: k8s.List[k8s.Deployment](mgr, ctx, kubeCtx, ns).
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

// Get fetches a single object of model type T by name, returning a typed model.
// Usage: k8s.Get[k8s.Pod](mgr, ctx, kubeCtx, ns, name).
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

// listWorkers bounds how many resource kinds are fetched concurrently in
// ListResources.
const listWorkers = 3

// ResourceTable is one kind's worth of rows for the dashboard.
type ResourceTable struct {
	Kind  string `json:"kind"`
	Items any    `json:"items"`
}

// ListResources fetches every dashboard resource kind for a namespace,
// concurrently through a bounded worker pool. A kind that errors (e.g. absent
// on the cluster) is skipped; an unknown or unavailable context returns an
// error. Tables keep a stable order and empty kinds are omitted.
func (m *Manager) ListResources(ctx context.Context, kubeCtx, namespace string) ([]ResourceTable, error) {
	// Build the client once up front so the workers reuse it (and so an
	// unusable context fails here instead of surfacing as an empty result).
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
			// Each index is handled by exactly one worker, so writing
			// out[i] from different goroutines is race-free.
			for i := range jobs {
				fetch := fetches[i]
				items, count, err := fetch.run()
				if err != nil || count == 0 {
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

	// guard to remove not loaded resources that were allocated in the table
	return slices.DeleteFunc(out, func(t ResourceTable) bool {
		return t.Kind == ""
	}), nil
}
