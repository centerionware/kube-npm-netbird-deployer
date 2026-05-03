package controllers

import (
	"context"
	"fmt"
	"reflect"

	v1 "kube-deploy/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EnsureRBAC creates or updates the full RBAC chain for an app:
// ServiceAccount, Role(s), ClusterRole(s), RoleBinding(s), ClusterRoleBinding(s)
func EnsureRBAC(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	if app.Spec.RBAC == nil {
		return nil
	}

	rbac := app.Spec.RBAC
	appLabels := map[string]string{
		"kube-deploy/app":       app.Name,
		"kube-deploy/namespace": app.Namespace,
	}

	saName := rbac.ServiceAccountName
	if saName == "" {
		saName = app.Name
	}

	// --- ServiceAccount ---
	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: app.Namespace,
			Labels:    appLabels,
		},
	}
	if err := upsertServiceAccount(ctx, c, sa); err != nil {
		return fmt.Errorf("service account: %w", err)
	}
	log.Info("service account ready", "name", saName)

	// --- Roles (namespace-scoped, created by us) ---
	for _, roleDef := range rbac.Roles {
		role := rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleDef.Name,
				Namespace: app.Namespace,
				Labels:    appLabels,
			},
			Rules: buildPolicyRules(roleDef.Rules),
		}
		if err := upsertRole(ctx, c, role); err != nil {
			return fmt.Errorf("role %s: %w", roleDef.Name, err)
		}
		log.Info("role ready", "name", roleDef.Name)

		// Bind this Role to the ServiceAccount
		rb := rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", app.Name, roleDef.Name),
				Namespace: app.Namespace,
				Labels:    appLabels,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: app.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleDef.Name,
			},
		}
		if err := upsertRoleBinding(ctx, c, rb); err != nil {
			return fmt.Errorf("role binding for %s: %w", roleDef.Name, err)
		}
	}

	// --- ClusterRoles (cluster-scoped, created by us) ---
	for _, crDef := range rbac.ClusterRoles {
		cr := rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s-%s", app.Namespace, app.Name, crDef.Name),
				Labels: appLabels,
			},
			Rules: buildPolicyRules(crDef.Rules),
		}
		if err := upsertClusterRole(ctx, c, cr); err != nil {
			return fmt.Errorf("cluster role %s: %w", crDef.Name, err)
		}
		log.Info("cluster role ready", "name", cr.Name)

		// Bind this ClusterRole to the ServiceAccount
		crb := rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s-%s", app.Namespace, app.Name, crDef.Name),
				Labels: appLabels,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: app.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     cr.Name,
			},
		}
		if err := upsertClusterRoleBinding(ctx, c, crb); err != nil {
			return fmt.Errorf("cluster role binding for %s: %w", crDef.Name, err)
		}
	}

	// --- Bind existing ClusterRoles (not created by us) ---
	for _, existingCR := range rbac.ClusterRoleBindings {
		crb := rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s-%s", app.Namespace, app.Name, existingCR),
				Labels: appLabels,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: app.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     existingCR,
			},
		}
		if err := upsertClusterRoleBinding(ctx, c, crb); err != nil {
			return fmt.Errorf("cluster role binding for existing role %s: %w", existingCR, err)
		}
		log.Info("bound existing cluster role", "clusterRole", existingCR)
	}

	// --- Bind existing Roles (not created by us) ---
	for _, existingRole := range rbac.RoleBindings {
		rb := rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", app.Name, existingRole),
				Namespace: app.Namespace,
				Labels:    appLabels,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: app.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     existingRole,
			},
		}
		if err := upsertRoleBinding(ctx, c, rb); err != nil {
			return fmt.Errorf("role binding for existing role %s: %w", existingRole, err)
		}
		log.Info("bound existing role", "role", existingRole)
	}

	return nil
}

// cleanupRBAC removes all RBAC resources created by the operator for this app
func cleanupRBAC(ctx context.Context, c client.Client, app *v1.App) error {
	log := log.FromContext(ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	if app.Spec.RBAC == nil {
		return nil
	}

	rbac := app.Spec.RBAC

	// ClusterRoles + ClusterRoleBindings we created
	for _, crDef := range rbac.ClusterRoles {
		name := fmt.Sprintf("%s-%s-%s", app.Namespace, app.Name, crDef.Name)
		var crb rbacv1.ClusterRoleBinding
		if err := c.Get(ctx, client.ObjectKey{Name: name}, &crb); err == nil {
			log.Info("deleting cluster role binding", "name", name)
			_ = c.Delete(ctx, &crb)
		}
		var cr rbacv1.ClusterRole
		if err := c.Get(ctx, client.ObjectKey{Name: name}, &cr); err == nil {
			log.Info("deleting cluster role", "name", name)
			_ = c.Delete(ctx, &cr)
		}
	}

	// ClusterRoleBindings to existing ClusterRoles
	for _, existingCR := range rbac.ClusterRoleBindings {
		name := fmt.Sprintf("%s-%s-%s", app.Namespace, app.Name, existingCR)
		var crb rbacv1.ClusterRoleBinding
		if err := c.Get(ctx, client.ObjectKey{Name: name}, &crb); err == nil {
			log.Info("deleting cluster role binding", "name", name)
			_ = c.Delete(ctx, &crb)
		}
	}

	// Roles + RoleBindings we created
	for _, roleDef := range rbac.Roles {
		rbName := fmt.Sprintf("%s-%s", app.Name, roleDef.Name)
		var rb rbacv1.RoleBinding
		if err := c.Get(ctx, client.ObjectKey{Name: rbName, Namespace: app.Namespace}, &rb); err == nil {
			log.Info("deleting role binding", "name", rbName)
			_ = c.Delete(ctx, &rb)
		}
		var role rbacv1.Role
		if err := c.Get(ctx, client.ObjectKey{Name: roleDef.Name, Namespace: app.Namespace}, &role); err == nil {
			if isOwnedByApp(role.Labels, app.Name) {
				log.Info("deleting role", "name", roleDef.Name)
				_ = c.Delete(ctx, &role)
			}
		}
	}

	// RoleBindings to existing Roles
	for _, existingRole := range rbac.RoleBindings {
		rbName := fmt.Sprintf("%s-%s", app.Name, existingRole)
		var rb rbacv1.RoleBinding
		if err := c.Get(ctx, client.ObjectKey{Name: rbName, Namespace: app.Namespace}, &rb); err == nil {
			log.Info("deleting role binding", "name", rbName)
			_ = c.Delete(ctx, &rb)
		}
	}

	// ServiceAccount
	saName := rbac.ServiceAccountName
	if saName == "" {
		saName = app.Name
	}
	var sa corev1.ServiceAccount
	if err := c.Get(ctx, client.ObjectKey{Name: saName, Namespace: app.Namespace}, &sa); err == nil {
		if isOwnedByApp(sa.Labels, app.Name) {
			log.Info("deleting service account", "name", saName)
			_ = c.Delete(ctx, &sa)
		}
	}

	return nil
}

// --- upsert helpers ---

func upsertServiceAccount(ctx context.Context, c client.Client, obj corev1.ServiceAccount) error {
	var existing corev1.ServiceAccount
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	return err
}

func upsertRole(ctx context.Context, c client.Client, obj rbacv1.Role) error {
	var existing rbacv1.Role
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(existing.Rules, obj.Rules) {
		existing.Rules = obj.Rules
		return c.Update(ctx, &existing)
	}
	return nil
}

func upsertClusterRole(ctx context.Context, c client.Client, obj rbacv1.ClusterRole) error {
	var existing rbacv1.ClusterRole
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(existing.Rules, obj.Rules) {
		existing.Rules = obj.Rules
		return c.Update(ctx, &existing)
	}
	return nil
}

func upsertRoleBinding(ctx context.Context, c client.Client, obj rbacv1.RoleBinding) error {
	var existing rbacv1.RoleBinding
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	return err
}

func upsertClusterRoleBinding(ctx context.Context, c client.Client, obj rbacv1.ClusterRoleBinding) error {
	var existing rbacv1.ClusterRoleBinding
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, &obj)
	}
	return err
}

func buildPolicyRules(rules []v1.RBACRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(rules))
	for i, r := range rules {
		out[i] = rbacv1.PolicyRule{
			APIGroups:     r.APIGroups,
			Resources:     r.Resources,
			Verbs:         r.Verbs,
			ResourceNames: r.ResourceNames,
		}
	}
	return out
}
