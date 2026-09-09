package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Pod struct {
	Name   string            `json:"name"`
	Status string            `json:"status"`
	Age    string            `json:"age"`
	IP     string            `json:"ip"`
	Node   string            `json:"node"`
	Image  string            `json:"image"`
	Labels map[string]string `json:"labels,omitempty"`
}

func (Pod) gvr() schema.GroupVersionResource { return resourceGVR["pods"] }

func (p *Pod) from(u unstructured.Unstructured) {
	ip, _, _ := unstructured.NestedString(u.Object, "status", "podIP")
	node, _, _ := unstructured.NestedString(u.Object, "spec", "nodeName")
	p.Name = u.GetName()
	p.Status = extractStatus(u)
	p.Age = formatAge(u)
	p.IP = ip
	p.Node = node
	p.Image = extractMainContainerImage(u)
	p.Labels = u.GetLabels()
}
