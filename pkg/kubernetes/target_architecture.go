package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/loft-sh/devpod/pkg/encoding"
	"github.com/loft-sh/devpod/pkg/random"
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

	// get target architnecture
	k.Log.Infof("Find out cluster architecture...")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	podName := encoding.SafeConcatNameMax([]string{"devpod", workspaceId, random.String(6)}, 32)
	err := k.runCommand(ctx, []string{
		"run", podName,
		"-n", k.namespace,
		"-q", "--restart=Never",
		"--image", k.helperImage(),
		"--labels", DevPodWorkspaceLabel + workspaceId,
		"--", "sh", "-c", "uname -m && tail -f /dev/null"}, os.Stdin, stdout, stderr)
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

	return "busybox"
}
