package kubernetes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/loft-sh/devpod/pkg/encoding"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	exactResourceNameDigestLength = 16
	architecturePodNameMaxLength  = 32
)

func (k *KubernetesDriver) getID(workspaceID string) string {
	if k.options.KubernetesResourceName != "" {
		return k.options.KubernetesResourceName
	}

	return getID(workspaceID)
}

func getID(workspaceID string) string {
	return "devpod-" + workspaceID
}

func (k *KubernetesDriver) getWorkspaceLabelValue(workspaceName string) string {
	if k.options.KubernetesResourceName == "" {
		return encoding.SafeConcatNameMax([]string{workspaceName}, validation.LabelValueMaxLength)
	}

	return deriveExactResourceName(workspaceName, workspaceName, validation.LabelValueMaxLength, false)
}

func (k *KubernetesDriver) getPullSecretsName(workspaceName string) string {
	name := fmt.Sprintf("devpod-pull-secret-%s", workspaceName)
	if k.options.KubernetesResourceName == "" {
		if len(name) <= validation.DNS1123SubdomainMaxLength {
			return name
		}

		return encoding.SafeConcatNameMax(
			[]string{"devpod", "pull", "secret", strings.ReplaceAll(workspaceName, ".", "-")},
			validation.DNS1123SubdomainMaxLength,
		)
	}

	return deriveExactResourceName(workspaceName, name, validation.DNS1123SubdomainMaxLength, len(name) > validation.DNS1123SubdomainMaxLength)
}

func (k *KubernetesDriver) getArchitecturePodName(workspaceName, randomSuffix string) string {
	if k.options.KubernetesResourceName == "" {
		return encoding.SafeConcatNameMax([]string{"devpod", workspaceName, randomSuffix}, architecturePodNameMaxLength)
	}

	name := strings.Join([]string{"devpod", workspaceName, randomSuffix}, "-")
	normalizedName := strings.ReplaceAll(name, ".", "-")
	if normalizedName == name && len(name) <= architecturePodNameMaxLength {
		return name
	}

	digest := exactResourceNameDigest(workspaceName)
	prefixLength := architecturePodNameMaxLength - len(digest) - len(randomSuffix) - 2
	prefix := strings.TrimRight(normalizedName[:min(len(normalizedName), prefixLength)], ".-_")
	return strings.Join([]string{prefix, digest, randomSuffix}, "-")
}

func (k *KubernetesDriver) getRoleBindingName(workspaceName string) string {
	return workspaceName
}

func deriveExactResourceName(originalName, name string, maxLength int, normalize bool) string {
	normalizedName := name
	if normalize {
		normalizedName = strings.ReplaceAll(normalizedName, ".", "-")
	}
	if normalizedName == name && len(name) <= maxLength {
		return name
	}

	digest := exactResourceNameDigest(originalName)
	prefixLength := maxLength - len(digest) - 1
	prefix := strings.TrimRight(normalizedName[:min(len(normalizedName), prefixLength)], ".-_")
	return prefix + "-" + digest
}

func exactResourceNameDigest(resourceName string) string {
	digest := sha256.Sum256([]byte(resourceName))
	return hex.EncodeToString(digest[:])[:exactResourceNameDigestLength]
}
