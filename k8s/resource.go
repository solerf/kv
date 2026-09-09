package k8s

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// resourceMeta is the common set of fields every resource card shows. Model
// types embed it so its fields promote into their JSON output.
type resourceMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Age       string            `json:"age"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (r *resourceMeta) fill(u unstructured.Unstructured) {
	r.Name = u.GetName()
	r.Namespace = u.GetNamespace()
	r.Age = formatAge(u)
	r.Status = extractStatus(u)
	r.Labels = u.GetLabels()
}
