package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Service struct {
	resourceMeta
}

func (Service) gvr() schema.GroupVersionResource { return resourceGVR["services"] }

func (s *Service) from(u unstructured.Unstructured) { s.fill(u) }
