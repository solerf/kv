package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type StatefulSet struct {
	resourceMeta
}

func (StatefulSet) gvr() schema.GroupVersionResource { return resourceGVR["statefulsets"] }

func (s *StatefulSet) from(u unstructured.Unstructured) { s.fill(u) }
