package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "Octrafic"
	repoName  = "octrafic-cli"
	apiBase   = "https://api.github.com/repos/" + repoOwner + "/" + repoName
)

// InstallMethod represents how octrafic was installed
type InstallMethod string

const (
	MethodHomebrew InstallMethod = "homebrew"
	MethodYay      InstallMethod = "yay"
	MethodParu     InstallMethod = "paru"
	MethodDeb      InstallMethod = "deb"
	MethodRPM      InstallMethod = "rpm"
	MethodScript   InstallMethod = "script"
	MethodManual   InstallMethod = "manual"
	MethodUnknown  InstallMethod = "unknown"
)

// UpdateInfo holds information about available updates
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	HTMLURL        string
	IsNewer        bool
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// CheckLatestVersion checks GitHub for the latest release and compares with current version
func CheckLatestVersion(currentVersion string) (*UpdateInfo, error) {
	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Get(apiBase + "/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latest,
		ReleaseNotes:   release.Body,
		HTMLURL:        release.HTMLURL,
		IsNewer:        IsNewer(latest, currentVersion),
	}, nil
}

// FetchReleaseNotes fetches release notes for a specific version (or latest if empty)
func FetchReleaseNotes(version string) (string, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	url := apiBase + "/releases/latest"
	if version != "" {
		v := version
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}
		url = apiBase + "/releases/tags/" + v
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch release notes: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("release not found (status %d)", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to parse release: %w", err)
	}

	return release.Body, release.HTMLURL, nil
}

// IsNewer returns true if latest version is newer than current
func IsNewer(latest, current string) bool {
	latestParts := parseVersion(latest)
	currentParts := parseVersion(current)

	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release suffix (e.g., "1.0.0-beta")
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

// DetectInstallationMethod detects how octrafic was installed
func DetectInstallationMethod() InstallMethod {
	execPath, err := os.Executable()
	if err != nil {
		return MethodUnknown
	}

	switch runtime.GOOS {
	case "darwin":
		// Check for Homebrew
		if strings.Contains(execPath, "/Cellar/octrafic") || strings.Contains(execPath, "/opt/homebrew") {
			return MethodHomebrew
		}
		// Default to script for macOS
		return MethodScript

	case "linux":
		// Check for Homebrew on Linux
		if strings.Contains(execPath, "/.linuxbrew/") || strings.Contains(execPath, "/home/linuxbrew") {
			return MethodHomebrew
		}

		// Check for yay/paru (Arch)
		if cmdExists("pacman") {
			if out, err := exec.Command("pacman", "-Qi", "octrafic-bin").CombinedOutput(); err == nil && len(out) > 0 {
				// Check which AUR helper was likely used
				if cmdExists("yay") {
					return MethodYay
				}
				if cmdExists("paru") {
					return MethodParu
				}
			}
		}

		// Check for deb package
		if cmdExists("dpkg") {
			if out, err := exec.Command("dpkg", "-l", "octrafic").CombinedOutput(); err == nil && strings.Contains(string(out), "octrafic") {
				return MethodDeb
			}
		}

		// Check for rpm package
		if cmdExists("rpm") {
			if out, err := exec.Command("rpm", "-q", "octrafic").CombinedOutput(); err == nil && !strings.Contains(string(out), "not installed") {
				return MethodRPM
			}
		}

		// Default to script for Linux
		return MethodScript

	case "windows":
		// Default to script for Windows
		return MethodScript

	default:
		return MethodUnknown
	}
}

// PerformUpdate performs the update using the detected installation method
func PerformUpdate(currentVersion string) error {
	// Check if update is available
	info, err := CheckLatestVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !info.IsNewer {
		return fmt.Errorf("already on latest version (%s)", currentVersion)
	}

	method := DetectInstallationMethod()
	fmt.Printf("Detected installation method: %s\n", method)
	fmt.Printf("Updating from %s to %s...\n", currentVersion, info.LatestVersion)

	switch method {
	case MethodHomebrew:
		return updateHomebrew()
	case MethodYay:
		return updateYay()
	case MethodParu:
		return updateParu()
	case MethodDeb, MethodRPM:
		return updateScript()
	case MethodScript, MethodManual, MethodUnknown:
		return updateBinary(info.LatestVersion)
	default:
		return fmt.Errorf("unknown installation method. Please update manually")
	}
}

func cmdExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func updateHomebrew() error {
	fmt.Println("Updating via Homebrew...")
	cmd := exec.Command("brew", "upgrade", "octrafic")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func updateYay() error {
	fmt.Println("Updating via yay...")
	cmd := exec.Command("yay", "-Syu", "octrafic-bin", "--noconfirm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func updateParu() error {
	fmt.Println("Updating via paru...")
	cmd := exec.Command("paru", "-Syu", "octrafic-bin", "--noconfirm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// updateScript runs the installation script to update octrafic
func updateScript() error {
	fmt.Println("Updating via installation script...")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		script := "iex (iwr -useb https://octrafic.com/install.ps1)"
		cmd = exec.Command("powershell", "-Command", script)
	default:
		script := "curl -fsSL https://octrafic.com/install.sh | bash"
		cmd = exec.Command("bash", "-c", script)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// updateBinary downloads the latest release and replaces the current binary
func updateBinary(version string) error {
	fmt.Println("Downloading and installing new binary...")

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	downloadURL, archiveName := getReleaseURL(version)
	fmt.Printf("Downloading %s...\n", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download release: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "octrafic-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to download: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	binaryPath, err := extractBinary(tmpFile.Name(), archiveName)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}
	defer func() { _ = os.Remove(binaryPath) }()

	fmt.Printf("Installing to %s...\n", execPath)

	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := copyFile(binaryPath, execPath); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	if err := os.Chmod(execPath, 0755); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	_ = os.Remove(backupPath)

	return nil
}

func getReleaseURL(version string) (string, string) {
	return buildReleaseURL(version, runtime.GOOS, runtime.GOARCH)
}

func buildReleaseURL(version, goos, goarch string) (string, string) {
	baseURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/", repoOwner, repoName, version)

	var archiveName string
	switch goos {
	case "windows":
		archiveName = "octrafic_Windows_x86_64.zip"
	case "darwin":
		if goarch == "arm64" {
			archiveName = "octrafic_Darwin_arm64.tar.gz"
		} else {
			archiveName = "octrafic_Darwin_x86_64.tar.gz"
		}
	case "linux":
		if goarch == "arm64" {
			archiveName = "octrafic_Linux_arm64.tar.gz"
		} else {
			archiveName = "octrafic_Linux_x86_64.tar.gz"
		}
	default:
		archiveName = "octrafic_Linux_x86_64.tar.gz"
	}

	return baseURL + archiveName, archiveName
}

func extractBinary(archivePath, archiveName string) (string, error) {
	if strings.HasSuffix(archiveName, ".zip") {
		return extractZip(archivePath)
	}
	return extractTarGz(archivePath)
}

func extractZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name == "octrafic.exe" || f.Name == "octrafic" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer func() { _ = rc.Close() }()

			tmpBinary, err := os.CreateTemp("", "octrafic-binary-*")
			if err != nil {
				return "", err
			}
			defer func() { _ = tmpBinary.Close() }()

			_, err = io.Copy(tmpBinary, rc)
			if err != nil {
				_ = os.Remove(tmpBinary.Name())
				return "", err
			}

			return tmpBinary.Name(), nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

func extractTarGz(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Name == "octrafic" || header.Name == "./octrafic" {
			tmpBinary, err := os.CreateTemp("", "octrafic-binary-*")
			if err != nil {
				return "", err
			}
			defer func() { _ = tmpBinary.Close() }()

			_, err = io.Copy(tmpBinary, tr)
			if err != nil {
				_ = os.Remove(tmpBinary.Name())
				return "", err
			}

			return tmpBinary.Name(), nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()

	_, err = io.Copy(destination, source)
	return err
}
