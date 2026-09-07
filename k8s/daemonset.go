package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DaemonSet struct {
	resourceMeta
}

func (DaemonSet) gvr() schema.GroupVersionResource { return resourceGVR["daemonsets"] }

func (d *DaemonSet) from(u unstructured.Unstructured) { d.fill(u) }
