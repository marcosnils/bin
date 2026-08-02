package providers

import (
	"net/url"
	"testing"
)

func TestNewCodeberg(t *testing.T) {
	cases := []struct {
		name          string
		rawURL        string
		expectedOwner string
		expectedRepo  string
		expectedTag   string
		wantErr       bool
	}{
		{name: "owner/repo", rawURL: "https://codeberg.org/owner/repo", expectedOwner: "owner", expectedRepo: "repo"},
		{name: "releases tag", rawURL: "https://codeberg.org/owner/repo/releases/tag/v1.0", expectedOwner: "owner", expectedRepo: "repo", expectedTag: "v1.0"},
		{name: "too few segments", rawURL: "https://codeberg.org/owner", wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			u, err := url.Parse(test.rawURL)
			if err != nil {
				t.Fatalf("url.Parse failed: %v", err)
			}
			p, err := newCodeberg(u)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Skipf("codeberg client requires network: %v", err)
			}
			cb, ok := p.(*codeberg)
			if !ok {
				t.Fatalf("expected *codeberg, got %T", p)
			}
			if cb.owner != test.expectedOwner {
				t.Errorf("expected owner %q, got %q", test.expectedOwner, cb.owner)
			}
			if cb.repo != test.expectedRepo {
				t.Errorf("expected repo %q, got %q", test.expectedRepo, cb.repo)
			}
			if cb.tag != test.expectedTag {
				t.Errorf("expected tag %q, got %q", test.expectedTag, cb.tag)
			}
			if cb.GetID() != "codeberg" {
				t.Errorf("expected id codeberg, got %s", cb.GetID())
			}
		})
	}
}
