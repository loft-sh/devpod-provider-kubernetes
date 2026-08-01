package options

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
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

func TestFromEnvKubernetesResourceName(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "feature-auth")
	t.Setenv("KUBERNETES_RESOURCE_NAME", "workspace.exact-name")

	got, err := FromEnv()
	assert.NoError(t, err)
	assert.Equal(t, "workspace.exact-name", got.ComparableOptions.KubernetesResourceName)

	raw, err := json.Marshal(got)
	assert.NoError(t, err)

	restored := &Options{}
	assert.NoError(t, json.Unmarshal(raw, restored))
	assert.Equal(t, got.ComparableOptions, restored.ComparableOptions)
}

func TestComparableOptionsIncludesKubernetesResourceName(t *testing.T) {
	left := &ComparableOptions{KubernetesResourceName: "workspace-one"}
	right := &ComparableOptions{KubernetesResourceName: "workspace-one"}
	assert.True(t, Equal(left, right))

	right.KubernetesResourceName = "workspace-two"
	assert.False(t, Equal(left, right))
}

func TestFromEnvKubernetesResourceNameOptional(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "feature-auth")

	for _, test := range []struct {
		name  string
		value *string
	}{
		{name: "absent"},
		{name: "empty", value: pointerTo("")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.value == nil {
				unsetEnvironment(t, "KUBERNETES_RESOURCE_NAME")
			} else {
				t.Setenv("KUBERNETES_RESOURCE_NAME", *test.value)
			}

			got, err := FromEnv()
			assert.NoError(t, err)
			assert.Empty(t, got.KubernetesResourceName)
		})
	}
}

func TestFromEnvAcceptsMaxLengthKubernetesResourceName(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "feature-auth")
	value := strings.Repeat("a", 253)
	t.Setenv("KUBERNETES_RESOURCE_NAME", value)

	got, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, value, got.KubernetesResourceName)
}

func TestFromEnvRejectsInvalidKubernetesResourceName(t *testing.T) {
	t.Setenv("DEVCONTAINER_ID", "feature-auth")

	for _, value := range []string{
		"Uppercase",
		"unsafe_name",
		"unsafe/name",
		"-leading-hyphen",
		"trailing-hyphen-",
		strings.Repeat("a", 254),
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("KUBERNETES_RESOURCE_NAME", value)

			_, err := FromEnv()
			assert.ErrorContains(t, err, "KUBERNETES_RESOURCE_NAME")
		})
	}
}

type providerOptionGroup struct {
	Options []string `json:"options"`
	Name    string   `json:"name"`
}

type providerSchema struct {
	OptionGroups []providerOptionGroup     `json:"optionGroups"`
	Options      map[string]map[string]any `json:"options"`
}

func TestProviderSchemasIncludeNewOptions(t *testing.T) {
	schemas := map[string]providerSchema{}
	for _, filename := range []string{"provider.yaml", "provider-dev.yaml"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "hack", "provider", filename))
		if err != nil {
			t.Fatal(err)
		}
		schema := providerSchema{}
		if err := yaml.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		for _, optionName := range []string{"PVC_DATA_SOURCE", "KUBERNETES_RESOURCE_NAME"} {
			definition, ok := schema.Options[optionName]
			assert.True(t, ok, "%s must define %s", filename, optionName)
			assert.NotEmpty(t, definition["description"], "%s must describe %s", filename, optionName)
			assert.True(t, schemaGroupHasOption(schema.OptionGroups, "Advanced Options", optionName), "%s must include %s in Advanced Options", filename, optionName)
		}
		schemas[filename] = schema
	}

	assert.Equal(t, schemas["provider.yaml"].Options["PVC_DATA_SOURCE"], schemas["provider-dev.yaml"].Options["PVC_DATA_SOURCE"])
	assert.Equal(t, schemas["provider.yaml"].Options["KUBERNETES_RESOURCE_NAME"], schemas["provider-dev.yaml"].Options["KUBERNETES_RESOURCE_NAME"])
}

func schemaGroupHasOption(groups []providerOptionGroup, groupName, optionName string) bool {
	for _, group := range groups {
		if group.Name != groupName {
			continue
		}
		for _, groupedOption := range group.Options {
			if groupedOption == optionName {
				return true
			}
		}
	}
	return false
}

func pointerTo(value string) *string {
	return &value
}
