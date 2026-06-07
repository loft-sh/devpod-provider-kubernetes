package kubernetes

import (
	"fmt"
	"strings"

	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// maxK8sNameLength is the maximum length for a Kubernetes resource name.
// See: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
const maxK8sNameLength = 63

// slugifyVolumeName converts a mount target path into a Kubernetes-safe
// volume name segment. It is intended to be used as a suffix, not a
// complete volume name.
//
// The rules are:
//  1. Strip leading and trailing `/`
//  2. Replace `/` with `-`
//  3. Strip all characters not in `[a-z0-9-]`
//  4. Collapse consecutive dashes
//  5. Truncate to 63 characters if needed
func slugifyVolumeName(target string) string {
	// 1. Strip leading and trailing slashes
	target = strings.Trim(target, "/")

	// 2. Replace slashes with dashes
	target = strings.ReplaceAll(target, "/", "-")

	// 3. Strip characters not in [a-z0-9-]
	//    We iterate runes to be safe with multi-byte characters.
	var b strings.Builder
	b.Grow(len(target))
	for _, r := range strings.ToLower(target) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		default:
			// drop character
		}
	}
	cleaned := b.String()

	// 4. Collapse consecutive dashes
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}
	cleaned = strings.Trim(cleaned, "-")

	// 5. Truncate to 63 chars
	if len(cleaned) > maxK8sNameLength {
		cleaned = cleaned[:maxK8sNameLength]
		// Re-trim trailing dashes in case truncation cut a double dash
		cleaned = strings.TrimRight(cleaned, "-")
	}

	return cleaned
}

// parseTmpfsSize extracts the optional `size=` value from a mount's
// `Other` field and parses it as a Kubernetes resource.Quantity.
//
// Returns (nil, nil) when no `size=` entry is present.
// Returns an error when a `size=` entry is present but cannot be parsed.
func parseTmpfsSize(mount *config.Mount) (*resource.Quantity, error) {
	if mount == nil {
		return nil, nil
	}

	const prefix = "size="
	var sizeStr string
	found := false
	for _, entry := range mount.Other {
		if strings.HasPrefix(entry, prefix) {
			sizeStr = strings.TrimPrefix(entry, prefix)
			found = true
			break
		}
	}

	if !found {
		return nil, nil
	}

	if sizeStr == "" {
		return nil, fmt.Errorf("empty size value in mount '%s'", mount.String())
	}

	quantity, err := resource.ParseQuantity(sizeStr)
	if err != nil {
		return nil, fmt.Errorf("parse tmpfs size %q for mount '%s': %w", sizeStr, mount.String(), err)
	}

	return &quantity, nil
}

// getTmpfsVolumeName returns the volume name to use for a tmpfs mount.
// If the mount has an explicit `source`, that source is used directly
// (prefixed with `devpod-`). Otherwise the slugified target is used.
func getTmpfsVolumeName(mount *config.Mount) string {
	if mount.Source != "" {
		return "devpod-" + mount.Source
	}
	return "devpod-" + slugifyVolumeName(mount.Target)
}

// getTmpfsVolume returns a Kubernetes Volume describing the tmpfs mount
// as an in-memory emptyDir. An optional sizeLimit is set when the
// mount's `Other` field includes a parseable `size=` entry.
func getTmpfsVolume(mount *config.Mount) corev1.Volume {
	emptyDir := &corev1.EmptyDirVolumeSource{
		Medium: corev1.StorageMediumMemory,
	}

	if quantity, err := parseTmpfsSize(mount); err != nil {
		// Skip sizeLimit on parse error — caller is expected to log.
		_ = quantity
	} else if quantity != nil {
		emptyDir.SizeLimit = quantity
	}

	return corev1.Volume{
		Name: getTmpfsVolumeName(mount),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: emptyDir,
		},
	}
}

// getTmpfsVolumeMount returns the VolumeMount that mounts a tmpfs
// volume into a container at the mount's target path.
func getTmpfsVolumeMount(mount *config.Mount) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      getTmpfsVolumeName(mount),
		MountPath: mount.Target,
	}
}
