package options

import (
	"fmt"
	"os"
	"reflect"
)

type Options struct {
	ComparableOptions

	KubernetesContext   string `json:"-"`
	KubernetesConfig    string `json:"-"`
	KubernetesNamespace string `json:"-"`
	KubectlPath         string `json:"-"`
	PodTimeout          string `json:"-"`
}

type ComparableOptions struct {
	DevContainerID     string `json:"devcontainerId,omitempty"`
	TargetArchitecture string `json:"targetArchitecture,omitempty"`

	KubernetesPullSecretsEnabled string `json:"kubernetesPullSecretsEnabled,omitempty"`
	CreateNamespace              string `json:"createNamespace,omitempty"`
	ClusterRole                  string `json:"clusterRole,omitempty"`
	ServiceAccount               string `json:"serviceAccount,omitempty"`

	HelperImage       string `json:"helperImage,omitempty"`
	HelperResources   string `json:"helperResources,omitempty"`
	InactivityTimeout string `json:"inactivityTimeout,omitempty"`
	StorageClass      string `json:"storageClass,omitempty"`

	DiskSize             string `json:"diskSize,omitempty"`
	PvcAccessMode        string `json:"pvcAccessMode,omitempty"`
	PvcAnnotations       string `json:"pvcAnnotations,omitempty"`
	NodeSelector         string `json:"nodeSelector,omitempty"`
	Resources            string `json:"resources,omitempty"`
	WorkspaceVolumeMount string `json:"workspaceVolumeMount,omitempty"`

	PodManifestTemplate              string `json:"podManifestTemplate,omitempty"`
	ArchDetectionPodManifestTemplate string `json:"archDetectionPodManifestTemplate,omitempty"`
	Labels                           string `json:"labels,omitempty"`

	DangerouslyOverrideImage string `json:"dangerouslyOverrideImage,omitempty"`
	StrictSecurity           bool   `json:"strictSecurity,omitempty"`
}

func FromEnv() (*Options, error) {
	retOptions := &Options{}

	var err error

	retOptions.DevContainerID, err = fromEnvOrError("DEVCONTAINER_ID")
	if err != nil {
		return nil, err
	}

	retOptions.TargetArchitecture = os.Getenv("TARGET_ARCHITECTURE")

	retOptions.DiskSize = os.Getenv("DISK_SIZE")
	retOptions.KubernetesContext = os.Getenv("KUBERNETES_CONTEXT")
	retOptions.KubernetesConfig = os.Getenv("KUBERNETES_CONFIG")
	retOptions.KubernetesNamespace = os.Getenv("KUBERNETES_NAMESPACE")
	retOptions.KubernetesPullSecretsEnabled = os.Getenv("KUBERNETES_PULL_SECRETS_ENABLED")
	retOptions.CreateNamespace = os.Getenv("CREATE_NAMESPACE")
	retOptions.ClusterRole = os.Getenv("CLUSTER_ROLE")
	retOptions.ServiceAccount = os.Getenv("SERVICE_ACCOUNT")
	retOptions.HelperImage = os.Getenv("HELPER_IMAGE")
	retOptions.HelperResources = os.Getenv("HELPER_RESOURCES")
	retOptions.KubectlPath = os.Getenv("KUBECTL_PATH")
	retOptions.InactivityTimeout = os.Getenv("INACTIVITY_TIMEOUT")
	retOptions.StorageClass = os.Getenv("STORAGE_CLASS")
	retOptions.PvcAccessMode = os.Getenv("PVC_ACCESS_MODE")
	retOptions.NodeSelector = os.Getenv("NODE_SELECTOR")
	retOptions.Resources = os.Getenv("RESOURCES")
	retOptions.PodManifestTemplate = os.Getenv("POD_MANIFEST_TEMPLATE")
	retOptions.Labels = os.Getenv("LABELS")
	retOptions.PodTimeout = os.Getenv("POD_TIMEOUT")
	retOptions.DangerouslyOverrideImage = os.Getenv("DANGEROUSLY_OVERRIDE_IMAGE")
	retOptions.StrictSecurity = os.Getenv("STRICT_SECURITY") == "true"
	retOptions.ArchDetectionPodManifestTemplate = os.Getenv("ARCH_DETECTION_POD_MANIFEST_TEMPLATE")
	retOptions.WorkspaceVolumeMount = os.Getenv("WORKSPACE_VOLUME_MOUNT")
	retOptions.PvcAnnotations = os.Getenv("PVC_ANNOTATIONS")

	return retOptions, nil
}

func Equal(a *ComparableOptions, b *ComparableOptions) bool {
	return reflect.DeepEqual(a, b)
}

func fromEnvOrError(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("couldn't find option %s in environment, please make sure %s is defined", name, name)
	}

	return val, nil
}
