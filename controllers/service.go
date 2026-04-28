package controllers

import (
	v1 "npm-operator/api/v1alpha1"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"context"
)

func ensureService(ctx context.Context, c client.Client, app v1.NpmApp) error {

	var svc core.Service

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &svc)

	if errors.IsNotFound(err) {

		svc = core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
			Spec: core.ServiceSpec{
				Selector: map[string]string{"app": app.Name},
				Ports: []core.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(app.Spec.Run.Port),
					},
				},
				Type: core.ServiceTypeClusterIP,
			},
		}

		return c.Create(ctx, &svc)
	}

	return nil
}