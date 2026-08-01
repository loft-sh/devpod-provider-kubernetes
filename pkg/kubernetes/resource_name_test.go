package kubernetes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	provideroptions "github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/driver"
	"github.com/loft-sh/log"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestWorkspaceResourceName(t *testing.T) {
	const workspaceID = "feature-auth"

	t.Run("default", func(t *testing.T) {
		driver := resourceNameTestDriver("")
		assert.Equal(t, "devpod-feature-auth", getID(workspaceID))
		assert.Equal(t, "devpod-feature-auth", driver.getID(workspaceID))
	})

	t.Run("exact override", func(t *testing.T) {
		driver := resourceNameTestDriver("workspace.exact-name")
		assert.Equal(t, "workspace.exact-name", driver.getID(workspaceID))
	})
}

func TestDerivedResourceNamesRemainValid(t *testing.T) {
	assert.Equal(t, "devpod-feature-auth", getWorkspaceLabelValue("devpod-feature-auth"))
	assert.Equal(t, "devpod-pull-secret-devpod-feature-auth", getPullSecretsName("devpod-feature-auth"))

	resourceName := strings.Repeat("a", validation.DNS1123SubdomainMaxLength)
	workspaceLabelValue := getWorkspaceLabelValue(resourceName)
	assert.Empty(t, validation.IsValidLabelValue(workspaceLabelValue))
	assert.LessOrEqual(t, len(workspaceLabelValue), validation.LabelValueMaxLength)
	pullSecretName := getPullSecretsName(resourceName)
	assert.Empty(t, validation.IsDNS1123Subdomain(pullSecretName))

	dottedResourceName := strings.Repeat("a.", 126) + "a"
	assert.Empty(t, validation.IsDNS1123Subdomain(dottedResourceName))
	assert.Empty(t, validation.IsDNS1123Subdomain(getPullSecretsName(dottedResourceName)))
}

func TestRenderedWorkspaceResourcesUseExactName(t *testing.T) {
	const resourceName = "workspace.exact-name"
	driver := resourceNameTestDriver(resourceName)

	rawPVC, err := driver.buildPersistentVolumeClaim(driver.getID("feature-auth"), resourceNameRunOptions())
	mustNoError(t, err)
	pvc := &corev1.PersistentVolumeClaim{}
	mustNoError(t, json.Unmarshal([]byte(rawPVC), pvc))
	assert.Equal(t, resourceName, pvc.Name)

	manifestPath := filepath.Join(t.TempDir(), "pod.json")
	driver.kubectl = writeFakeKubectl(t, `
printf '%s\n' "$*" >> "$COMMAND_LOG"
case "$*" in
  *"create -f -"*) cat > "$MANIFEST_PATH" ;;
esac
`)
	t.Setenv("COMMAND_LOG", filepath.Join(t.TempDir(), "commands.log"))
	t.Setenv("MANIFEST_PATH", manifestPath)

	mustNoError(t, driver.runContainer(context.Background(), driver.getID("feature-auth"), resourceNameRunOptions(), false))
	podRaw, err := os.ReadFile(manifestPath)
	mustNoError(t, err)
	pod := &corev1.Pod{}
	mustNoError(t, json.Unmarshal(podRaw, pod))
	assert.Equal(t, resourceName, pod.Name)
	persistedOptions := &provideroptions.Options{}
	mustNoError(t, json.Unmarshal([]byte(pod.Annotations[DevPodLastAppliedAnnotation]), persistedOptions))
	assert.Equal(t, resourceName, persistedOptions.KubernetesResourceName)
	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("expected one volume, got %d", len(pod.Spec.Volumes))
	}
	assert.Equal(t, resourceName, pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName)
}

func TestLifecycleCommandsUseExactName(t *testing.T) {
	const resourceName = "workspace.exact-name"
	driver := resourceNameTestDriver(resourceName)
	commandLog := filepath.Join(t.TempDir(), "commands.log")
	pvcResponse := filepath.Join(t.TempDir(), "pvc.json")

	containerInfo, err := driver.getDevContainerInformation(resourceName, resourceNameRunOptions())
	mustNoError(t, err)
	pvcRaw, err := json.Marshal(&corev1.PersistentVolumeClaim{
		ObjectMeta: objectMetaWithInfo(resourceName, containerInfo),
	})
	mustNoError(t, err)
	mustNoError(t, os.WriteFile(pvcResponse, pvcRaw, 0o600))

	driver.kubectl = writeFakeKubectl(t, `
printf '%s\n' "$*" >> "$COMMAND_LOG"
case "$*" in
  "get pvc "*" --ignore-not-found -o json") cat "$PVC_RESPONSE" ;;
  *"create -f -"*) cat >/dev/null ;;
esac
`)
	t.Setenv("COMMAND_LOG", commandLog)
	t.Setenv("PVC_RESPONSE", pvcResponse)

	ctx := context.Background()
	mustNoError(t, driver.RunDevContainer(ctx, "feature-auth", resourceNameRunOptions()))
	mustNoError(t, driver.StartDevContainer(ctx, "feature-auth"))
	_, err = driver.FindDevContainer(ctx, "feature-auth")
	mustNoError(t, err)
	mustNoError(t, driver.StopDevContainer(ctx, "feature-auth"))
	mustNoError(t, driver.DeleteDevContainer(ctx, "feature-auth"))

	rawLog, err := os.ReadFile(commandLog)
	mustNoError(t, err)
	commands := string(rawLog)
	for _, expected := range []string{
		"get pvc " + resourceName,
		"get pod " + resourceName,
		"delete po " + resourceName,
		"delete pvc " + resourceName,
	} {
		assert.Contains(t, commands, expected)
	}
	assert.NotContains(t, commands, "devpod-feature-auth")
}

func resourceNameTestDriver(resourceName string) *KubernetesDriver {
	return NewKubernetesDriver(&provideroptions.Options{
		ComparableOptions: provideroptions.ComparableOptions{
			KubernetesResourceName: resourceName,
		},
		PodTimeout: "1s",
	}, log.Default.ErrorStreamOnly()).(*KubernetesDriver)
}

func resourceNameRunOptions() *driver.RunOptions {
	return &driver.RunOptions{
		UID:        "workspace-uid",
		Image:      "alpine:3.20",
		Entrypoint: "/bin/sh",
		Cmd:        []string{"-c", "sleep 300"},
		WorkspaceMount: &config.Mount{
			Type:   "volume",
			Target: "/workspace",
		},
	}
}

func objectMetaWithInfo(name, containerInfo string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Annotations: map[string]string{
			DevPodInfoAnnotation: containerInfo,
		},
	}
}

func writeFakeKubectl(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubectl")
	script := "#!/bin/sh\nset -eu\n" + strings.TrimSpace(body) + "\n"
	mustNoError(t, os.WriteFile(path, []byte(script), 0o700))
	return path
}

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
