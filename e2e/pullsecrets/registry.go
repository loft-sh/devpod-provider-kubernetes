package pullsecrets

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type ContainerRegistry interface {
	Login()
	Logout()

	PublicImageName() string
	PrivateImageName() string

	Username() string
	Password() string
	Server() string
}

type Registry struct {
	username string
	password string
	server   string
}

func (r *Registry) Password() string {
	return r.password
}

func (r *Registry) Username() string {
	return r.username
}

func (r *Registry) Server() string {
	return r.server
}

func (r *Registry) Login() {
	cmd := exec.Command(
		"docker",
		"login",
		r.server,
		"--username", r.username,
		"--password", r.password,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("failed to get stdin pipe: %v", err))
	}

	go func() {
		defer stdin.Close()
		_, _ = stdin.Write([]byte(r.password))
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to login to Docker: %v, output: %s", err, output))
	}
}

func (r *Registry) Logout() {
	cmd := exec.Command("docker", "logout", r.server)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to logout of Docker: %v, output: %s", err, output))
	}
}

type AWSRegistry struct{ Registry }

func (r *AWSRegistry) imageName(basename string) string {
	return path.Join(r.Server(), basename)
}

func (r *AWSRegistry) PrivateImageName() string {
	return r.imageName("private-test-image")
}

func (r *AWSRegistry) PublicImageName() string {
	return r.imageName("public-test-image")
}

type GithubRegistry struct{ Registry }

func (r *GithubRegistry) imageName(basename string) string {
	return path.Join("ghcr.io/loft-sh/devpod-provider-kubernetes/", basename)
}

func (r *GithubRegistry) PrivateImageName() string {
	return r.imageName("private-test-image")
}

func (r *GithubRegistry) PublicImageName() string {
	return r.imageName("public-test-image")
}

type DockerHubRegistry struct{ Registry }

func (r *DockerHubRegistry) imageName(basename string) string {
	return path.Join(r.Username(), basename)
}

func (r *DockerHubRegistry) PrivateImageName() string {
	return r.imageName("private-test-image")
}

func (r *DockerHubRegistry) PublicImageName() string {
	return r.imageName("public-test-image")
}

func RegistryFromEnv() (ContainerRegistry, error) {
	dockerUsername := os.Getenv("DOCKER_USERNAME")
	dockerPassword := os.Getenv("DOCKER_PASSWORD")
	containerRegistry := os.Getenv("CONTAINER_REGISTRY")

	if dockerUsername == "" || dockerPassword == "" {
		return nil, fmt.Errorf("DOCKER_USERNAME and/or DOCKER_PASSWORD are not set")
	}

	reg := &Registry{
		username: dockerUsername,
		password: dockerPassword,
		server:   containerRegistry,
	}

	if strings.Contains(containerRegistry, "amazonaws.com") {
		return &AWSRegistry{*reg}, nil
	}
	if strings.Contains(containerRegistry, "ghcr.io") {
		return &GithubRegistry{*reg}, nil
	}
	if containerRegistry == "" || containerRegistry == "docker.io" {
		return &DockerHubRegistry{*reg}, nil
	}

	return nil, fmt.Errorf("unsupported registry: %s", containerRegistry)
}
