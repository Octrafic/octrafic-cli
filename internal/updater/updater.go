package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "Octrafic"
	repoName  = "octrafic-cli"
	apiBase   = "https://api.github.com/repos/" + repoOwner + "/" + repoName
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
