package providers

import (
	"net/url"
	"testing"
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
