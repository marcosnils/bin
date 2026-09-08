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
