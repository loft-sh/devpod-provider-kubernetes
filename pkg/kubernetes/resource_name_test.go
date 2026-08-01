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

func TestDefaultDerivedResourceNamesPreserveLegacyMapping(t *testing.T) {
	driver := resourceNameTestDriver("")

	t.Run("short workspace name", func(t *testing.T) {
		const workspaceName = "devpod-feature-auth"
		const randomSuffix = "abcdef"

		workspaceLabelValue := driver.getWorkspaceLabelValue(workspaceName)
		pullSecretName := driver.getPullSecretsName(workspaceName)
		assert.Equal(t, workspaceName, workspaceLabelValue)
		assert.Equal(t, "devpod-pull-secret-devpod-feature-auth", pullSecretName)
		assert.Equal(t, "devpod-devpod-feature-auth-ced3c", driver.getArchitecturePodName(workspaceName, randomSuffix))
		assert.Equal(t, workspaceName, driver.getRoleBindingName(workspaceName))
		assert.Empty(t, validation.IsValidLabelValue(workspaceLabelValue))
		assert.LessOrEqual(t, len(workspaceLabelValue), validation.LabelValueMaxLength)
		assert.Empty(t, validation.IsDNS1123Subdomain(pullSecretName))
		assert.LessOrEqual(t, len(pullSecretName), validation.DNS1123SubdomainMaxLength)
	})

	t.Run("long workspace name", func(t *testing.T) {
		workspaceName := strings.Join([]string{
			strings.Repeat("a", 63), strings.Repeat("b", 63), strings.Repeat("c", 63), strings.Repeat("d", 61),
		}, ".")
		const legacyWorkspaceLabelValue = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-5fcf0"
		const legacyPullSecretName = "devpod-pull-secret-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc-dddddddddddddddddddddddddddddddddddd-9c5e9"

		workspaceLabelValue := driver.getWorkspaceLabelValue(workspaceName)
		pullSecretName := driver.getPullSecretsName(workspaceName)
		assert.Equal(t, legacyWorkspaceLabelValue, workspaceLabelValue)
		assert.Equal(t, legacyPullSecretName, pullSecretName)
		assert.Empty(t, validation.IsValidLabelValue(workspaceLabelValue))
		assert.Len(t, workspaceLabelValue, validation.LabelValueMaxLength)
		assert.Empty(t, validation.IsDNS1123Subdomain(pullSecretName))
		assert.Len(t, pullSecretName, validation.DNS1123SubdomainMaxLength)
	})
}

func TestExactDerivedResourceNamesAreValidAndCollisionResistant(t *testing.T) {
	segments := []string{
		strings.Repeat("a", 30), strings.Repeat("b", 30), strings.Repeat("c", 30), strings.Repeat("d", 30),
		strings.Repeat("e", 30), strings.Repeat("f", 30), strings.Repeat("g", 30), strings.Repeat("h", 30),
	}
	dottedName := strings.Join(segments, ".")
	hyphenatedName := strings.Replace(dottedName, ".", "-", 1)
	assert.Empty(t, validation.IsDNS1123Subdomain(dottedName))
	assert.Empty(t, validation.IsDNS1123Subdomain(hyphenatedName))

	maxNameOne := strings.Join([]string{
		strings.Repeat("i", 63), strings.Repeat("j", 63), strings.Repeat("k", 63), strings.Repeat("l", 61),
	}, ".")
	maxNameTwo := maxNameOne[:len(maxNameOne)-1] + "m"
	assert.Len(t, maxNameOne, validation.DNS1123SubdomainMaxLength)
	assert.Empty(t, validation.IsDNS1123Subdomain(maxNameOne))
	assert.Empty(t, validation.IsDNS1123Subdomain(maxNameTwo))

	for _, resourceName := range []string{"workspace-exact-name", dottedName, hyphenatedName, maxNameOne, maxNameTwo} {
		t.Run(resourceName[:8], func(t *testing.T) {
			driver := resourceNameTestDriver(resourceName)
			workspaceLabelValue := driver.getWorkspaceLabelValue(resourceName)
			pullSecretName := driver.getPullSecretsName(resourceName)
			architecturePodName := driver.getArchitecturePodName(resourceName, "abcdef")
			roleBindingName := driver.getRoleBindingName(resourceName)

			assert.Empty(t, validation.IsValidLabelValue(workspaceLabelValue))
			assert.LessOrEqual(t, len(workspaceLabelValue), validation.LabelValueMaxLength)
			assert.Empty(t, validation.IsDNS1123Subdomain(pullSecretName))
			assert.LessOrEqual(t, len(pullSecretName), validation.DNS1123SubdomainMaxLength)
			assert.Empty(t, validation.IsDNS1123Subdomain(architecturePodName))
			assert.LessOrEqual(t, len(architecturePodName), architecturePodNameMaxLength)
			assert.Empty(t, validation.IsDNS1123Subdomain(roleBindingName))

			digest := exactResourceNameDigest(resourceName)
			assert.Contains(t, workspaceLabelValue, digest)
			assert.Contains(t, pullSecretName, digest)
			assert.Contains(t, architecturePodName, digest)
		})
	}

	dottedDriver := resourceNameTestDriver(dottedName)
	hyphenatedDriver := resourceNameTestDriver(hyphenatedName)
	assert.NotEqual(t, dottedDriver.getWorkspaceLabelValue(dottedName), hyphenatedDriver.getWorkspaceLabelValue(hyphenatedName))
	assert.NotEqual(t, dottedDriver.getPullSecretsName(dottedName), hyphenatedDriver.getPullSecretsName(hyphenatedName))
	assert.NotEqual(t, dottedDriver.getArchitecturePodName(dottedName, "abcdef"), hyphenatedDriver.getArchitecturePodName(hyphenatedName, "abcdef"))

	shortDottedName := "workspace.exact-name"
	shortDottedDriver := resourceNameTestDriver(shortDottedName)
	shortArchitectureName := shortDottedDriver.getArchitecturePodName(shortDottedName, "abcdef")
	assert.Contains(t, shortArchitectureName, exactResourceNameDigest(shortDottedName))
	assert.True(t, strings.HasSuffix(shortArchitectureName, "-abcdef"))
	assert.NotEqual(t, shortArchitectureName, resourceNameTestDriver("workspace-exact-name").getArchitecturePodName("workspace-exact-name", "abcdef"))

	maxDriverOne := resourceNameTestDriver(maxNameOne)
	maxDriverTwo := resourceNameTestDriver(maxNameTwo)
	assert.NotEqual(t, maxDriverOne.getWorkspaceLabelValue(maxNameOne), maxDriverTwo.getWorkspaceLabelValue(maxNameTwo))
	assert.NotEqual(t, maxDriverOne.getPullSecretsName(maxNameOne), maxDriverTwo.getPullSecretsName(maxNameTwo))
	assert.NotEqual(t, maxDriverOne.getArchitecturePodName(maxNameOne, "abcdef"), maxDriverTwo.getArchitecturePodName(maxNameTwo, "abcdef"))
}

func TestExactDerivedResourceNamesDoNotCollideWithShortExactNames(t *testing.T) {
	transformedName := strings.Join([]string{
		strings.Repeat("a", 63), strings.Repeat("b", 63), strings.Repeat("c", 63), strings.Repeat("d", 61),
	}, ".")
	const randomSuffix = "abcdef"
	transformedDriver := resourceNameTestDriver(transformedName)

	tests := []struct {
		name            string
		transformed     string
		shortExactName  func(string) string
		deriveShortName func(*KubernetesDriver, string) string
	}{
		{
			name:           "workspace label",
			transformed:    transformedDriver.getWorkspaceLabelValue(transformedName),
			shortExactName: func(output string) string { return output },
			deriveShortName: func(driver *KubernetesDriver, name string) string {
				return driver.getWorkspaceLabelValue(name)
			},
		},
		{
			name:        "pull secret",
			transformed: transformedDriver.getPullSecretsName(transformedName),
			shortExactName: func(output string) string {
				return strings.TrimPrefix(output, "devpod-pull-secret-")
			},
			deriveShortName: func(driver *KubernetesDriver, name string) string {
				return driver.getPullSecretsName(name)
			},
		},
		{
			name:        "architecture pod",
			transformed: transformedDriver.getArchitecturePodName(transformedName, randomSuffix),
			shortExactName: func(output string) string {
				return strings.TrimSuffix(strings.TrimPrefix(output, "devpod-"), "-"+randomSuffix)
			},
			deriveShortName: func(driver *KubernetesDriver, name string) string {
				return driver.getArchitecturePodName(name, randomSuffix)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shortExactName := test.shortExactName(test.transformed)
			assert.Empty(t, validation.IsDNS1123Subdomain(shortExactName))
			shortDerivedName := test.deriveShortName(resourceNameTestDriver(shortExactName), shortExactName)

			assert.NotEqual(t, test.transformed, shortDerivedName)
			assert.Contains(t, shortDerivedName, exactResourceNameDigest(shortExactName))
		})
	}
}

func TestTargetArchitectureUsesExactHelperMapping(t *testing.T) {
	resourceName := "workspace-exact-name"
	driver := resourceNameTestDriver(resourceName)
	manifestPath := filepath.Join(t.TempDir(), "architecture-pod.json")
	driver.kubectl = writeFakeKubectl(t, `
case "$*" in
  "create -f -") cat > "$MANIFEST_PATH" ;;
  "logs "*) printf 'x86_64\n' ;;
esac
`)
	t.Setenv("MANIFEST_PATH", manifestPath)

	architecture, err := driver.TargetArchitecture(context.Background(), "feature-auth")
	mustNoError(t, err)
	assert.Equal(t, "amd64", architecture)

	podRaw, err := os.ReadFile(manifestPath)
	mustNoError(t, err)
	pod := &corev1.Pod{}
	mustNoError(t, json.Unmarshal(podRaw, pod))
	assert.Empty(t, validation.IsDNS1123Subdomain(pod.Name))
	assert.LessOrEqual(t, len(pod.Name), architecturePodNameMaxLength)
	assert.Contains(t, pod.Name, exactResourceNameDigest(resourceName))
	expectedWorkspaceLabel := driver.getWorkspaceLabelValue(resourceName)
	assert.Contains(t, expectedWorkspaceLabel, exactResourceNameDigest(resourceName))
	assert.Equal(t, expectedWorkspaceLabel, pod.Labels[DevPodWorkspaceLabel])
}

func TestExactWorkspaceLabelIsConsistentAcrossArchitectureLifecycle(t *testing.T) {
	const resourceName = "workspace-exact-name"
	driver := resourceNameTestDriver(resourceName)
	commandLog := filepath.Join(t.TempDir(), "commands.log")
	manifestPath := filepath.Join(t.TempDir(), "workspace-pod.json")
	driver.kubectl = writeFakeKubectl(t, `
printf '%s\n' "$*" >> "$COMMAND_LOG"
case "$*" in
  "get pods -o=name -l "*) printf 'pod/architecture-helper\n' ;;
  "create -f -") cat > "$MANIFEST_PATH" ;;
esac
`)
	t.Setenv("COMMAND_LOG", commandLog)
	t.Setenv("MANIFEST_PATH", manifestPath)

	mustNoError(t, driver.runContainer(context.Background(), resourceName, resourceNameRunOptions(), false))

	expectedLabel := driver.getWorkspaceLabelValue(resourceName)
	assert.Contains(t, expectedLabel, exactResourceNameDigest(resourceName))
	rawLog, err := os.ReadFile(commandLog)
	mustNoError(t, err)
	commands := string(rawLog)
	assert.Contains(t, commands, "get pods -o=name -l "+DevPodWorkspaceLabel+"="+expectedLabel)
	assert.Contains(t, commands, "delete pods --force -l "+DevPodWorkspaceLabel+"="+expectedLabel)

	podRaw, err := os.ReadFile(manifestPath)
	mustNoError(t, err)
	pod := &corev1.Pod{}
	mustNoError(t, json.Unmarshal(podRaw, pod))
	requirements := pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions
	assert.Equal(t, []string{expectedLabel}, requirements[0].Values)
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
