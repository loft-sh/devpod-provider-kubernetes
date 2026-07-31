package kubernetes

import (
	"encoding/json"
	"os"
	"testing"

	provideroptions "github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/devpod/pkg/driver"
	"github.com/loft-sh/log"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildPersistentVolumeClaimWithDataSource(t *testing.T) {
	driver := testDriver(t, map[string]string{"PVC_DATA_SOURCE": "devpod-template-abc123"})

	raw, err := driver.buildPersistentVolumeClaim("devpod-feature-auth", testRunOptions())
	assert.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	assert.NoError(t, json.Unmarshal([]byte(raw), pvc))
	assert.Equal(t, &corev1.TypedLocalObjectReference{
		Kind: "PersistentVolumeClaim",
		Name: "devpod-template-abc123",
	}, pvc.Spec.DataSource)
	assert.NotEmpty(t, pvc.Annotations[DevPodInfoAnnotation])
	assert.Equal(t, "workspace-uid", pvc.Labels[DevPodWorkspaceUIDLabel])
}

func TestBuildPersistentVolumeClaimWithoutDataSource(t *testing.T) {
	for _, test := range []struct {
		name string
		env  map[string]string
	}{
		{name: "absent", env: nil},
		{name: "empty", env: map[string]string{"PVC_DATA_SOURCE": ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := testDriver(t, test.env)

			raw, err := driver.buildPersistentVolumeClaim("devpod-feature-auth", testRunOptions())
			assert.NoError(t, err)

			pvc := &corev1.PersistentVolumeClaim{}
			assert.NoError(t, json.Unmarshal([]byte(raw), pvc))
			assert.Nil(t, pvc.Spec.DataSource)
			assert.NotEmpty(t, pvc.Annotations[DevPodInfoAnnotation])
			assert.Equal(t, "workspace-uid", pvc.Labels[DevPodWorkspaceUIDLabel])
		})
	}
}

func testDriver(t *testing.T, env map[string]string) *KubernetesDriver {
	t.Helper()
	t.Setenv("DEVCONTAINER_ID", "devpod-feature-auth")
	if value, ok := env["PVC_DATA_SOURCE"]; ok {
		t.Setenv("PVC_DATA_SOURCE", value)
	} else {
		oldValue, wasSet := os.LookupEnv("PVC_DATA_SOURCE")
		if err := os.Unsetenv("PVC_DATA_SOURCE"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv("PVC_DATA_SOURCE", oldValue)
			} else {
				_ = os.Unsetenv("PVC_DATA_SOURCE")
			}
		})
	}
	for name, value := range env {
		if name != "PVC_DATA_SOURCE" {
			t.Setenv(name, value)
		}
	}

	providerOptions, err := provideroptions.FromEnv()
	if err != nil {
		t.Fatal(err)
	}

	return NewKubernetesDriver(providerOptions, log.Default.ErrorStreamOnly()).(*KubernetesDriver)
}

func testRunOptions() *driver.RunOptions {
	return &driver.RunOptions{UID: "workspace-uid"}
}
