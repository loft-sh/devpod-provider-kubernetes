package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/loft-sh/devpod/pkg/command"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *KubernetesDriver) createServiceAccount(ctx context.Context, id, serviceAccount string) error {
	// try to find serviceaccount
	out, err := k.findServiceAccount(ctx, serviceAccount)
	if err != nil {
		return command.WrapCommandError(out, err)
	} else if len(out) == 0 {
		// create service account if it does not exist
		serviceAccountRaw, err := json.Marshal(&corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceAccount",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   serviceAccount,
				Labels: ExtraDevPodLabels,
			},
		})
		if err != nil {
			return err
		}

		k.Log.Infof("Create Service Account '%s'", serviceAccount)
		buf := &bytes.Buffer{}
		err = k.runCommand(ctx, []string{"create", "-f", "-"}, bytes.NewReader(serviceAccountRaw), buf, buf)
		if err != nil {
			return errors.Wrapf(err, "create service account: %s", buf.String())
		} else if k.isDryRun() {
			k.dryRun.AddManifest(buf.Bytes())
		}
	}

	// try to find role binding
	if k.options.ClusterRole != "" {
		out, err = k.findRoleBinding(ctx, id)
		if err != nil {
			return command.WrapCommandError(out, err)
		} else if len(out) == 0 {
			// create role binding
			roleBindingRaw, err := json.Marshal(&rbacv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RoleBinding",
					APIVersion: rbacv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   id,
					Labels: ExtraDevPodLabels,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "ServiceAccount",
						Name: serviceAccount,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "ClusterRole",
					Name:     k.options.ClusterRole,
				},
			})
			if err != nil {
				return err
			}

			k.Log.Infof("Create Role Binding '%s'", serviceAccount)
			buf := &bytes.Buffer{}
			err = k.runCommand(ctx, []string{"create", "-f", "-"}, bytes.NewReader(roleBindingRaw), buf, buf)
			if err != nil {
				return errors.Wrapf(err, "create role binding: %s", buf.String())
			} else if k.isDryRun() {
				k.dryRun.AddManifest(buf.Bytes())
			}
		}
	}

	return nil
}

func (k *KubernetesDriver) findServiceAccount(ctx context.Context, serviceAccount string) ([]byte, error) {
	if k.isDryRun() {
		return nil, nil
	}
	return k.buildCmd(ctx, []string{"get", "serviceaccount", serviceAccount, "--ignore-not-found", "-o", "json"}).Output()
}

func (k *KubernetesDriver) findRoleBinding(ctx context.Context, id string) ([]byte, error) {
	if k.isDryRun() {
		return nil, nil
	}
	return k.buildCmd(ctx, []string{"get", "rolebinding", id, "--ignore-not-found", "-o", "json"}).Output()
}
