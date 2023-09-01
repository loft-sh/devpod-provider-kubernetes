package options

import (
	"fmt"
	"os"
)

type Options struct {
	DevContainerID string

	DiskSize string

	KubernetesContext   string
	KubernetesConfig    string
	KubernetesNamespace string

	CreateNamespace string
	ClusterRole     string
	ServiceAccount  string

	HelperImage     string
	HelperResources string

	KubectlPath       string
	InactivityTimeout string
	StorageClass      string

	PvcAccessMode string
	NodeSelector  string
	Resources     string

	PodManifestTemplate string
	Labels              string
}

func FromEnv() (*Options, error) {
	retOptions := &Options{}

	var err error

	retOptions.DevContainerID, err = fromEnvOrError("DEVCONTAINER_ID")
	if err != nil {
		return nil, err
	}

	retOptions.DiskSize = os.Getenv("DISK_SIZE")
	retOptions.KubernetesContext = os.Getenv("KUBERNETES_CONTEXT")
	retOptions.KubernetesConfig = os.Getenv("KUBERNETES_CONFIG")
	retOptions.KubernetesNamespace = os.Getenv("KUBERNETES_NAMESPACE")
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

	return retOptions, nil
}

func fromEnvOrError(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("couldn't find option %s in environment, please make sure %s is defined", name, name)
	}

	return val, nil
}
