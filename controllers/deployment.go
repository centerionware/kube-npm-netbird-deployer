package controllers

import (
	"context"
	"reflect"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureDeployment(ctx context.Context, c client.Client, app v1.NpmApp, image string) error {

	var dep appsv1.Deployment

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &dep)

	desiredEnv := []corev1.EnvVar{}
	for k, v := range app.Spec.Env {
		desiredEnv = append(desiredEnv, corev1.EnvVar{Name: k, Value: v})
	}

	command := []string{}
	if app.Spec.Run.Command != "" {
		command = []string{"sh", "-c", app.Spec.Run.Command}
	}

	port := int32(app.Spec.Run.Port)

	if errors.IsNotFound(err) {

		dep = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: app.APIVersion,
						Kind:       app.Kind,
						Name:       app.Name,
						UID:        app.UID,
						Controller: boolPtr(true),
					},
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": app.Name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": app.Name},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "app",
								Image:   image,
								Command: command,
								Env:     desiredEnv,
								Ports: []corev1.ContainerPort{
									{ContainerPort: port},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"cpu":    resource.MustParse("100m"),
										"memory": resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
		}

		return c.Create(ctx, &dep)
	}

	updated := false
	cn := &dep.Spec.Template.Spec.Containers[0]

	if image != "" && cn.Image != image {
		cn.Image = image
		updated = true
	}

	if !reflect.DeepEqual(cn.Env, desiredEnv) {
		cn.Env = desiredEnv
		updated = true
	}

	if !reflect.DeepEqual(cn.Command, command) {
		cn.Command = command
		updated = true
	}

	if len(cn.Ports) == 0 || cn.Ports[0].ContainerPort != port {
		cn.Ports = []corev1.ContainerPort{{ContainerPort: port}}
		updated = true
	}

	if updated {
		return c.Update(ctx, &dep)
	}

	return nil
}