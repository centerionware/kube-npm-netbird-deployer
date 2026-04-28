package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureKpackImage(ctx context.Context, c client.Client, app v1.NpmApp) error {

	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kpack.io",
		Version: "v1alpha2",
		Kind:    "Image",
	})

	image.SetName(app.Name)
	image.SetNamespace(app.Namespace)

	image.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: app.APIVersion,
			Kind:       app.Kind,
			Name:       app.Name,
			UID:        app.UID,
			Controller: boolPtr(true),
		},
	})

	image.Object["spec"] = map[string]interface{}{
		"tag": app.Spec.Image,
		"source": map[string]interface{}{
			"git": map[string]interface{}{
				"url":      app.Spec.Repo,
				"revision": app.Spec.Revision,
			},
		},
	}

	return c.Patch(ctx, image, client.Apply, client.ForceOwnership, client.FieldOwner("npm-operator"))
}

func getLatestImageDigest(ctx context.Context, c client.Client, app v1.NpmApp) (string, error) {

	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kpack.io",
		Version: "v1alpha2",
		Kind:    "Image",
	})

	err := c.Get(ctx, client.ObjectKey{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, image)

	if err != nil {
		return "", err
	}

	status, found, _ := unstructured.NestedMap(image.Object, "status")
	if !found {
		return "", nil
	}

	latest, found, _ := unstructured.NestedString(status, "latestImage")
	if !found {
		return "", nil
	}

	return latest, nil
}

func boolPtr(b bool) *bool {
	return &b
}