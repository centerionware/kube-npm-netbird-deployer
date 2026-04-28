package main

import (
	"npm-operator/controllers"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {

	ctrl.SetLogger(zap.New())

	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})

	reconciler := &controllers.NpmAppReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	ctrl.NewControllerManagedBy(mgr).
		For(&controllers.NpmApp{}).
		Complete(reconciler)

	mgr.Start(ctrl.SetupSignalHandler())
}