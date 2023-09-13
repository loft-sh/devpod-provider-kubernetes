package pullsecrets

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/kubernetes"
	options2 "github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("Pull secrets", func() {
	var namespace string
	var client *k8s.Clientset

	createEphemeralNamespace := func() {
		namespace = fmt.Sprintf("test-ns-%d", rand.Int())
		_, err := client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	}

	deleteNamespace := func() {
		err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}

	setUpK8sClient := func() {
		kubeConfig := getKubeConfigPath()

		config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		Expect(err).NotTo(HaveOccurred())

		client, err = k8s.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		setUpK8sClient()
		createEphemeralNamespace()
	})

	AfterEach(func() {
		deleteNamespace()
	})

	// NOTE: It was tested with Docker Hub and AWS ECR
	It("should create pull secret and make pod use it", func() {
		By("Login to private container registry")

		pullSecretName := "test-pull-secret"

		dockerUsername := os.Getenv("DOCKER_USERNAME")
		dockerPassword := os.Getenv("DOCKER_PASSWORD")
		containerRegistry := os.Getenv("CONTAINER_REGISTRY")
		if dockerUsername == "" || dockerPassword == "" {
			Skip("DOCKER_USERNAME and/or DOCKER_PASSWORD are not set")
		}

		imageName := imageName(dockerUsername, containerRegistry)

		dockerLogin(dockerUsername, dockerPassword, containerRegistry)
		dockerBuild(imageName, "pullsecrets/")
		dockerPush(imageName)

		By("Create pull secret")

		options := options2.Options{
			KubernetesNamespace: namespace,
		}
		driver := kubernetes.NewKubernetesDriver(
			&options, log.Default.ErrorStreamOnly()).(*kubernetes.KubernetesDriver)

		err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())

		By("Create pod with the image from the private registry")
		createPod(namespace, imageName, pullSecretName, client)
	})
})

func getKubeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	kubeConfigPath := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		panic(fmt.Sprintf("kubeconfig file does not exist at path: %s", kubeConfigPath))
	}
	return kubeConfigPath
}

func createPod(namespace, image, pullSecretName string, client *k8s.Clientset) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "test-container",
					Image:           image,
					ImagePullPolicy: v1.PullAlways,
				},
			},
			ImagePullSecrets: []v1.LocalObjectReference{
				{
					Name: pullSecretName,
				},
			},
		},
	}

	_, err := client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() v1.PodPhase {
		pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return pod.Status.Phase
	}, time.Minute*1, time.Second*5).Should(Or(Equal(v1.PodRunning), Equal(v1.PodSucceeded)))
}

func imageName(dockerUsername, containerRegistry string) string {
	if containerRegistry != "" {
		return path.Join(containerRegistry, "test-image")
	}
	return path.Join(dockerUsername, "test-image")

}

func dockerBuild(image, dockerfileDirectory string) {
	cmd := exec.Command("docker", "build", "-t", image, dockerfileDirectory)
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to build image: %v", err))
	}
}

func dockerPush(image string) {
	cmd := exec.Command("docker", "push", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to push image: %v, output: %s", err, output))
	}
}

func dockerLogin(username, password, server string) {
	cmd := exec.Command(
		"docker",
		"login",
		server,
		"--username", username,
		"--password", password,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("failed to get stdin pipe: %v", err))
	}

	go func() {
		defer stdin.Close()
		_, _ = stdin.Write([]byte(password))
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to login to Docker: %v, output: %s", err, output))
	}
}
