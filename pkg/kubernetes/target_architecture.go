package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/loft-sh/devpod/pkg/encoding"
	"github.com/loft-sh/devpod/pkg/random"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *KubernetesDriver) TargetArchitecture(ctx context.Context, workspaceId string) (string, error) {
	workspaceId = getID(workspaceId)

	// namespace
	if k.namespace != "" && k.options.CreateNamespace == "true" {
		k.Log.Debugf("Create namespace '%s'", k.namespace)
		buf := &bytes.Buffer{}
		err := k.runCommand(ctx, []string{"create", "ns", k.namespace}, nil, buf, buf)
		if err != nil {
			k.Log.Debugf("Error creating namespace: %v", err)
		}
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
	// parse pod manifest template if provided
	if len(k.options.ArchDetectionPodManifestTemplate) > 0 {
		p, err := getPodTemplate(k.options.ArchDetectionPodManifestTemplate)
		if err != nil {
			return "", err
		}
		pod = p
	}
	podName := encoding.SafeConcatNameMax([]string{"devpod", workspaceId, random.String(6)}, 32)
	pod.Namespace = k.namespace
	pod.Name = podName

	// configure labels
	labels := map[string]string{}
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	for k, label := range pod.Labels {
		labels[k] = label
	}
	labels[DevPodWorkspaceLabel] = workspaceId

	pod.Labels = labels
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.Containers = getArchitectureDetectionPodContainers(pod, k.helperImage(), []string{"sh", "-c", "uname -m && tail -f /dev/null"})

	podRaw, err := json.Marshal(pod)
	if err != nil {
		return "", err
	}

	// get target architecture
	k.Log.Infof("Find out cluster architecture...")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = k.runCommand(ctx, []string{"create", "-f", "-"}, strings.NewReader(string(podRaw)), stdout, stderr)
	if err != nil {
		return "", fmt.Errorf("find out cluster architecture: %s %s %w", stdout.String(), stderr.String(), err)
	}

	// wait for pod running
	k.Log.Infof("Waiting for cluster architecture job to come up...")
	_, err = k.waitPodRunning(ctx, podName)
	if err != nil {
		return "", fmt.Errorf("find out cluster architecture: %s %s %w", stdout.String(), stderr.String(), err)
	}

	// capture uname output
	err = k.runCommand(ctx, []string{"logs", podName, "-n", k.namespace}, os.Stdin, stdout, stderr)
	if err != nil {
		return "", fmt.Errorf("find out cluster architecture: %s %s %w", stdout.String(), stderr.String(), err)
	}

	unameOutput := stdout.String()
	if strings.Contains(unameOutput, "arm") || strings.Contains(unameOutput, "aarch") {
		return "arm64", nil
	}

	return "amd64", nil
}

func (k *KubernetesDriver) helperImage() string {
	if k.options.HelperImage != "" {
		return k.options.HelperImage
	}

	return "busybox:latest"
}

func getArchitectureDetectionPodContainers(
	pod *corev1.Pod,
	imageName string,
	args []string,
) []corev1.Container {
	devPodContainer := corev1.Container{
		Name:  DevContainerName,
		Image: imageName,
		Args:  args,
	}

	// merge with existing container if it exists
	var existingDevPodContainer *corev1.Container
	retContainers := []corev1.Container{}
	if pod != nil {
		for i, container := range pod.Spec.Containers {
			if container.Name == DevContainerName {
				existingDevPodContainer = &pod.Spec.Containers[i]
			} else {
				retContainers = append(retContainers, container)
			}
		}
	}

	if existingDevPodContainer != nil {
		devPodContainer.Env = append(existingDevPodContainer.Env, devPodContainer.Env...)
		devPodContainer.EnvFrom = existingDevPodContainer.EnvFrom
		devPodContainer.Ports = existingDevPodContainer.Ports
		devPodContainer.VolumeMounts = append(existingDevPodContainer.VolumeMounts, devPodContainer.VolumeMounts...)
		devPodContainer.ImagePullPolicy = existingDevPodContainer.ImagePullPolicy
		devPodContainer.Resources = existingDevPodContainer.Resources

		if devPodContainer.SecurityContext == nil && existingDevPodContainer.SecurityContext != nil {
			devPodContainer.SecurityContext = existingDevPodContainer.SecurityContext
		}
	}
	retContainers = append(retContainers, devPodContainer)

	return retContainers
}
