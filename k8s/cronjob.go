package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CronJob struct {
	resourceMeta
}

func (CronJob) gvr() schema.GroupVersionResource { return resourceGVR["cronjobs"] }

func (c *CronJob) from(u unstructured.Unstructured) { c.fill(u) }
