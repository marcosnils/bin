package providers

import (
	"net/url"
	"testing"
)

func TestNewGitLab(t *testing.T) {
	cases := []struct {
		name          string
		rawURL        string
		expectedOwner string
		expectedRepo  string
		expectedTag   string
		wantErr       bool
	}{
		{name: "owner/repo", rawURL: "https://gitlab.com/owner/repo", expectedOwner: "owner", expectedRepo: "repo"},
		{name: "releases tag", rawURL: "https://gitlab.com/owner/repo/releases/v1.0", expectedOwner: "owner", expectedRepo: "repo", expectedTag: "v1.0"},
		{name: "too few segments", rawURL: "https://gitlab.com/owner", wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			u, err := url.Parse(test.rawURL)
			if err != nil {
				t.Fatalf("url.Parse failed: %v", err)
			}
			p, err := newGitLab(u)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gl, ok := p.(*gitLab)
			if !ok {
				t.Fatalf("expected *gitLab, got %T", p)
			}
			if gl.owner != test.expectedOwner {
				t.Errorf("expected owner %q, got %q", test.expectedOwner, gl.owner)
			}
			if gl.repo != test.expectedRepo {
				t.Errorf("expected repo %q, got %q", test.expectedRepo, gl.repo)
			}
			if gl.tag != test.expectedTag {
				t.Errorf("expected tag %q, got %q", test.expectedTag, gl.tag)
			}
			if gl.GetID() != "gitlab" {
				t.Errorf("expected id gitlab, got %s", gl.GetID())
			}
		})
	}
}
