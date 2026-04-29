package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var GroupVersion = schema.GroupVersion{
	Group:   "npm.centerionware.app",
	Version: "v1alpha1",
}

var SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
var AddToScheme = SchemeBuilder.AddToScheme

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&NpmApp{},
		&NpmAppList{},
	)

	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}