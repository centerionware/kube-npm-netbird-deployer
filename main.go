package main

import (
	"log"
	"os"

	v1 "npm-operator/api/v1alpha1"
	"npm-operator/controllers"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// CRD scheme (your operator types)
	utilruntime.Must(v1.AddToScheme(scheme))

	// Kubernetes built-in types
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
}

func main() {

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	cfg := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Printf("manager init failed: %v", err)
		os.Exit(1)
	}

	r := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
	}

	if err := controllers.Setup(mgr, r); err != nil {
		log.Printf("controller setup failed: %v", err)
		os.Exit(1)
	}

	log.Println("starting controller manager...")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Printf("manager exited: %v", err)
		os.Exit(1)
	}
}