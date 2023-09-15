package pullsecrets

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
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
	var driver *kubernetes.KubernetesDriver
	const DockerfileDirectory = "pullsecrets/testdata/"

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

	prepareK8sDriver := func() *kubernetes.KubernetesDriver {
		options := options2.Options{
			KubernetesNamespace: namespace,
		}
		driver := kubernetes.NewKubernetesDriver(
			&options, log.Default.ErrorStreamOnly()).(*kubernetes.KubernetesDriver)
		return driver
	}

	BeforeEach(func() {
		setUpK8sClient()
		createEphemeralNamespace()
		driver = prepareK8sDriver()
	})

	AfterEach(func() {
		deleteNamespace()
	})

	// NOTE: It was tested with Docker Hub and AWS ECR, make sure image is private
	It("should create pull secret and make pod use it", func() {
		By("Login to private container registry")

		registry, err := RegistryFromEnv()
		if err != nil {
			Skip(err.Error())
		}

		pullSecretName := "test-pull-secret"
		imageName := registry.ImageName("private-test-image")

		registry.Login()
		defer registry.Logout()
		dockerBuild(imageName, DockerfileDirectory)
		registry.Push(imageName)

		By("Create pull secret")

		created, err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())

		By("Create pod with the image from the private registry")
		createPod(client, namespace, imageName, pullSecretName)
	})

	It("should delete created pull secret if called DeletePullSecret()", func() {
		registry, err := RegistryFromEnv()
		if err != nil {
			Skip(err.Error())
		}

		pullSecretName := "test-pull-secret"
		imageName := registry.ImageName("private-test-image")

		registry.Login()
		defer registry.Logout()

		created, err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())

		_, err = client.CoreV1().Secrets(namespace).Get(context.TODO(), pullSecretName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = driver.DeletePullSecret(context.TODO(), pullSecretName)
		Expect(err).NotTo(HaveOccurred())

		_, err = client.CoreV1().Secrets(namespace).Get(context.TODO(), pullSecretName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
	})

	It("shouldn't recreate pull secret if it exists and haven't changed", func() {
		registry, err := RegistryFromEnv()
		if err != nil {
			Skip(err.Error())
		}

		pullSecretName := "test-pull-secret"
		imageName := registry.ImageName("private-test-image")

		registry.Login()
		defer registry.Logout()

		dockerBuild(imageName, DockerfileDirectory)
		registry.Push(imageName)

		created, err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())

		_, err = client.CoreV1().Secrets(namespace).Get(context.TODO(), pullSecretName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		created, err = driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())
	})

	// NOTE: make sure the image is public
	It("should work with public images without pull secret", func() {
		registry, err := RegistryFromEnv()
		if err != nil {
			Skip(err.Error())
		}

		if registry.isAWSContainerRegistry() {
			Skip("This test doesn't support AWS ECR public images")
		}

		pullSecretName := "test-pull-secret"
		imageName := registry.ImageName("public-test-image")

		registry.Login()
		dockerBuild(imageName, DockerfileDirectory)
		registry.Push(imageName)

		// we should be able to push image without pull secret
		registry.Logout()

		// there shouldn't be any error, but the pull secret shouldn't be created
		created, err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())

		_, err = client.CoreV1().Secrets(namespace).Get(context.TODO(), pullSecretName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())

		// create pod without pull secret
		createPod(client, namespace, imageName)
	})

	It("should be able to read pull secret", func() {
		registry, err := RegistryFromEnv()
		if err != nil {
			Skip(err.Error())
		}
		pullSecretName := "test-pull-secret"
		imageName := registry.ImageName("public-test-image")
		registryName, err := kubernetes.GetRegistryFromImageName(imageName)
		if err != nil {
			panic(err)
		}

		registry.Login()
		defer registry.Logout()
		created, err := driver.EnsurePullSecret(context.TODO(), pullSecretName, imageName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())

		authToken, err := driver.ReadSecretContents(context.TODO(), pullSecretName, registryName)
		Expect(err).NotTo(HaveOccurred())
		Expect(authToken).To(SatisfyAny(
			BeEquivalentTo(registry.Password),
			BeEquivalentTo(fmt.Sprintf("%s:%s", registry.Username, registry.Password)),
		))
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

func createPod(client *k8s.Clientset, namespace, image string, pullSecretName ...string) {
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
		},
	}

	if len(pullSecretName) > 0 {
		pod.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			{
				Name: pullSecretName[0],
			},
		}
	}

	_, err := client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() v1.PodPhase {
		pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return pod.Status.Phase
	}, time.Minute*1, time.Second*5).Should(Or(Equal(v1.PodRunning), Equal(v1.PodSucceeded)))
}

func dockerBuild(image, dockerfileDirectory string) {
	cmd := exec.Command("docker", "build", "-t", image, dockerfileDirectory)
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("failed to build image: %v", err))
	}
}
