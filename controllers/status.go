package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateStatus(ctx context.Context, c client.Client, app *v1.NpmApp, job batchv1.Job, image string) error {

	phase := "Building"

	if job.Status.Succeeded > 0 {
		phase = "Succeeded"
	}

	if job.Status.Failed > 0 {
		phase = "Failed"
	}

	app.Status.Phase = phase
	app.Status.Image = image
	app.Status.JobName = job.Name

	return c.Status().Update(ctx, app)
}