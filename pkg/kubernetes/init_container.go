package kubernetes

import (
	"fmt"
	"strings"

	"github.com/loft-sh/devpod/pkg/driver"
	corev1 "k8s.io/api/core/v1"
)

func (k *KubernetesDriver) getInitContainer(options *driver.RunOptions, pod *corev1.Pod) ([]corev1.Container, error) {
	commands := []string{}

	// find the volume type mounts
	volumeMounts := []corev1.VolumeMount{}
	for idx, mount := range options.Mounts {
		if mount.Type != "volume" {
			continue
		}

		volumeMount := getVolumeMount(idx+1, mount)
		copyFrom := volumeMount.MountPath
		volumeMount.MountPath = "/" + volumeMount.SubPath
		volumeMounts = append(volumeMounts, volumeMount)
		commands = append(commands, fmt.Sprintf(`cp -a %s/. %s/ || true`, strings.TrimRight(copyFrom, "/"), strings.TrimRight(volumeMount.MountPath, "/")))
	}

	// check if there is at least one mount
	if len(volumeMounts) == 0 {
		return nil, nil
	}

	securityContext := &corev1.SecurityContext{
		RunAsUser:    &[]int64{0}[0],
		RunAsGroup:   &[]int64{0}[0],
		RunAsNonRoot: &[]bool{false}[0],
	}
	if k.options.StrictSecurity {
		securityContext = nil
	}

	initContainer := corev1.Container{
		Name:            "devpod-init",
		Image:           options.Image,
		Command:         []string{"sh"},
		Args:            []string{"-c", strings.Join(commands, "\n") + "\n"},
		Resources:       parseResources(k.options.HelperResources, k.Log),
		VolumeMounts:    volumeMounts,
		SecurityContext: securityContext,
	}

	// look for existing init container definition
	var existingInitContainer *corev1.Container
	if len(pod.Spec.Containers) > 0 {
		for i, container := range pod.Spec.Containers {
			if container.Name == InitContainerName {
				existingInitContainer = &pod.Spec.Containers[i]
			}
		}
	}

	if existingInitContainer != nil {
		initContainer.Env = append(existingInitContainer.Env, initContainer.Env...)
		initContainer.EnvFrom = existingInitContainer.EnvFrom
		initContainer.Ports = existingInitContainer.Ports
		initContainer.VolumeMounts = append(existingInitContainer.VolumeMounts, initContainer.VolumeMounts...)
		initContainer.ImagePullPolicy = existingInitContainer.ImagePullPolicy

		if initContainer.SecurityContext == nil && existingInitContainer.SecurityContext != nil {
			initContainer.SecurityContext = existingInitContainer.SecurityContext
		}
	}

	return []corev1.Container{initContainer}, nil

}
