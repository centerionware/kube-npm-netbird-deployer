package controllers

import (
	"context"
	"fmt"

	v1 "kube-deploy/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanupRuntime deletes all resources created by the operator for this app.
// Used by both App and ContainerApp reconcilers on deletion.
func cleanupRuntime(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	// Deployment
	var deploy appsv1.Deployment
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &deploy); err == nil {
		log.Info("deleting deployment")
		if err := c.Delete(ctx, &deploy); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting deployment: %w", err)
		}
	}

	// Service
	var svc corev1.Service
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &svc); err == nil {
		log.Info("deleting service")
		if err := c.Delete(ctx, &svc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting service: %w", err)
		}
	}

	// Ingress
	var ing networkingv1.Ingress
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &ing); err == nil {
		log.Info("deleting ingress")
		if err := c.Delete(ctx, &ing); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting ingress: %w", err)
		}
	}

	// HTTPRoute (Gateway API)
	if err := deleteHTTPRoute(ctx, c, app.Name, app.Namespace); err != nil {
		log.Error(err, "HTTPRoute delete failed (best-effort)")
	}

	// HPA
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &hpa); err == nil {
		log.Info("deleting HPA")
		if err := c.Delete(ctx, &hpa); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("deleting HPA: %w", err)
		}
	}

	// ServiceAccount (if we created one)
	var sa corev1.ServiceAccount
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &sa); err == nil {
		if isOwnedByApp(sa.Labels, app.Name) {
			log.Info("deleting service account")
			if err := c.Delete(ctx, &sa); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("deleting service account: %w", err)
			}
		}
	}

	// PVCs created by EnsureVolumes — only delete ones we created
	if err := cleanupPVCs(ctx, c, app, log); err != nil {
		log.Error(err, "PVC cleanup failed (best-effort)")
	}

	log.Info("runtime cleanup complete")
	return nil
}

// cleanupPVCs deletes PVCs that were created by the operator for this app.
// PVCs from external claimNames (pre-existing) are left alone.
func cleanupPVCs(ctx context.Context, c client.Client, app *v1.App, log interface{ Info(string, ...any) }) error {
	for _, vol := range app.Spec.Run.Volumes {
		if vol.PVC == nil {
			continue // ConfigMap, Secret, EmptyDir, HostPath — not our PVCs
		}

		claimName := vol.PVC.ClaimName
		if claimName == "" {
			claimName = vol.Name // auto-named — we created it
		} else {
			continue // explicit claimName means it's pre-existing, leave it alone
		}

		var pvc corev1.PersistentVolumeClaim
		if err := c.Get(ctx, client.ObjectKey{Name: claimName, Namespace: app.Namespace}, &pvc); err == nil {
			log.Info("deleting PVC", "name", claimName)
			if err := c.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("deleting PVC %s: %w", claimName, err)
			}
		}
	}
	return nil
}

func isOwnedByApp(labels map[string]string, appName string) bool {
	return labels != nil && labels["kube-deploy/app"] == appName
}
