package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "ts.opentelekomcloud.com", Version: "v1alpha1"}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&TypesenseCluster{},
		&TypesenseClusterList{},
	)
	return nil
}

func init() {
	utilruntime.Must(AddToScheme(runtime.NewScheme()))
}
