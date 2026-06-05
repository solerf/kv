package k8s

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	dynamic   dynamic.Interface
	clientset kubernetes.Interface
}

func NewClient(kubeconfig, kubeContext string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}

	if !hasKubeconfig(rules) {
		return nil, fmt.Errorf("kubeconfig not found: could not connect to a Kubernetes cluster.\n\nSet KUBECONFIG, pass --kubeconfig, or place a config at ~/.kube/config")
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig found but could not connect: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Kubernetes cluster: %w", err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %w", err)
	}

	return &Client{dynamic: dyn, clientset: cs}, nil
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

func (c *Client) List(ctx context.Context, kind, namespace string) ([]unstructured.Unstructured, error) {
	gvr, ok := resourceGVR[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	var res *unstructured.UnstructuredList
	var err error

	if namespace == "" {
		res, err = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		res, err = c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", kind, err)
	}

	return res.Items, nil
}

func (c *Client) Get(ctx context.Context, kind, namespace, name string) (*unstructured.Unstructured, error) {
	gvr, ok := resourceGVR[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	var res *unstructured.Unstructured
	var err error

	if namespace == "" {
		res, err = c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	} else {
		res, err = c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("getting %s/%s: %w", kind, name, err)
	}

	return res, nil
}

func ExtractStatus(item unstructured.Unstructured) string {
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

// ExtractMainContainerImage returns the image of the main application container
// Prioritizes containers with common main app names (app, main) over sidecars
func ExtractMainContainerImage(item unstructured.Unstructured) string {
	containers, found, _ := unstructured.NestedSlice(item.Object, "spec", "containers")
	if !found || len(containers) == 0 {
		return "-"
	}

	// Common names for main application containers
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

func NestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	return unstructured.NestedString(obj, fields...)
}

func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) StreamLogs(ctx context.Context, namespace, name string, follow bool) (io.ReadCloser, error) {
	tailLines := int64(200)
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	return req.Stream(ctx)
}

type PodMetrics struct {
	Timestamp string `json:"timestamp"`
	CPU       int64  `json:"cpu"`
	Memory    int64  `json:"memory"`
}

func (c *Client) GetPodMetrics(ctx context.Context, namespace, name string) (*PodMetrics, error) {
	gvr := schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}

	obj, err := c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod metrics: %w", err)
	}

	containers, found, _ := unstructured.NestedSlice(obj.Object, "containers")
	if !found || len(containers) == 0 {
		return &PodMetrics{}, nil
	}

	var totalCPU, totalMem int64
	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		usage, _, _ := unstructured.NestedStringMap(container, "usage")
		if cpu, ok := usage["cpu"]; ok {
			totalCPU += parseCPU(cpu)
		}
		if mem, ok := usage["memory"]; ok {
			totalMem += parseMem(mem)
		}
	}

	ts, _, _ := unstructured.NestedString(obj.Object, "timestamp")

	return &PodMetrics{
		Timestamp: ts,
		CPU:       totalCPU,
		Memory:    totalMem,
	}, nil
}

const (
	nanocoresToMillicores = 1_000_000
	coresToMillicores     = 1000
	kiBToMiB              = 1024
	miBToMiB              = 1
	giBToMiB              = 1024
	tiBToMiB              = 1024 * 1024
	piBToMiB              = 1024 * 1024 * 1024
	bytesToMiB            = 1024 * 1024
)

func parseCPU(s string) int64 {
	var val int64
	if len(s) > 0 && s[len(s)-1] == 'n' {
		fmt.Sscanf(s, "%dn", &val)
		return val / nanocoresToMillicores
	}
	if len(s) > 0 && s[len(s)-1] == 'm' {
		fmt.Sscanf(s, "%dm", &val)
		return val
	}
	fmt.Sscanf(s, "%d", &val)
	return val * coresToMillicores
}

func parseMem(s string) int64 {
	var val int64
	if len(s) > 2 && s[len(s)-2:] == "Ki" {
		fmt.Sscanf(s, "%dKi", &val)
		return val / kiBToMiB
	}
	if len(s) > 2 && s[len(s)-2:] == "Mi" {
		fmt.Sscanf(s, "%dMi", &val)
		return val * miBToMiB
	}
	if len(s) > 2 && s[len(s)-2:] == "Gi" {
		fmt.Sscanf(s, "%dGi", &val)
		return val * giBToMiB
	}
	if len(s) > 2 && s[len(s)-2:] == "Ti" {
		fmt.Sscanf(s, "%dTi", &val)
		return val * tiBToMiB
	}
	if len(s) > 2 && s[len(s)-2:] == "Pi" {
		fmt.Sscanf(s, "%dPi", &val)
		return val * piBToMiB
	}
	fmt.Sscanf(s, "%d", &val)
	return val / bytesToMiB
}
