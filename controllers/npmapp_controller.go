package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "npm-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"
)

type NpmAppReconciler struct {
	client.Client
}

// ---------------- ENTRY POINT ----------------

func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	image := resolveImage(app)

	// STEP 1: BUILD (sync job)
	jobCompleted, err := r.ensureBuild(ctx, app, image)
	if err != nil {
		return ctrl.Result{}, err
	}

	// wait for build
	if !jobCompleted {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// STEP 2: DEPLOY
	if err := r.ensureDeployment(ctx, app, image); err != nil {
		return ctrl.Result{}, err
	}

	// STEP 3: SERVICE
	if err := r.ensureService(ctx, app); err != nil {
		return ctrl.Result{}, err
	}

	// STEP 4: STATUS
	app.Status.Image = image
	app.Status.Phase = "Ready"
	_ = r.Status().Update(ctx, &app)

	return ctrl.Result{}, nil
}

// ---------------- BUILD ----------------

func (r *NpmAppReconciler) ensureBuild(ctx context.Context, app v1.NpmApp, image string) (bool, error) {

	jobName := app.Name + "-build"

	// If job exists and succeeded → done
	var job corev1.Pod
	err := r.Get(ctx, client.ObjectKey{
		Name:      jobName,
		Namespace: app.Namespace,
	}, &job)

	if err == nil {
		if job.Status.Phase == corev1.PodSucceeded {
			return true, nil
		}
		return false, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	// Create build pod (BuildKit inline)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: app.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,

			InitContainers: []corev1.Container{
				{
					Name:  "clone",
					Image: "alpine/git",
					Command: []string{
						"sh", "-c",
						fmt.Sprintf("git clone %s /workspace", app.Spec.Repo),
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "ws", MountPath: "/workspace"},
					},
				},
			},

			Containers: []corev1.Container{
				{
					Name:  "build",
					Image: "moby/buildkit:rootless",
					Command: []string{
						"buildctl-daemonless.sh",
						"build",
						"--frontend=dockerfile.v0",
						"--local=context=/workspace",
						"--local=dockerfile=/workspace",
						"--output",
						fmt.Sprintf("type=image,name=%s,push=true", image),
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "ws", MountPath: "/workspace"},
					},
				},
			},

			Volumes: []corev1.Volume{
				{
					Name: "ws",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	return false, r.Create(ctx, &pod)
}

// ---------------- DEPLOYMENT ----------------

func (r *NpmAppReconciler) ensureDeployment(ctx context.Context, app v1.NpmApp, image string) error {

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptrInt32(1),
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
							Name:  "app",
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(app.Spec.Run.Port)},
							},
						},
					},
				},
			},
		},
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, &deploy)
	}

	if err != nil {
		return err
	}

	existing.Spec = deploy.Spec
	return r.Update(ctx, &existing)
}

// ---------------- SERVICE ----------------

func (r *NpmAppReconciler) ensureService(ctx context.Context, app v1.NpmApp) error {

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Annotations: mergeAnnotations(app.Spec.Service.Annotations, map[string]string{
				"netbird.io/expose": "true",
			}),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": app.Name},
			Ports: []corev1.ServicePort{
				{
					Port: int32(app.Spec.Run.Port),
				},
			},
		},
	}

	var existing corev1.Service
	err := r.Get(ctx, client.ObjectKey{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, &svc)
	}

	if err != nil {
		return err
	}

	existing.Annotations = svc.Annotations
	existing.Spec = svc.Spec
	return r.Update(ctx, &existing)
}

// ---------------- HELPERS ----------------

func resolveImage(app v1.NpmApp) string {
	return fmt.Sprintf(
		"registry.registry.svc.cluster.local:5000/apps/%s:latest",
		app.Name,
	)
}

func mergeAnnotations(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func ptrInt32(i int32) *int32 {
	return &i
}