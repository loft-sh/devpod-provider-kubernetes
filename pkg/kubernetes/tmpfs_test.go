package kubernetes

import (
	"strings"
	"testing"

	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	corev1 "k8s.io/api/core/v1"
)

func TestSlugifyVolumeName(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "standard dev shm path",
			target: "/dev/shm",
			want:   "dev-shm",
		},
		{
			name:   "tmp cache path",
			target: "/tmp/cache",
			want:   "tmp-cache",
		},
		{
			name:   "home user dotfile path",
			target: "/home/user/.npm",
			want:   "home-user-npm",
		},
		{
			name:   "root path edge case returns empty",
			target: "/",
			want:   "",
		},
		{
			name:   "consecutive slashes collapse",
			target: "/tmp//cache///data",
			want:   "tmp-cache-data",
		},
		{
			name:   "consecutive dashes collapse",
			target: "/foo---bar",
			want:   "foo-bar",
		},
		{
			name:   "special characters are stripped",
			target: "/path/with@special!chars#",
			want:   "path-withspecialchars",
		},
		{
			name:   "uppercase is lowercased",
			target: "/Home/User/Documents",
			want:   "home-user-documents",
		},
		{
			name:   "leading and trailing slashes stripped",
			target: "///foo///",
			want:   "foo",
		},
		{
			name:   "truncation at 63 chars",
			target: "/" + strings.Repeat("a", 100),
			want:   strings.Repeat("a", 63),
		},
		{
			name:   "no slashes returns sanitized string",
			target: "myvolume",
			want:   "myvolume",
		},
		{
			name:   "underscores are stripped",
			target: "/var/lib_data",
			want:   "var-libdata",
		},
		{
			name:   "only special characters returns empty",
			target: "/@!#",
			want:   "",
		},
		{
			name:   "truncation cuts at a dash and re-trims",
			// 62 'a's + "--x" → after collapse → 62 'a's + "-x" (64 chars)
			// truncate to 63 → 62 'a's + "-" → re-trim trailing dash → 62 'a's
			target: "/" + strings.Repeat("a", 62) + "--x",
			want:   strings.Repeat("a", 62),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugifyVolumeName(tt.target)
			if got != tt.want {
				t.Errorf("slugifyVolumeName(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestSlugifyVolumeName_MaxLength(t *testing.T) {
	// Defensive: the result must never exceed 63 characters regardless of input.
	inputs := []string{
		"/" + strings.Repeat("a", 200),
		"/" + strings.Repeat("a", 60) + "-" + strings.Repeat("b", 200),
		"/" + strings.Repeat("a-", 100),
	}
	for _, in := range inputs {
		got := slugifyVolumeName(in)
		if len(got) > maxK8sNameLength {
			t.Errorf("slugifyVolumeName(%q) length = %d, want <= %d", in, len(got), maxK8sNameLength)
		}
	}
}

func TestParseTmpfsSize(t *testing.T) {
	t.Run("size with Gi suffix", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=1Gi"},
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected quantity, got nil")
		}
		if got.String() != "1Gi" {
			t.Errorf("got = %s, want 1Gi", got.String())
		}
	})

	t.Run("size with Mi suffix", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=512Mi"},
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected quantity, got nil")
		}
		if got.String() != "512Mi" {
			t.Errorf("got = %s, want 512Mi", got.String())
		}
	})

	t.Run("size as plain bytes", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=1073741824"},
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected quantity, got nil")
		}
		if got.String() != "1073741824" {
			t.Errorf("got = %s, want 1073741824", got.String())
		}
	})

	t.Run("no size returns nil no error", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("no size with other entries returns nil no error", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"ro", "nosuid"},
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("invalid size returns error", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=not-a-quantity"},
		}
		got, err := parseTmpfsSize(mount)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got != nil {
			t.Errorf("expected nil quantity on error, got %v", got)
		}
	})

	t.Run("size= is picked up among multiple other entries", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"ro", "size=2Gi", "nosuid"},
		}
		got, err := parseTmpfsSize(mount)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected quantity, got nil")
		}
		if got.String() != "2Gi" {
			t.Errorf("got = %s, want 2Gi", got.String())
		}
	})

	t.Run("empty size value returns error", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size="},
		}
		if _, err := parseTmpfsSize(mount); err == nil {
			t.Fatal("expected error for empty size value, got nil")
		}
	})

	t.Run("nil mount returns nil no error", func(t *testing.T) {
		got, err := parseTmpfsSize(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestGetTmpfsVolumeName(t *testing.T) {
	t.Run("with source uses source directly", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Source: "mycache",
			Target: "/cache",
		}
		got := getTmpfsVolumeName(mount)
		if got != "devpod-mycache" {
			t.Errorf("got = %q, want %q", got, "devpod-mycache")
		}
	})

	t.Run("without source uses slugified target", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
		}
		got := getTmpfsVolumeName(mount)
		if got != "devpod-dev-shm" {
			t.Errorf("got = %q, want %q", got, "devpod-dev-shm")
		}
	})

	t.Run("with empty source falls back to slugified target", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Source: "",
			Target: "/tmp/cache",
		}
		got := getTmpfsVolumeName(mount)
		if got != "devpod-tmp-cache" {
			t.Errorf("got = %q, want %q", got, "devpod-tmp-cache")
		}
	})

	t.Run("with source and deep target still uses source", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Source: "fast",
			Target: "/var/lib/data/something/deep",
		}
		got := getTmpfsVolumeName(mount)
		if got != "devpod-fast" {
			t.Errorf("got = %q, want %q", got, "devpod-fast")
		}
	})
}

func TestGetTmpfsVolume(t *testing.T) {
	t.Run("with size sets sizeLimit and Memory medium", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=1Gi"},
		}
		vol := getTmpfsVolume(mount)

		if vol.Name != "devpod-dev-shm" {
			t.Errorf("vol.Name = %q, want %q", vol.Name, "devpod-dev-shm")
		}
		if vol.EmptyDir == nil {
			t.Fatal("vol.EmptyDir is nil")
		}
		if vol.EmptyDir.Medium != corev1.StorageMediumMemory {
			t.Errorf("Medium = %q, want %q", vol.EmptyDir.Medium, corev1.StorageMediumMemory)
		}
		if vol.EmptyDir.SizeLimit == nil {
			t.Fatal("SizeLimit is nil, want set")
		}
		if vol.EmptyDir.SizeLimit.String() != "1Gi" {
			t.Errorf("SizeLimit = %s, want 1Gi", vol.EmptyDir.SizeLimit.String())
		}
	})

	t.Run("without size leaves sizeLimit nil and medium Memory", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
		}
		vol := getTmpfsVolume(mount)

		if vol.Name != "devpod-dev-shm" {
			t.Errorf("vol.Name = %q, want %q", vol.Name, "devpod-dev-shm")
		}
		if vol.EmptyDir == nil {
			t.Fatal("vol.EmptyDir is nil")
		}
		if vol.EmptyDir.Medium != corev1.StorageMediumMemory {
			t.Errorf("Medium = %q, want %q", vol.EmptyDir.Medium, corev1.StorageMediumMemory)
		}
		if vol.EmptyDir.SizeLimit != nil {
			t.Errorf("SizeLimit = %v, want nil", vol.EmptyDir.SizeLimit)
		}
	})

	t.Run("with invalid size leaves sizeLimit nil", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
			Other:  []string{"size=garbage"},
		}
		vol := getTmpfsVolume(mount)

		if vol.EmptyDir == nil {
			t.Fatal("vol.EmptyDir is nil")
		}
		if vol.EmptyDir.Medium != corev1.StorageMediumMemory {
			t.Errorf("Medium = %q, want %q", vol.EmptyDir.Medium, corev1.StorageMediumMemory)
		}
		if vol.EmptyDir.SizeLimit != nil {
			t.Errorf("SizeLimit = %v, want nil (parse error should skip size)", vol.EmptyDir.SizeLimit)
		}
	})

	t.Run("with source uses source for volume name", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Source: "mycache",
			Target: "/cache",
			Other:  []string{"size=512Mi"},
		}
		vol := getTmpfsVolume(mount)
		if vol.Name != "devpod-mycache" {
			t.Errorf("vol.Name = %q, want %q", vol.Name, "devpod-mycache")
		}
	})
}

func TestGetTmpfsVolumeMount(t *testing.T) {
	t.Run("mount path and name match mount", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Target: "/dev/shm",
		}
		vm := getTmpfsVolumeMount(mount)

		if vm.Name != "devpod-dev-shm" {
			t.Errorf("vm.Name = %q, want %q", vm.Name, "devpod-dev-shm")
		}
		if vm.MountPath != "/dev/shm" {
			t.Errorf("vm.MountPath = %q, want %q", vm.MountPath, "/dev/shm")
		}
		if vm.SubPath != "" {
			t.Errorf("vm.SubPath = %q, want empty", vm.SubPath)
		}
	})

	t.Run("mount name matches getTmpfsVolume name with source", func(t *testing.T) {
		mount := &config.Mount{
			Type:   "tmpfs",
			Source: "shared",
			Target: "/tmp/shared",
		}
		vm := getTmpfsVolumeMount(mount)
		vol := getTmpfsVolume(mount)

		if vm.Name != vol.Name {
			t.Errorf("mount name %q does not match volume name %q", vm.Name, vol.Name)
		}
		if vm.Name != "devpod-shared" {
			t.Errorf("vm.Name = %q, want %q", vm.Name, "devpod-shared")
		}
		if vm.MountPath != "/tmp/shared" {
			t.Errorf("vm.MountPath = %q, want %q", vm.MountPath, "/tmp/shared")
		}
	})
}
