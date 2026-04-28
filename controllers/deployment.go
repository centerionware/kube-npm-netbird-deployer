package controllers

import (
	v1 "npm-operator/api/v1alpha1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"context"
)

func ensureDeployment(ctx context.Context, c client.Client, app v1.NpmApp, image string) error {

	var dep apps.Deployment

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &dep)

	if errors.IsNotFound(err) {

		dep = apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
			Spec: apps.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": app.Name},
				},
				Template: core.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": app.Name},
					},
					Spec: core.PodSpec{
						Containers: []core.Container{
							{
								Name:  "app",
								Image: image,
								Ports: []core.ContainerPort{
									{ContainerPort: int32(app.Spec.Run.Port)},
								},
							},
						},
					},
				},
			},
		}

		return c.Create(ctx, &dep)
	}

	return nil
}