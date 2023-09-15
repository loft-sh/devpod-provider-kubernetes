package pullsecrets

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type Registry struct {
	Username string
	Password string
	Server   string
}

func RegistryFromEnv() (*Registry, error) {
	dockerUsername := os.Getenv("DOCKER_USERNAME")
	dockerPassword := os.Getenv("DOCKER_PASSWORD")
	containerRegistry := os.Getenv("CONTAINER_REGISTRY")
	if dockerUsername == "" || dockerPassword == "" {
		return nil, fmt.Errorf("DOCKER_USERNAME and/or DOCKER_PASSWORD are not set")
	}
	return &Registry{
		Username: dockerUsername,
		Password: dockerPassword,
		Server:   containerRegistry,
	}, nil

}

func (r *Registry) isAWSContainerRegistry() bool {
	return strings.Contains(r.Server, "amazonaws.com")
}

func (r *Registry) isGithubContainerRegistry() bool {
	return strings.Contains(r.Server, "ghcr.io")
}

func (r *Registry) isDockerHub() bool {
	return r.Server == "" || r.Server == "docker.io"
}

func (r *Registry) Login() {
	cmd := exec.Command(
		"docker",
		"login",
		r.Server,
		"--username", r.Username,
		"--password", r.Password,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("failed to get stdin pipe: %v", err))
	}

	go func() {
		defer stdin.Close()
		_, _ = stdin.Write([]byte(r.Password))
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to login to Docker: %v, output: %s", err, output))
	}
}

func (r *Registry) Logout() {
	cmd := exec.Command("docker", "logout", r.Server)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to logout of Docker: %v, output: %s", err, output))
	}
}

func (r *Registry) ImageName(basename string) string {
	if r.isAWSContainerRegistry() {
		return path.Join(r.Server, basename)
	}
	if r.isGithubContainerRegistry() {
		return path.Join("ghcr.io/loft-sh/devpod-provider-kubernetes/", basename)
	}
	if r.isDockerHub() {
		return path.Join(r.Username, basename)
	}

	panic(fmt.Sprintf("unsupported registry: %s", r.Server))
}
