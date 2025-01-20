package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	optionspkg "github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/driver"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DevContainerName = "devpod"
const InitContainerName = "devpod-init"

const (
	DevPodCreatedLabel      = "devpod.sh/created"
	DevPodWorkspaceLabel    = "devpod.sh/workspace"
	DevPodWorkspaceUIDLabel = "devpod.sh/workspace-uid"

	DevPodInfoAnnotation        = "devpod.sh/info"
	DevPodLastAppliedAnnotation = "devpod.sh/last-applied-configuration"
)

var ExtraDevPodLabels = map[string]string{
	DevPodCreatedLabel: "true",
}

type DevContainerInfo struct {
	WorkspaceID string
	Options     *driver.RunOptions
}

func (k *KubernetesDriver) RunDevContainer(
	ctx context.Context,
	workspaceId string,
	options *driver.RunOptions,
) error {
	workspaceId = getID(workspaceId)

	// namespace
	if k.namespace != "" && k.options.CreateNamespace == "true" {
		k.Log.Debugf("Create namespace '%s'", k.namespace)
		buf := &bytes.Buffer{}
		err := k.runCommand(ctx, []string{"create", "ns", k.namespace}, nil, buf, buf)
		if err != nil {
			k.Log.Debugf("Error creating namespace: %s%v", buf.String(), err)
		}
	}

	// check if persistent volume claim already exists
	initialize := false
	pvc, containerInfo, err := k.getDevContainerPvc(ctx, workspaceId)
	if err != nil {
		return err
	}

	if pvc == nil {
		if options == nil {
			return fmt.Errorf("No options provided and no persistent volume claim found for workspace '%s'", workspaceId)
		}

		// create persistent volume claim
		err = k.createPersistentVolumeClaim(ctx, workspaceId, options)
		if err != nil {
			return err
		}

		initialize = true
	}

	// reuse driver.RunOptions from existing workspace if none provided
	if options == nil && containerInfo != nil && containerInfo.Options != nil {
		options = containerInfo.Options
	}

	// create dev container
	err = k.runContainer(ctx, workspaceId, options, initialize)
	if err != nil {
		return err
	}

	return nil
}

func (k *KubernetesDriver) runContainer(
	ctx context.Context,
	id string,
	options *driver.RunOptions,
	initialize bool,
) (err error) {
	// get workspace mount
	mount := options.WorkspaceMount
	if mount.Target == "" {
		return fmt.Errorf("workspace mount target is empty")
	}
	if k.options.WorkspaceVolumeMount != "" {
		// Ensure workspace volume mount option is parent or same dir as workspace mount
		rel, err := filepath.Rel(k.options.WorkspaceVolumeMount, mount.Target)
		if err != nil {
			k.Log.Warn("Relative filepath: %v", err)
		} else if strings.HasPrefix(rel, "..") {
			k.Log.Warnf("Workspace volume mount needs to be the same as the workspace mount or a parent, skipping option. WorkspaceVolumeMount: %s, MountTarget: %s", k.options.WorkspaceVolumeMount, mount.Target)
		} else {
			mount.Target = k.options.WorkspaceVolumeMount
			k.Log.Debugf("Using workspace volume mount: %s", k.options.WorkspaceVolumeMount)
		}
	}

	// read pod template
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	if len(k.options.PodManifestTemplate) > 0 {
		k.Log.Debugf("trying to get pod template manifest from %s", k.options.PodManifestTemplate)
		pod, err = getPodTemplate(k.options.PodManifestTemplate)
		if err != nil {
			return err
		}
	}

	// get init containers
	initContainers, err := k.getInitContainers(options, pod, initialize)
	if err != nil {
		return errors.Wrap(err, "build init container")
	}

	// loop over volume mounts
	volumeMounts := []corev1.VolumeMount{getVolumeMount(0, mount)}
	for idx, mount := range options.Mounts {
		volumeMount := getVolumeMount(idx+1, mount)
		if mount.Type == "bind" || mount.Type == "volume" {
			volumeMounts = append(volumeMounts, volumeMount)
		} else {
			k.Log.Warnf("Unsupported mount type '%s' in mount '%s', will skip", mount.Type, mount.String())
		}
	}

	// capabilities
	var capabilities *corev1.Capabilities
	if len(options.CapAdd) > 0 {
		capabilities = &corev1.Capabilities{}
		for _, cap := range options.CapAdd {
			capabilities.Add = append(capabilities.Add, corev1.Capability(cap))
		}
	}

	// env vars
	envVars := []corev1.EnvVar{}
	for k, v := range options.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	// service account
	serviceAccount := ""
	if k.options.ServiceAccount != "" {
		serviceAccount = k.options.ServiceAccount

		// create service account
		err = k.createServiceAccount(ctx, id, serviceAccount)
		if err != nil {
			return fmt.Errorf("create service account: %w", err)
		}
	}

	// labels
	labels, err := getLabels(pod, k.options.Labels)
	if err != nil {
		return err
	}
	labels[DevPodWorkspaceUIDLabel] = options.UID

	// node selector
	nodeSelector, err := getNodeSelector(pod, k.options.NodeSelector)
	if err != nil {
		return err
	}

	// parse resources
	resources := corev1.ResourceRequirements{}
	if len(pod.Spec.Containers) > 0 {
		resources = pod.Spec.Containers[0].Resources
	}
	if k.options.Resources != "" {
		resources = parseResources(k.options.Resources, k.Log)
	}

	// ensure pull secrets
	pullSecretsCreated := false
	if k.options.KubernetesPullSecretsEnabled == "true" {
		pullSecretsCreated, err = k.EnsurePullSecret(ctx, getPullSecretsName(id), options.Image)
		if err != nil {
			return err
		}
	}

	// create the pod manifest
	pod.ObjectMeta.Name = id
	pod.ObjectMeta.Labels = labels

	pod.Spec.ServiceAccountName = serviceAccount
	pod.Spec.NodeSelector = nodeSelector
	pod.Spec.InitContainers = initContainers
	pod.Spec.Containers = getContainers(pod, options.Image, options.Entrypoint, options.Cmd, envVars, volumeMounts, capabilities, resources, options.Privileged, k.options.DangerouslyOverrideImage, k.options.StrictSecurity)
	pod.Spec.Volumes = getVolumes(pod, id)

	affinity := false
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	affinityPodID := ""

	err = k.runCommand(ctx, []string{"get", "pods", "-o=name", "-l", DevPodWorkspaceLabel + "=" + id}, nil, stdout, stderr)
	if err != nil {
		k.Log.Debugf("skipping finding cluster architecture: %s %s %w", stdout.String(), stderr.String(), err)
	}
	if stdout.String() != "" {
		affinityPodID = strings.TrimSpace(stdout.String())
		affinity = true
	}

	if affinity && k.options.NodeSelector == "" {
		k.Log.Infof("Found architecture detecting pod: %s, using PodAffinity...", affinityPodID)

		// ensure we have a pod affinity, and in that case we have, just add ours
		if pod.Spec.Affinity == nil || pod.Spec.Affinity.PodAffinity == nil {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{},
			}
		}

		// append our affinity term
		pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      DevPodWorkspaceLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{id},
						},
					},
				},
				Namespaces:  []string{k.namespace},
				TopologyKey: "kubernetes.io/hostname",
			})
	}

	if k.options.KubernetesPullSecretsEnabled == "true" && pullSecretsCreated {
		pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: getPullSecretsName(id)}}
	}

	// try to get existing pod
	existingPod, err := k.getPod(ctx, id)
	if err != nil {
		return errors.Wrapf(err, "get pod: %s", id)
	}

	if existingPod != nil {
		existingOptions := &optionspkg.Options{}
		err := json.Unmarshal([]byte(existingPod.GetAnnotations()[DevPodLastAppliedAnnotation]), existingOptions)
		if err != nil {
			k.Log.Errorf("Error unmarshalling existing provider options, continuing...: %s", err)
		}

		if optionspkg.Equal(&existingOptions.ComparableOptions, &k.options.ComparableOptions) {
			// Nothing changed, can safely return
			k.Log.Debug("Provider options did not change, skipping update")
			return nil
		}

		// Stop the current pod
		k.Log.Debug("Provider options changed")
		err = k.waitPodDeleted(ctx, id)
		if err != nil {
			return errors.Wrapf(err, "stop devcontainer: %s", id)
		}
	}

	err = k.runPod(ctx, id, pod, affinity)
	if err != nil {
		return err
	}

	return nil
}

func (k *KubernetesDriver) runPod(ctx context.Context, id string, pod *corev1.Pod, affinity bool) error {
	var err error

	// set configuration before creating the pod
	lastAppliedConfigRaw, err := json.Marshal(k.options)
	if err != nil {
		return errors.Wrap(err, "marshal last applied config")
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[DevPodLastAppliedAnnotation] = string(lastAppliedConfigRaw)

	// marshal the pod
	podRaw, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	k.Log.Debugf("Create pod with: %s", string(podRaw))
	// create the pod
	k.Log.Infof("Create Pod '%s'", id)
	buf := &bytes.Buffer{}
	err = k.runCommand(ctx, []string{"create", "-f", "-"}, strings.NewReader(string(podRaw)), buf, buf)
	if err != nil {
		return errors.Wrapf(err, "create pod: %s", buf.String())
	}

	// wait for pod running
	k.Log.Infof("Waiting for DevContainer Pod '%s' to come up...", id)
	_, err = k.waitPodRunning(ctx, id)
	if err != nil {
		return err
	}

	if affinity {
		k.Log.Infof("Cleaning up architecture detection pod")
		err := k.runCommand(ctx, []string{"delete", "pods", "--force", "-l", DevPodWorkspaceLabel + "=" + id}, nil, buf, buf)
		if err != nil {
			return errors.Wrapf(err, "cleanup jobs: %s", buf.String())
		}
	}

	return nil
}

func getContainers(
	pod *corev1.Pod,
	imageName,
	entrypoint string,
	args []string,
	envVars []corev1.EnvVar,
	volumeMounts []corev1.VolumeMount,
	capabilities *corev1.Capabilities,
	resources corev1.ResourceRequirements,
	privileged *bool,
	overrideImage string,
	strictSecurity bool,
) []corev1.Container {
	devPodContainer := corev1.Container{
		Name:         DevContainerName,
		Image:        imageName,
		Command:      []string{entrypoint},
		Args:         args,
		Env:          envVars,
		Resources:    resources,
		VolumeMounts: volumeMounts,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: capabilities,
			Privileged:   privileged,
			RunAsUser:    &[]int64{0}[0],
			RunAsGroup:   &[]int64{0}[0],
			RunAsNonRoot: &[]bool{false}[0],
		},
	}

	if overrideImage != "" {
		devPodContainer.Image = overrideImage
	}

	if strictSecurity {
		devPodContainer.SecurityContext = nil
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

		if devPodContainer.SecurityContext == nil && existingDevPodContainer.SecurityContext != nil {
			devPodContainer.SecurityContext = existingDevPodContainer.SecurityContext
		}
	}
	retContainers = append(retContainers, devPodContainer)

	return retContainers
}

func getVolumes(pod *corev1.Pod, id string) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "devpod",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: id,
				},
			},
		},
	}

	if pod.Spec.Volumes != nil {
		volumes = append(volumes, pod.Spec.Volumes...)
	}

	return volumes
}

func getVolumeMount(idx int, mount *config.Mount) corev1.VolumeMount {
	subPath := strconv.Itoa(idx)
	if mount.Type == "volume" && mount.Source != "" {
		subPath = strings.TrimPrefix(mount.Source, "/")
	}

	return corev1.VolumeMount{
		Name:      "devpod",
		MountPath: mount.Target,
		SubPath:   fmt.Sprintf("devpod/%s", subPath),
	}
}

func getLabels(pod *corev1.Pod, rawLabels string) (map[string]string, error) {
	labels := map[string]string{}
	if pod.ObjectMeta.Labels != nil {
		for k, v := range pod.ObjectMeta.Labels {
			labels[k] = v
		}
	}
	if rawLabels != "" {
		extraLabels, err := parseLabels(rawLabels)
		if err != nil {
			return nil, fmt.Errorf("parse labels: %w", err)
		}
		for k, v := range extraLabels {
			labels[k] = v
		}
	}
	// make sure we don't overwrite the devpod labels
	for k, v := range ExtraDevPodLabels {
		labels[k] = v
	}

	return labels, nil
}

func getNodeSelector(pod *corev1.Pod, rawNodeSelector string) (map[string]string, error) {
	nodeSelector := map[string]string{}
	if pod.Spec.NodeSelector != nil {
		for k, v := range pod.Spec.NodeSelector {
			nodeSelector[k] = v
		}
	}

	if rawNodeSelector != "" {
		selector, err := parseLabels(rawNodeSelector)
		if err != nil {
			return nil, fmt.Errorf("parsing node selector: %w", err)
		}
		for k, v := range selector {
			nodeSelector[k] = v
		}
	}

	return nodeSelector, nil
}

func (k *KubernetesDriver) StartDevContainer(ctx context.Context, workspaceId string) error {
	workspaceId = getID(workspaceId)
	_, containerInfo, err := k.getDevContainerPvc(ctx, workspaceId)
	if err != nil {
		return err
	} else if containerInfo == nil {
		return fmt.Errorf("persistent volume '%s' not found", workspaceId)
	}

	return k.runContainer(
		ctx,
		workspaceId,
		containerInfo.Options,
		false,
	)
}

func getID(workspaceID string) string {
	return "devpod-" + workspaceID
}

func getPullSecretsName(workspaceID string) string {
	return fmt.Sprintf("devpod-pull-secret-%s", workspaceID)
}
