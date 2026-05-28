package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releaseAPIURL = "https://api.github.com/repos/basmulder03/git-projects-sync/releases/latest"

// ReleaseInfo holds metadata for a GitHub release.
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// CheckLatest fetches the latest release from GitHub and reports whether
// currentVersion is outdated. Dev builds always return false.
func CheckLatest(currentVersion string) (*ReleaseInfo, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, false, fmt.Errorf("decode release info: %w", err)
	}

	if currentVersion == "dev" {
		return &info, false, nil
	}

	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(info.TagName, "v")
	return &info, latest > current, nil
}
