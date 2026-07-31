package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	providerkubernetes "github.com/loft-sh/devpod-provider-kubernetes/pkg/kubernetes"
	provideroptions "github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/driver"
	"github.com/loft-sh/log"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Register tests
	_ "github.com/loft-sh/devpod-provider-kubernetes/e2e/pullsecrets"
)

const cloneMarker = "devpod-pvc-clone-marker"

var _ = ginkgo.Describe("PVC data source", func() {
	ginkgo.It("clones the source PVC contents", func() {
		if os.Getenv("PVC_CLONE_E2E") != "1" {
			ginkgo.Skip("set PVC_CLONE_E2E=1 to run against a clone-capable CSI driver")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		kubeConfig := os.Getenv("KUBECONFIG")
		if kubeConfig == "" {
			home, err := os.UserHomeDir()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			kubeConfig = filepath.Join(home, ".kube", "config")
		}
		clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		client, err := kubernetes.NewForConfig(clientConfig)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		namespace := fmt.Sprintf("pvc-clone-e2e-%d", time.Now().UnixNano())
		_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			_ = client.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		})

		sourceName := "devpod-template-abc123"
		_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: sourceName},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		}, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		writerName := "pvc-clone-marker-writer"
		_, err = client.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: writerName},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{{
					Name:         "writer",
					Image:        "alpine:3.20",
					Command:      []string{"/bin/sh", "-c", fmt.Sprintf("mkdir -p /data/devpod/0 && printf '%%s' %q > /data/devpod/0/marker", cloneMarker)},
					VolumeMounts: []corev1.VolumeMount{{Name: "source", MountPath: "/data"}},
				}},
				Volumes: []corev1.Volume{{
					Name: "source",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: sourceName},
					},
				}},
			},
		}, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Eventually(func() (corev1.PodPhase, error) {
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, writerName, metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			return pod.Status.Phase, nil
		}, 5*time.Minute, 2*time.Second).Should(gomega.Equal(corev1.PodSucceeded))

		writerPod, err := client.CoreV1().Pods(namespace).Get(ctx, writerName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(writerPod.Spec.NodeName).NotTo(gomega.BeEmpty())
		writerNode, err := client.CoreV1().Nodes().Get(ctx, writerPod.Spec.NodeName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		nodeSelectorKey := corev1.LabelTopologyZone
		nodeSelectorValue := writerNode.Labels[nodeSelectorKey]
		if nodeSelectorValue == "" {
			nodeSelectorKey = corev1.LabelHostname
			nodeSelectorValue = writerNode.Labels[nodeSelectorKey]
		}
		gomega.Expect(nodeSelectorValue).NotTo(gomega.BeEmpty(), "writer node must have a zone or hostname label")

		err = client.CoreV1().Pods(namespace).Delete(ctx, writerName, metav1.DeleteOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			_, err := client.CoreV1().Pods(namespace).Get(ctx, writerName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, time.Minute, time.Second).Should(gomega.BeTrue())

		setProviderEnv(map[string]string{
			"CREATE_NAMESPACE":                "false",
			"DEVCONTAINER_ID":                 "devpod-pvc-clone-target",
			"KUBERNETES_CONFIG":               kubeConfig,
			"KUBERNETES_NAMESPACE":            namespace,
			"KUBERNETES_PULL_SECRETS_ENABLED": "false",
			"NODE_SELECTOR":                   fmt.Sprintf("%s=%s", nodeSelectorKey, nodeSelectorValue),
			"PVC_DATA_SOURCE":                 sourceName,
		})
		providerOptions, err := provideroptions.FromEnv()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		provider := providerkubernetes.NewKubernetesDriver(providerOptions, log.Default.ErrorStreamOnly())
		err = provider.RunDevContainer(ctx, providerOptions.DevContainerID, &driver.RunOptions{
			UID:        "pvc-clone-e2e-uid",
			Image:      "alpine:3.20",
			Entrypoint: "/bin/sh",
			Cmd:        []string{"-c", "cat /workspace/marker; sleep 300"},
			WorkspaceMount: &config.Mount{
				Type:   "volume",
				Target: "/workspace",
			},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		targetPod, err := client.CoreV1().Pods(namespace).Get(ctx, providerOptions.DevContainerID, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(targetPod.Spec.NodeSelector).To(gomega.HaveKeyWithValue(nodeSelectorKey, nodeSelectorValue))

		gomega.Eventually(func() string {
			raw, err := client.CoreV1().Pods(namespace).GetLogs(providerOptions.DevContainerID, &corev1.PodLogOptions{}).DoRaw(ctx)
			if err != nil {
				return ""
			}
			return string(raw)
		}, time.Minute, time.Second).Should(gomega.ContainSubstring(cloneMarker))
	})
})

func setProviderEnv(values map[string]string) {
	for name, value := range values {
		name, value := name, value
		oldValue, wasSet := os.LookupEnv(name)
		gomega.Expect(os.Setenv(name, value)).To(gomega.Succeed())
		ginkgo.DeferCleanup(func() {
			if wasSet {
				_ = os.Setenv(name, oldValue)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func TestRunE2ETests(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Devpod provider kubernetes e2e suite")
}
