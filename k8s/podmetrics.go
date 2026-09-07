package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PodMetrics is the CPU/memory usage of a pod, summed across its containers and
// normalized to millicores and MiB.
type PodMetrics struct {
	Timestamp string `json:"timestamp"`
	CPU       int64  `json:"cpu"`
	Memory    int64  `json:"memory"`
}

var podMetricsGVR = schema.GroupVersionResource{
	Group:    "metrics.k8s.io",
	Version:  "v1beta1",
	Resource: "pods",
}

func (m *Manager) PodMetrics(ctx context.Context, kubeCtx, namespace, name string) (*PodMetrics, error) {
	obj, err := m.getGVR(ctx, kubeCtx, podMetricsGVR, namespace, name)
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

// parseQuantity scans the leading integer of a Kubernetes quantity using the
// given format, returning 0 if the value does not parse.
func parseQuantity(s, format string) int64 {
	var val int64
	if _, err := fmt.Sscanf(s, format, &val); err != nil {
		return 0
	}
	return val
}

func parseCPU(s string) int64 {
	switch {
	case len(s) > 0 && s[len(s)-1] == 'n':
		return parseQuantity(s, "%dn") / nanocoresToMillicores
	case len(s) > 0 && s[len(s)-1] == 'm':
		return parseQuantity(s, "%dm")
	default:
		return parseQuantity(s, "%d") * coresToMillicores
	}
}

func parseMem(s string) int64 {
	switch {
	case len(s) > 2 && s[len(s)-2:] == "Ki":
		return parseQuantity(s, "%dKi") / kiBToMiB
	case len(s) > 2 && s[len(s)-2:] == "Mi":
		return parseQuantity(s, "%dMi") * miBToMiB
	case len(s) > 2 && s[len(s)-2:] == "Gi":
		return parseQuantity(s, "%dGi") * giBToMiB
	case len(s) > 2 && s[len(s)-2:] == "Ti":
		return parseQuantity(s, "%dTi") * tiBToMiB
	case len(s) > 2 && s[len(s)-2:] == "Pi":
		return parseQuantity(s, "%dPi") * piBToMiB
	default:
		return parseQuantity(s, "%d") / bytesToMiB
	}
}
