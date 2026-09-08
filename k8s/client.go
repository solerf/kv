package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// client is one connection to a single kube context; callers go through Manager.
type client struct {
	dynamic    dynamic.Interface
	clientset  kubernetes.Interface
	httpClient *http.Client
}

// ErrKubeconfigNotFound is returned when no kubeconfig can be located.
var ErrKubeconfigNotFound = errors.New("kubeconfig not found: could not connect to a Kubernetes cluster")

func newClient(kubeconfig, kubeContext string) (*client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}

	if !hasKubeconfig(rules) {
		return nil, fmt.Errorf("%w\n\nSet KUBECONFIG, pass --kubeconfig, or place a config at ~/.kube/config", ErrKubeconfigNotFound)
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig found but could not connect: %w", err)
	}

	// Share one HTTP client so close() can release the connection pool as a unit.
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("could not build HTTP client: %w", err)
	}

	dyn, err := dynamic.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Kubernetes cluster: %w", err)
	}

	cs, err := kubernetes.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %w", err)
	}

	return &client{dynamic: dyn, clientset: cs, httpClient: httpClient}, nil
}

// close releases idle keep-alive connections; in-flight requests are unaffected.
func (c *client) close() {
	c.httpClient.CloseIdleConnections()
}

func hasKubeconfig(rules *clientcmd.ClientConfigLoadingRules) bool {
	if rules.ExplicitPath != "" {
		_, err := os.Stat(rules.ExplicitPath)
		return err == nil
	}
	for _, path := range rules.Precedence {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// resourceGVR is the single source of truth for each supported kind's GVR (GroupVersionResource),
// keyed by the kind string used in requests.
var resourceGVR = map[string]schema.GroupVersionResource{
	"pods":         {Group: "", Version: "v1", Resource: "pods"},
	"services":     {Group: "", Version: "v1", Resource: "services"},
	"deployments":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulsets": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonsets":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"configmaps":   {Group: "", Version: "v1", Resource: "configmaps"},
	"secrets":      {Group: "", Version: "v1", Resource: "secrets"},
	"ingresses":    {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"jobs":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjobs":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
}

func (c *client) listGVR(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]unstructured.Unstructured, error) {
	var res *unstructured.UnstructuredList
	var err error

	if namespace == "" {
		res, err = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		res, err = c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, classify(err)
	}

	return res.Items, nil
}

func (c *client) getGVR(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	var (
		obj *unstructured.Unstructured
		err error
	)
	if namespace == "" {
		obj, err = c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	return obj, classify(err)
}

func (c *client) deletePod(ctx context.Context, namespace, name string) error {
	return classify(c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func (c *client) streamLogs(ctx context.Context, namespace, name string, follow bool) (io.ReadCloser, error) {
	tailLines := int64(200)
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	rc, err := req.Stream(ctx)
	return rc, classify(err)
}

// formatAge renders a creation timestamp the way the UI expects it.
func formatAge(u unstructured.Unstructured) string {
	return u.GetCreationTimestamp().Format("2006-01-02 15:04")
}

func extractStatus(item unstructured.Unstructured) string {
	// For pods, surface a container's waiting reason (e.g. CrashLoopBackOff)
	// since status.phase stays "Running" while a container keeps restarting.
	if reason := containerWaitingReason(item); reason != "" {
		return reason
	}

	phase, found, _ := unstructured.NestedString(item.Object, "status", "phase")
	if found {
		return phase
	}

	// For workloads (deployments, statefulsets, daemonsets, ...) report the
	// ready replica count. This must come before the generic conditions check,
	// otherwise we'd return a condition type (e.g. "Available") instead.
	replicas, found, _ := unstructured.NestedInt64(item.Object, "status", "readyReplicas")
	if found {
		desired, _, _ := unstructured.NestedInt64(item.Object, "spec", "replicas")
		return fmt.Sprintf("%d/%d ready", replicas, desired)
	}

	conditions, found, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	if found && len(conditions) > 0 {
		last := conditions[len(conditions)-1]
		if cond, ok := last.(map[string]interface{}); ok {
			if t, ok := cond["type"].(string); ok {
				return t
			}
		}
	}

	return "-"
}

// containerWaitingReason returns the first container waiting reason found in a
// pod's status (e.g. CrashLoopBackOff, ImagePullBackOff), or "" if none.
func containerWaitingReason(item unstructured.Unstructured) string {
	statuses, found, _ := unstructured.NestedSlice(item.Object, "status", "containerStatuses")
	if !found {
		return ""
	}
	for _, s := range statuses {
		cs, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if reason, _, _ := unstructured.NestedString(cs, "state", "waiting", "reason"); reason != "" {
			return reason
		}
	}
	return ""
}

// extractMainContainerImage returns the image of the main application
// container, preferring containers with common main app names over sidecars.
func extractMainContainerImage(item unstructured.Unstructured) string {
	containers, found, _ := unstructured.NestedSlice(item.Object, "spec", "containers")
	if !found || len(containers) == 0 {
		return "-"
	}

	mainContainerNames := []string{"app", "main", "application"}

	// First pass: look for containers with common main app names
	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := container["name"].(string)
		if !ok {
			continue
		}

		for _, mainName := range mainContainerNames {
			if name == mainName {
				if image, ok := container["image"].(string); ok {
					return image
				}
			}
		}
	}

	// Fallback: return the first container if no match found
	if firstContainer, ok := containers[0].(map[string]interface{}); ok {
		if image, ok := firstContainer["image"].(string); ok {
			return image
		}
	}

	return "-"
}
