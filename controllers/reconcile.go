package controllers

import (
	"context"
	"time"

	v1 "npm-operator/api/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NpmAppReconciler struct {
	client.Client
}

func Setup(mgr ctrl.Manager, r *NpmAppReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NpmApp{}).
		Complete(r)
}

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	image, ready, err := EnsureBuild(ctx, r.Client, &app)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !ready {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if err := EnsureRuntime(ctx, r.Client, &app, image); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}