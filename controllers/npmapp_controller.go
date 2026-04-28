package controllers

import (
	"context"
	"time"

	v1 "npm-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Reconciler

type NpmAppReconciler struct {
	client.Client
}

// ---------------- MAIN LOOP ----------------

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	commit, err := getGitSHA(app)
	if err != nil {
		app.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, &app)
		return reconcile.Result{}, err
	}

	app.Status.Commit = commit

	image := resolveImage(app)

	done, err := ensureBuildJob(ctx, r.Client, app, image, commit)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !done {
		app.Status.Phase = "Building"
		_ = r.Status().Update(ctx, &app)
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.ensureDeployment(ctx, app, image); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureService(ctx, app); err != nil {
		return reconcile.Result{}, err
	}

	app.Status.Phase = "Ready"
	app.Status.Image = image
	app.Status.LastGoodImage = image
	_ = r.Status().Update(ctx, &app)

	return reconcile.Result{}, nil
}