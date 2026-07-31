package options

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromEnvPvcDataSource(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "devpod-feature-auth")
	t.Setenv("PVC_DATA_SOURCE", "devpod-template-abc123")

	got, err := FromEnv()
	assert.NoError(t, err)
	assert.Equal(t, "devpod-template-abc123", got.ComparableOptions.PvcDataSource)

	raw, err := json.Marshal(got)
	assert.NoError(t, err)

	restored := &Options{}
	assert.NoError(t, json.Unmarshal(raw, restored))
	assert.Equal(t, got.ComparableOptions, restored.ComparableOptions)
}

func TestFromEnvPvcDataSourceOptional(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "devpod-feature-auth")

	t.Run("absent", func(t *testing.T) {
		unsetEnvironment(t, "PVC_DATA_SOURCE")

		got, err := FromEnv()
		assert.NoError(t, err)
		assert.Empty(t, got.PvcDataSource)
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("PVC_DATA_SOURCE", "")

		got, err := FromEnv()
		assert.NoError(t, err)
		assert.Empty(t, got.PvcDataSource)
	})
}

func unsetEnvironment(t *testing.T, name string) {
	t.Helper()
	oldValue, wasSet := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(name, oldValue)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestFromEnvRejectsInvalidPvcDataSource(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "devpod-feature-auth")

	for _, value := range []string{
		"Not_A_DNS_Name",
		"devpod/template",
		strings.Repeat("a", 254),
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PVC_DATA_SOURCE", value)

			_, err := FromEnv()
			assert.ErrorContains(t, err, "PVC_DATA_SOURCE")
		})
	}
}
