package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Job struct {
	resourceMeta
}

func (Job) gvr() schema.GroupVersionResource { return resourceGVR["jobs"] }

func (j *Job) from(u unstructured.Unstructured) { j.fill(u) }
