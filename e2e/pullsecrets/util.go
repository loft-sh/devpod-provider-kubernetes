package pullsecrets

import (
	"fmt"
	"os"
	"os/exec"
	"path"
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

func (r *Registry) Push(image string) {
	cmd := exec.Command("docker", "push", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to push image: %v, output: %s", err, output))
	}
}

func (r *Registry) ImageName(basename string) string {
	if r.Server != "" {
		return path.Join(r.Server, basename)
	}
	return path.Join(r.Username, basename)
}
