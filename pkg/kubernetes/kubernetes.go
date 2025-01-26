package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/devpod/pkg/command"
	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/driver"
	"github.com/loft-sh/log"

	perrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

func NewKubernetesDriver(options *options.Options, log log.Logger) driver.Driver {
	kubectl := "kubectl"
	if options.KubectlPath != "" {
		kubectl = options.KubectlPath
	}

	if options.KubernetesNamespace != "" {
		log.Debugf("Use Kubernetes Namespace '%s'", options.KubernetesNamespace)
	}
	if options.KubernetesConfig != "" {
		log.Debugf("Use Kubernetes Config '%s'", options.KubernetesConfig)
	}
	if options.KubernetesContext != "" {
		log.Debugf("Use Kubernetes Context '%s'", options.KubernetesContext)
	}
	return &KubernetesDriver{
		kubectl: kubectl,

		kubeConfig: options.KubernetesConfig,
		context:    options.KubernetesContext,
		namespace:  options.KubernetesNamespace,

		options: options,
		Log:     log,
	}
}

type KubernetesDriver struct {
	kubectl string

	kubeConfig string
	namespace  string
	context    string

	dryRun *dryRunConfig
	output string

	options *options.Options
	Log     log.Logger
}

func (k *KubernetesDriver) FindDevContainer(ctx context.Context, workspaceId string) (*config.ContainerDetails, error) {
	workspaceId = getID(workspaceId)

	pvc, containerInfo, err := k.getDevContainerPvc(ctx, workspaceId)
	if err != nil {
		return nil, err
	}

	return k.infoFromObject(ctx, pvc, containerInfo)
}

func (k *KubernetesDriver) getDevContainerPvc(ctx context.Context, id string) (*corev1.PersistentVolumeClaim, *DevContainerInfo, error) {
	// try to find pvc
	out, err := k.buildCmd(ctx, []string{"get", "pvc", id, "--ignore-not-found", "-o", "json"}).Output()
	if err != nil {
		return nil, nil, command.WrapCommandError(out, err)
	} else if len(out) == 0 {
		return nil, nil, nil
	}

	// try to unmarshal pvc
	pvc := &corev1.PersistentVolumeClaim{}
	err = json.Unmarshal(out, pvc)
	if err != nil {
		return nil, nil, perrors.Wrap(err, "unmarshal pvc")
	} else if pvc.Annotations == nil || pvc.Annotations[DevPodInfoAnnotation] == "" {
		return nil, nil, fmt.Errorf("pvc is missing dev container info annotation")
	}

	// get container info
	containerInfo := &DevContainerInfo{}
	err = json.Unmarshal([]byte(pvc.GetAnnotations()[DevPodInfoAnnotation]), containerInfo)
	if err != nil {
		return nil, nil, perrors.Wrap(err, "decode dev container info")
	}

	return pvc, containerInfo, nil
}

func (k *KubernetesDriver) infoFromObject(ctx context.Context, pvc *corev1.PersistentVolumeClaim, containerInfo *DevContainerInfo) (*config.ContainerDetails, error) {
	if pvc == nil {
		return nil, nil
	}

	// check pod
	pod, err := k.waitPodRunning(ctx, pvc.Name)
	if err != nil {
		k.Log.Infof("Error finding pod: %v", err)
		k.Log.Warn("If the pod does not come up automatically it is stuck in an error state. Recreate the workspace to recover from this")
		pod = nil
	}

	// determine status
	status := "exited"
	if pod != nil {
		status = "running"
	}

	// check started
	startedAt := pvc.CreationTimestamp.String()
	if pod != nil {
		startedAt = pod.CreationTimestamp.String()
	}

	return &config.ContainerDetails{
		ID:      pvc.Name,
		Created: pvc.CreationTimestamp.String(),
		State: config.ContainerDetailsState{
			Status:    status,
			StartedAt: startedAt,
		},
		Config: config.ContainerDetailsConfig{
			Labels: config.ListToObject(containerInfo.Options.Labels),
		},
	}, nil
}

func (k *KubernetesDriver) StopDevContainer(ctx context.Context, workspaceId string) error {
	workspaceId = getID(workspaceId)

	// delete pod
	out, err := k.buildCmd(ctx, []string{"delete", "po", workspaceId, "--ignore-not-found"}).CombinedOutput()
	if err != nil {
		return perrors.Wrapf(err, "delete pod: %s", string(out))
	}

	return nil
}

func (k *KubernetesDriver) DeleteDevContainer(ctx context.Context, workspaceId string) error {
	workspaceId = getID(workspaceId)

	// delete pod
	k.Log.Infof("Delete pod '%s'...", workspaceId)
	err := k.deletePod(ctx, workspaceId)
	if err != nil {
		return err
	}

	// delete pvc
	k.Log.Infof("Delete persistent volume claim '%s'...", workspaceId)
	out, err := k.buildCmd(ctx, []string{"delete", "pvc", workspaceId, "--ignore-not-found", "--grace-period=5"}).CombinedOutput()
	if err != nil {
		return perrors.Wrapf(err, "delete pvc: %s", string(out))
	}

	// delete role binding & service account
	if k.options.ClusterRole != "" {
		k.Log.Infof("Delete role binding '%s'...", workspaceId)
		out, err := k.buildCmd(ctx, []string{"delete", "rolebinding", workspaceId, "--ignore-not-found"}).CombinedOutput()
		if err != nil {
			return perrors.Wrapf(err, "delete role binding: %s", string(out))
		}
	}

	// delete pull secret
	if k.options.KubernetesPullSecretsEnabled != "" {
		k.Log.Infof("Delete pull secret '%s'...", workspaceId)
		err := k.DeletePullSecret(ctx, getPullSecretsName(workspaceId))
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *KubernetesDriver) deletePod(ctx context.Context, podName string) error {
	out, err := k.buildCmd(ctx, []string{"delete", "po", podName, "--ignore-not-found", "--grace-period=10"}).CombinedOutput()
	if err != nil {
		return perrors.Wrapf(err, "delete pod: %s", string(out))
	}

	return nil
}

func (k *KubernetesDriver) CommandDevContainer(ctx context.Context, workspaceId, user, command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	workspaceId = getID(workspaceId)

	args := []string{"exec", "-c", "devpod"}
	if stdin != nil {
		args = append(args, "-i")
	}
	args = append(args, workspaceId)
	if user != "" && user != "root" {
		args = append(args, "--", "su", user, "-c", command)
	} else {
		args = append(args, "--", "sh", "-c", command)
	}

	return k.runCommand(ctx, args, stdin, stdout, stderr)
}

func (k *KubernetesDriver) GetDevContainerLogs(ctx context.Context, workspaceID string, stdout io.Writer, stderr io.Writer) error {
	workspaceID = getID(workspaceID)

	args := []string{"logs", "pods/" + workspaceID, "-c", "devpod"}

	return k.runCommand(ctx, args, nil, stdout, stderr)
}

func (k *KubernetesDriver) RenderTemplate(ctx context.Context, workspaceID string, verbose bool) error {
	k.dryRun = NewDryRunConfig(DryRunClient)
	k.output = "yaml"

	if verbose {
		providerOptionsMsg := "Rendering template with provider options:\n\n"
		providerOptionsMsg += fmt.Sprintf("Namespace: %s\n", k.namespace)
		providerOptionsMsg += fmt.Sprintf("Context: %s\n", k.context)
		providerOptionsMsg += fmt.Sprintf("Kubectl: %s\n", k.kubectl)
		providerOptionsMsg += fmt.Sprintf("KubeConfig: %s\n", k.kubeConfig)
		providerOptionsMsg += "\n"
		if k.options != nil {
			providerOptionsMsg += k.options.Display()
		}
		k.Log.Info(strings.TrimSpace(providerOptionsMsg) + "\n")
	}

	// TODO: This could potentially be done through main DevPod as well
	// for more realistic results
	privileged := false
	fakeRunOptions := driver.RunOptions{
		UID:         "FAKE-UID",
		User:        "FAKE-USER",
		Image:       "devpod-sh:fake",
		Entrypoint:  "entrypoint",
		Cmd:         []string{"cmd"},
		Env:         map[string]string{},
		CapAdd:      []string{},
		SecurityOpt: []string{},
		Labels:      []string{},
		Privileged:  &privileged,
		WorkspaceMount: &config.Mount{
			Target: "/workspaces/FAKE",
			Type:   "volume",
			Source: "FAKE",
		},
		Mounts: []*config.Mount{},
	}

	// We want to ignore all of the logs aside from out own
	logger := log.Default.ErrorStreamOnly()
	bufferLogger := NewBufferLogger(logger)
	k.Log = bufferLogger
	err := k.RunDevContainer(ctx, workspaceID, &fakeRunOptions)
	if err != nil && verbose {
		logger.Warnf("Encountered an error, manifests might not be complete: %v", err)
		bufferLogger.Flush()
	}
	out := strings.Builder{}
	for i, v := range k.dryRun.manifests {
		if i != 0 {
			out.WriteString("---")
		}
		out.WriteString(strings.TrimSpace(v))
	}
	logger.Info(out.String())

	return err // still return error to signal to consuming process that we encountered an error during rendering
}

func (k *KubernetesDriver) buildCmd(ctx context.Context, args []string) *exec.Cmd {
	newArgs := []string{}
	if k.namespace != "" {
		newArgs = append(newArgs, "--namespace", k.namespace)
	}
	if k.kubeConfig != "" {
		newArgs = append(newArgs, "--kubeconfig", k.kubeConfig)
	}
	if k.context != "" {
		newArgs = append(newArgs, "--context", k.context)
	}
	if k.dryRun != nil {
		newArgs = append(newArgs, fmt.Sprintf("%s=%s", "--dry-run", k.dryRun.strategy))
	}
	if k.output != "" {
		newArgs = append(newArgs, fmt.Sprintf("%s=%s", "--output", k.output))
	}

	newArgs = append(newArgs, args...)
	k.Log.Debugf("Run command: %s %s", k.kubectl, strings.Join(newArgs, " "))
	return exec.CommandContext(ctx, k.kubectl, newArgs...)
}

func (k *KubernetesDriver) runCommand(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return k.runCommandWithDir(ctx, "", args, stdin, stdout, stderr)
}

func (k *KubernetesDriver) runCommandWithDir(ctx context.Context, dir string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := k.buildCmd(ctx, args)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (k *KubernetesDriver) isDryRun() bool {
	return k.dryRun != nil
}
