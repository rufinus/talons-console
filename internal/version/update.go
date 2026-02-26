package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// githubReleasesURL is the GitHub Releases API endpoint. Variable so tests can override.
var githubReleasesURL = "https://api.github.com/repos/openclaw/talons/releases/latest" //nolint:gochecknoglobals

// UpdateInfo contains the result of an update check.
type UpdateInfo struct {
	Current     string
	Latest      string
	UpToDate    bool
	DownloadURL string
}

// CheckUpdate queries GitHub for the latest release and compares it to the
// currently installed version. Returns an error if the network request fails
// or if the GitHub API returns a non-200 status.
func CheckUpdate(ctx context.Context) (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("talons/%s", Version))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update check failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	info := &UpdateInfo{
		Current:     Version,
		Latest:      latest,
		DownloadURL: release.HTMLURL,
		UpToDate:    compareVersions(Version, latest) >= 0,
	}
	return info, nil
}

// compareVersions returns >0 if a>b, 0 if equal, <0 if a<b.
func compareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] - pb[i]
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	// strip pre-release and build-metadata suffixes
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}
