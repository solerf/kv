package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Deployment struct {
	resourceMeta
}

func (Deployment) gvr() schema.GroupVersionResource { return resourceGVR["deployments"] }

func (d *Deployment) from(u unstructured.Unstructured) { d.fill(u) }
