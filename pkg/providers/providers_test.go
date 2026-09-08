package providers

import (
	"testing"
)

func TestNew(t *testing.T) {
	cases := []struct {
		name       string
		u          string
		provider   string
		expectedID string
		wantErr    bool
		mayNetwork bool
	}{
		{name: "docker", u: "docker://postgres:1.2.3", expectedID: "docker"},
		{name: "goinstall scheme", u: "goinstall://github.com/foo/bar@v1.0", expectedID: "goinstall"},
		{name: "goinstall hint", u: "github.com/foo/bar@v1.0", provider: "goinstall", expectedID: "goinstall"},
		{name: "github bare host", u: "github.com/owner/repo", expectedID: "github"},
		{name: "github https", u: "https://github.com/owner/repo", expectedID: "github"},
		{name: "github hint", u: "example.com/owner/repo", provider: "github", expectedID: "github"},
		{name: "gitlab host", u: "gitlab.com/owner/repo", expectedID: "gitlab"},
		{name: "gitlab hint", u: "example.com/owner/repo", provider: "gitlab", expectedID: "gitlab"},
		{name: "codeberg host", u: "codeberg.org/owner/repo", expectedID: "codeberg", mayNetwork: true},
		{name: "unknown host", u: "example.com/owner/repo", wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p, err := New(test.u, test.provider)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (provider=%v)", p)
				}
				return
			}
			if err != nil {
				if test.mayNetwork {
					t.Skipf("%s client requires network: %v", test.name, err)
				}
				t.Fatalf("unexpected error: %v", err)
			}
			if p.GetID() != test.expectedID {
				t.Errorf("expected id %s, got %s", test.expectedID, p.GetID())
			}
		})
	}
}
