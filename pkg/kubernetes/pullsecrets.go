package kubernetes

import (
	"context"
	"fmt"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/docker"
	perrors "github.com/pkg/errors"
	k8sv1 "k8s.io/api/core/v1"
)

func (k *KubernetesDriver) EnsurePullSecret(
	ctx context.Context,
	pullSecretName string,
	dockerImage string,
) (bool, error) {
	k.Log.Debugf("Ensure pull secrets")

	host, err := GetRegistryFromImageName(dockerImage)
	if err != nil {
		return false, fmt.Errorf("get registry from image name: %w", err)
	}

	dockerCredentials, err := docker.GetAuthConfig(host)
	if err != nil || dockerCredentials == nil || dockerCredentials.Username == "" || dockerCredentials.Secret == "" {
		k.Log.Debugf("Couldn't retrieve credentials for registry: %s", host)
		return false, nil
	}

	if k.secretExists(ctx, pullSecretName) {
		k.Log.Debugf("Pull secret '%s' already exists. Recreating...", pullSecretName)
		err := k.DeletePullSecret(ctx, pullSecretName)
		if err != nil {
			return false, err
		}
	}

	err = k.createPullSecret(ctx, pullSecretName, dockerCredentials)
	if err != nil {
		return false, err
	}

	k.Log.Infof("Pull secret '%s' created", pullSecretName)
	return true, nil
}

func (k *KubernetesDriver) DeletePullSecret(
	ctx context.Context,
	pullSecretName string) error {
	if !k.secretExists(ctx, pullSecretName) {
		return nil
	}

	args := []string{
		"delete",
		"secret",
		pullSecretName,
	}

	out, err := k.buildCmd(ctx, args).CombinedOutput()
	if err != nil {
		return perrors.Wrapf(err, "delete pull secret: %s", string(out))
	}

	return nil
}

func (k *KubernetesDriver) secretExists(
	ctx context.Context,
	pullSecretName string,
) bool {
	args := []string{
		"get",
		"secret",
		pullSecretName,
	}

	_, err := k.buildCmd(ctx, args).CombinedOutput()
	if err != nil {
		return false
	}
	return true
}

func (k *KubernetesDriver) createPullSecret(
	ctx context.Context,
	pullSecretName string,
	dockerCredentials *docker.Credentials,
) error {

	authToken := dockerCredentials.Secret
	if dockerCredentials.Username != "" {
		authToken = dockerCredentials.Username + ":" + authToken
	}

	email := "noreply@loft.sh"

	encodedSecretData, err := PreparePullSecretData(dockerCredentials.ServerURL, authToken, email)
	if err != nil {
		return perrors.Wrap(err, "prepare pull secret data")
	}

	args := []string{
		"create",
		"secret",
		"generic",
		pullSecretName,
		"--type", string(k8sv1.SecretTypeDockerConfigJson),
		"--from-literal", encodedSecretData,
	}

	out, err := k.buildCmd(ctx, args).CombinedOutput()
	if err != nil {
		return perrors.Wrapf(err, "create pull secret: %s", string(out))
	}

	return nil
}