package kubernetes

import (
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/registry"
)

const OfficialDockerRegistry = "https://index.docker.io/v1/"

// GetRegistryFromImageName retrieves the registry name from an imageName
func GetRegistryFromImageName(imageName string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", err
	}

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", err
	}

	if repoInfo.Index.Official || repoInfo.Index.Name == "hub.docker.com" {
		return OfficialDockerRegistry, nil
	}

	return repoInfo.Index.Name, nil
}
