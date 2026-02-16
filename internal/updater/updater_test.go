package updater

import (
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"newer major", "2.0.0", "1.0.0", true},
		{"newer minor", "1.2.0", "1.1.0", true},
		{"newer patch", "1.0.2", "1.0.1", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"older version", "1.0.0", "1.1.0", false},
		{"with v prefix", "v2.0.0", "v1.0.0", true},
		{"mixed prefix", "2.0.0", "v1.0.0", true},
		{"pre-release", "1.0.0-beta", "1.0.0-alpha", false},
		{"pre-release newer", "1.1.0-beta", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    [3]int
	}{
		{"simple", "1.2.3", [3]int{1, 2, 3}},
		{"with v prefix", "v1.2.3", [3]int{1, 2, 3}},
		{"with pre-release", "1.2.3-beta", [3]int{1, 2, 3}},
		{"missing patch", "1.2", [3]int{1, 2, 0}},
		{"missing minor and patch", "1", [3]int{1, 0, 0}},
		{"zero version", "0.0.0", [3]int{0, 0, 0}},
		{"large numbers", "10.20.30", [3]int{10, 20, 30}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVersion(tt.version)
			if got != tt.want {
				t.Errorf("parseVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestBuildReleaseURL(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		goos        string
		goarch      string
		wantURL     string
		wantArchive string
	}{
		{
			name:        "linux amd64",
			version:     "1.2.3",
			goos:        "linux",
			goarch:      "amd64",
			wantURL:     "https://github.com/Octrafic/octrafic-cli/releases/download/v1.2.3/octrafic_Linux_x86_64.tar.gz",
			wantArchive: "octrafic_Linux_x86_64.tar.gz",
		},
		{
			name:        "linux arm64",
			version:     "1.2.3",
			goos:        "linux",
			goarch:      "arm64",
			wantURL:     "https://github.com/Octrafic/octrafic-cli/releases/download/v1.2.3/octrafic_Linux_arm64.tar.gz",
			wantArchive: "octrafic_Linux_arm64.tar.gz",
		},
		{
			name:        "darwin amd64",
			version:     "1.2.3",
			goos:        "darwin",
			goarch:      "amd64",
			wantURL:     "https://github.com/Octrafic/octrafic-cli/releases/download/v1.2.3/octrafic_Darwin_x86_64.tar.gz",
			wantArchive: "octrafic_Darwin_x86_64.tar.gz",
		},
		{
			name:        "darwin arm64",
			version:     "1.2.3",
			goos:        "darwin",
			goarch:      "arm64",
			wantURL:     "https://github.com/Octrafic/octrafic-cli/releases/download/v1.2.3/octrafic_Darwin_arm64.tar.gz",
			wantArchive: "octrafic_Darwin_arm64.tar.gz",
		},
		{
			name:        "windows",
			version:     "1.2.3",
			goos:        "windows",
			goarch:      "amd64",
			wantURL:     "https://github.com/Octrafic/octrafic-cli/releases/download/v1.2.3/octrafic_Windows_x86_64.zip",
			wantArchive: "octrafic_Windows_x86_64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotArchive := buildReleaseURL(tt.version, tt.goos, tt.goarch)
			if gotURL != tt.wantURL {
				t.Errorf("buildReleaseURL(%q, %q, %q) URL = %q, want %q", tt.version, tt.goos, tt.goarch, gotURL, tt.wantURL)
			}
			if gotArchive != tt.wantArchive {
				t.Errorf("buildReleaseURL(%q, %q, %q) archive = %q, want %q", tt.version, tt.goos, tt.goarch, gotArchive, tt.wantArchive)
			}
		})
	}
}

func TestCmdExists(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"existing command", "ls", true},
		{"non-existing command", "thisdoesnotexist123456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmdExists(tt.cmd)
			if got != tt.want {
				t.Errorf("cmdExists(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestDetectInstallationMethod(t *testing.T) {
	method := DetectInstallationMethod()

	if method == "" {
		t.Error("DetectInstallationMethod() returned empty string")
	}

	validMethods := []InstallMethod{
		MethodHomebrew,
		MethodYay,
		MethodParu,
		MethodDeb,
		MethodRPM,
		MethodScript,
		MethodManual,
		MethodUnknown,
	}

	found := false
	for _, valid := range validMethods {
		if method == valid {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("DetectInstallationMethod() = %q, not a valid installation method", method)
	}
}
