package main

import (
	"npm-operator/api/v1alpha1"
	"npm-operator/controllers"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func main() {

	ctrl.SetLogger(zap.New())

	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	scheme.AddKnownTypes(v1alpha1.GroupVersion, &v1alpha1.NpmApp{})

	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})

	reconciler := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
		Scheme: scheme,
	}

	c, _ := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NpmApp{}).
		Build(reconciler)

	c.Watch(
		&source.Kind{Type: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kpack.io/v1alpha2",
				"kind":       "Image",
			},
		}},
		handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&v1alpha1.NpmApp{},
		),
	)

	mgr.Start(ctrl.SetupSignalHandler())
}