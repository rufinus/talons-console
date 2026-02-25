package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setTestURL overrides the GitHub releases URL for testing.
func setTestURL(url string) { githubReleasesURL = url }

// helper to build a mock GitHub releases response
func mockRelease(tagName, htmlURL string) []byte {
	b, _ := json.Marshal(map[string]string{
		"tag_name": tagName,
		"html_url": htmlURL,
	})
	return b
}

// saveVersion saves the current Version and restores it after the test.
func withVersion(t *testing.T, v string) func() {
	t.Helper()
	orig := Version
	Version = v
	return func() { Version = orig }
}

// TestCheckUpdate_NewerAvailable: remote has a newer release → UpToDate=false.
func TestCheckUpdate_NewerAvailable(t *testing.T) {
	defer withVersion(t, "1.0.0")()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockRelease("v2.0.0", "https://github.com/openclaw/talons/releases/tag/v2.0.0"))
	}))
	defer ts.Close()

	// Temporarily override the URL for the test.
	origURL := githubReleasesURL
	setTestURL(ts.URL)
	defer setTestURL(origURL)

	info, err := CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpToDate {
		t.Errorf("expected UpToDate=false, got true (current=%s latest=%s)", info.Current, info.Latest)
	}
	if info.Latest != "2.0.0" {
		t.Errorf("expected latest=2.0.0, got %s", info.Latest)
	}
	if info.Current != "1.0.0" {
		t.Errorf("expected current=1.0.0, got %s", info.Current)
	}
}

// TestCheckUpdate_UpToDate: remote has the same version → UpToDate=true.
func TestCheckUpdate_UpToDate(t *testing.T) {
	defer withVersion(t, "1.2.3")()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockRelease("v1.2.3", "https://github.com/openclaw/talons/releases/tag/v1.2.3"))
	}))
	defer ts.Close()

	origURL := githubReleasesURL
	setTestURL(ts.URL)
	defer setTestURL(origURL)

	info, err := CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.UpToDate {
		t.Errorf("expected UpToDate=true, got false (current=%s latest=%s)", info.Current, info.Latest)
	}
}

// TestCheckUpdate_NetworkError: closed server → returns error.
func TestCheckUpdate_NetworkError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // close immediately so the request fails

	origURL := githubReleasesURL
	setTestURL(ts.URL)
	defer setTestURL(origURL)

	_, err := CheckUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error from closed server, got nil")
	}
}

// TestCheckUpdate_APIError: server returns 403 → returns error.
func TestCheckUpdate_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	origURL := githubReleasesURL
	setTestURL(ts.URL)
	defer setTestURL(origURL)

	_, err := CheckUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to mention 403, got: %v", err)
	}
}

// TestCheckUpdate_InvalidVersion: server returns non-semver tag → handles gracefully.
func TestCheckUpdate_InvalidVersion(t *testing.T) {
	defer withVersion(t, "1.0.0")()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockRelease("not-a-version", "https://example.com"))
	}))
	defer ts.Close()

	origURL := githubReleasesURL
	setTestURL(ts.URL)
	defer setTestURL(origURL)

	info, err := CheckUpdate(context.Background())
	// Should not error — just produce a result with latest="not-a-version".
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Latest != "not-a-version" {
		t.Errorf("expected latest=not-a-version, got %s", info.Latest)
	}
	// "not-a-version" parses to [0,0,0], "1.0.0" parses to [1,0,0] → current > latest → UpToDate=true
	if !info.UpToDate {
		t.Errorf("expected UpToDate=true for non-parseable latest, got false")
	}
}

// TestCompareVersions: table-driven tests for version comparison logic.
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want string // "gt", "eq", "lt"
	}{
		{"1.0.0", "1.0.0", "eq"},
		{"2.0.0", "1.0.0", "gt"},
		{"1.0.0", "2.0.0", "lt"},
		{"1.2.3", "1.2.4", "lt"},
		{"1.2.4", "1.2.3", "gt"},
		{"1.10.0", "1.9.0", "gt"},
		{"v1.0.0", "v1.0.0", "eq"},
		{"v2.0.0", "v1.9.9", "gt"},
		{"1.0.0-beta", "1.0.0", "eq"},  // pre-release stripped, equal
		{"1.0.0+build", "1.0.0", "eq"}, // build metadata stripped
		{"dev", "1.0.0", "lt"},         // non-parseable → 0.0.0 < 1.0.0
		{"0.0.0", "0.0.0", "eq"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tc.a, tc.b), func(t *testing.T) {
			got := compareVersions(tc.a, tc.b)
			var got_s string
			switch {
			case got > 0:
				got_s = "gt"
			case got < 0:
				got_s = "lt"
			default:
				got_s = "eq"
			}
			if got_s != tc.want {
				t.Errorf("compareVersions(%q, %q) = %d (%s), want %s", tc.a, tc.b, got, got_s, tc.want)
			}
		})
	}
}

// TestVersionString: String() returns the expected formatted output.
func TestVersionString(t *testing.T) {
	origV, origC, origD := Version, Commit, BuildDate
	defer func() { Version, Commit, BuildDate = origV, origC, origD }()

	Version = "1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-02-25"

	got := String()
	want := "talons 1.2.3 (commit: abc1234, built: 2026-02-25)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
