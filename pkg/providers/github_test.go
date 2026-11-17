package providers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/marcosnils/bin/pkg/httpclient"
)

func TestNewGitHub(t *testing.T) {
	cases := []struct {
		name          string
		rawURL        string
		expectedOwner string
		expectedRepo  string
		expectedTag   string
		expectFilter  string
		wantErr       bool
	}{
		{name: "owner/repo", rawURL: "https://github.com/owner/repo", expectedOwner: "owner", expectedRepo: "repo"},
		{name: "releases tag", rawURL: "https://github.com/owner/repo/releases/tag/v1.0", expectedOwner: "owner", expectedRepo: "repo", expectedTag: "v1.0"},
		{name: "releases download", rawURL: "https://github.com/owner/repo/releases/download/v1.0", expectedOwner: "owner", expectedRepo: "repo", expectedTag: "v1.0"},
		{name: "filter", rawURL: "https://github.com/owner/repo?filter=*.tar.gz", expectedOwner: "owner", expectedRepo: "repo", expectFilter: "*.tar.gz"},
		{name: "invalid filter", rawURL: "https://github.com/owner/repo?filter=%5B", wantErr: true},
		{name: "too few segments", rawURL: "https://github.com/owner", wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			u, err := url.Parse(test.rawURL)
			if err != nil {
				t.Fatalf("url.Parse failed: %v", err)
			}
			p, err := newGitHub(u)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gh, ok := p.(*gitHub)
			if !ok {
				t.Fatalf("expected *gitHub, got %T", p)
			}
			if gh.owner != test.expectedOwner {
				t.Errorf("expected owner %q, got %q", test.expectedOwner, gh.owner)
			}
			if gh.repo != test.expectedRepo {
				t.Errorf("expected repo %q, got %q", test.expectedRepo, gh.repo)
			}
			if gh.tag != test.expectedTag {
				t.Errorf("expected tag %q, got %q", test.expectedTag, gh.tag)
			}
			if gh.filter != test.expectFilter {
				t.Errorf("expected filter %q, got %q", test.expectFilter, gh.filter)
			}
			if test.expectFilter != "" {
				if gh.url.RawQuery != "" && gh.url.Query().Get("filter") != "" {
					t.Errorf("expected filter removed from query, got %q", gh.url.RawQuery)
				}
			}
			if gh.GetID() != "github" {
				t.Errorf("expected id github, got %s", gh.GetID())
			}
		})
	}
}

// TestGitHubWithFilter verifies that the URL returned for a filtered release
// keeps the ?filter= parameter, so `bin update` doesn't lose it after the
// first upgrade (https://github.com/marcosnils/bin/issues/302).
func TestGitHubWithFilter(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		in     string
		want   string
	}{
		{name: "no filter", in: "https://github.com/o/r/releases/tag/v1.0", want: "https://github.com/o/r/releases/tag/v1.0"},
		{name: "filter", filter: "sessfind-v*", in: "https://github.com/o/r/releases/tag/sessfind-v0.9.2", want: "https://github.com/o/r/releases/tag/sessfind-v0.9.2?filter=sessfind-v%2A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := &gitHub{filter: c.filter}
			if got := g.withFilter(c.in); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			// the produced URL must round-trip through newGitHub with the same filter
			u, err := url.Parse(g.withFilter(c.in))
			if err != nil {
				t.Fatal(err)
			}
			p, err := newGitHub(u)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.(*gitHub).filter; got != c.filter {
				t.Fatalf("round-trip filter: got %q, want %q", got, c.filter)
			}
		})
	}
}

type headerRecorder struct{ auth []string }

func (h *headerRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	h.auth = append(h.auth, r.Header.Get("Authorization"))
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(`{"message":"Not Found"}`)),
		Header:     http.Header{"Content-Type": {"application/json"}},
		Request:    r,
	}, nil
}

func TestGitHubAuthToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh is a shell script")
	}
	cases := []struct {
		name       string
		env        map[string]string
		gh         string // fake gh script body; "" means gh is not on PATH
		wantAuth   string
		wantGHArgs string // "" means gh must not run
	}{
		{
			name:       "gh logged in",
			gh:         "echo gho_fromgh",
			wantAuth:   "Bearer gho_fromgh",
			wantGHArgs: "auth token --hostname github.com",
		},
		{
			name:     "GITHUB_AUTH_TOKEN wins over gh",
			env:      map[string]string{"GITHUB_AUTH_TOKEN": "fromenv"},
			gh:       "echo gho_fromgh",
			wantAuth: "Bearer fromenv",
		},
		{
			name:     "GITHUB_TOKEN wins over gh",
			env:      map[string]string{"GITHUB_TOKEN": "fromenv"},
			gh:       "echo gho_fromgh",
			wantAuth: "Bearer fromenv",
		},
		{
			name:       "gh not logged in",
			gh:         "echo 'no oauth token found for github.com' >&2; exit 1",
			wantGHArgs: "auth token --hostname github.com",
		},
		{
			name: "gh not installed",
		},
		{
			name: "GHES does not use gh's github.com token",
			env: map[string]string{
				"GHES_BASE_URL":   "https://ghes.example.com/api/v3/",
				"GHES_UPLOAD_URL": "https://ghes.example.com/api/uploads/",
				"GHES_AUTH_TOKEN": "ghes",
			},
			gh:       "echo gho_fromgh",
			wantAuth: "Bearer ghes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"GITHUB_AUTH_TOKEN", "GITHUB_TOKEN", "GHES_BASE_URL", "GHES_UPLOAD_URL", "GHES_AUTH_TOKEN"} {
				t.Setenv(k, tc.env[k])
			}
			dir := t.TempDir()
			argsFile := filepath.Join(dir, "gh-args")
			if tc.gh != "" {
				script := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %q\n%s\n", argsFile, tc.gh)
				if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir)
			ghCLIToken = sync.OnceValue(readGHCLIToken)

			rec := &headerRecorder{}
			// unauthenticated go-github clients use http.DefaultTransport
			origBin, origDefault := httpclient.Client.Transport, http.DefaultTransport
			httpclient.Client.Transport, http.DefaultTransport = rec, rec
			t.Cleanup(func() {
				httpclient.Client.Transport, http.DefaultTransport = origBin, origDefault
			})

			u, _ := url.Parse("https://github.com/owner/repo")
			p, err := newGitHub(u)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Fetch(&FetchOpts{}); err == nil {
				t.Fatal("expected Fetch to fail against the 404 transport")
			}

			if len(rec.auth) == 0 {
				t.Fatal("no request reached the transport")
			}
			if rec.auth[0] != tc.wantAuth {
				t.Errorf("Authorization: want %q, got %q", tc.wantAuth, rec.auth[0])
			}
			gotArgs, _ := os.ReadFile(argsFile)
			if got := strings.TrimSpace(string(gotArgs)); got != tc.wantGHArgs {
				t.Errorf("gh args: want %q, got %q", tc.wantGHArgs, got)
			}
		})
	}
}
