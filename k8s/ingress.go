package k8s

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Ingress struct {
	resourceMeta
	DeterministicDNS string `json:"deterministicDNS,omitempty"`
}

func (Ingress) gvr() schema.GroupVersionResource { return resourceGVR["ingresses"] }

func (i *Ingress) from(u unstructured.Unstructured) {
	i.fill(u)
	if dns := deterministicDNSName(u.GetAnnotations()); dns != "" {
		i.DeterministicDNS = dns
	}
}

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
