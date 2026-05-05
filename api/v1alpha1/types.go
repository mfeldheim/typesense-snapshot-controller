package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TypesenseCluster is a minimal read-only representation of the ts.opentelekomcloud.com CRD.
// We only need to watch its existence — all snapshot config comes from env vars.
type TypesenseCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TypesenseClusterSpec `json:"spec,omitempty"`
}

type TypesenseClusterSpec struct {
	Replicas int32 `json:"replicas,omitempty"`
}

type TypesenseClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TypesenseCluster `json:"items"`
}

// DeepCopyObject implements Object interface.
func (in *TypesenseCluster) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(TypesenseCluster)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec
	return out
}

// DeepCopyObject implements Object interface.
func (in *TypesenseClusterList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(TypesenseClusterList)
	*out = *in
	out.ListMeta = *in.ListMeta.DeepCopy()
	if in.Items != nil {
		out.Items = make([]TypesenseCluster, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *TypesenseCluster) DeepCopyInto(out *TypesenseCluster) {
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec
}
