package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "npm-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureBuildJob(ctx context.Context, c client.Client, app v1.NpmApp) (string, error) {

	jobName := app.Name + "-build"
	image := resolveImage(app)

	var job batchv1.Job
	err := c.Get(ctx, client.ObjectKey{
		Name:      jobName,
		Namespace: app.Namespace,
	}, &job)

	if err == nil {
		return image, nil
	}

	if !errors.IsNotFound(err) {
		return "", err
	}

	// Dockerfile
	dockerfile := generateDockerfile(app)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-dockerfile",
			Namespace: app.Namespace,
		},
		Data: map[string]string{
			"Dockerfile": dockerfile,
		},
	}
	_ = c.Create(ctx, &cm)

	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "dockerfile",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name,
					},
				},
			},
		},
	}

	init := corev1.Container{
		Name:  "git-clone",
		Image: "alpine/git",
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("git clone %s /workspace", app.Spec.Repo),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}

	build := corev1.Container{
		Name:  "build",
		Image: "moby/buildkit:rootless",
		Command: []string{
			"buildctl-daemonless.sh",
			"build",
			"--progress=plain",
			"--frontend=dockerfile.v0",
			"--local=context=/workspace",
			"--local=dockerfile=/workspace",
			"--output",
			fmt.Sprintf("type=image,name=%s,push=true", image),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}

	job = batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: app.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptrInt32(2),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{init},
					Containers:     []corev1.Container{build},
					Volumes:        volumes,
				},
			},
		},
	}

	if err := c.Create(ctx, &job); err != nil {
		return "", err
	}

	return image, nil
}

func ptrInt32(i int32) *int32 {
	return &i
}