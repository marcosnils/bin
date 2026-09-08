package providers

import (
	"testing"
)

func TestParseRepo(t *testing.T) {
	cases := []struct {
		name              string
		path              string
		expectedRepo      string
		expectedTag       string
		expectedName      string
		expectedLatestURL string
	}{
		{
			name:              "no tag",
			path:              "github.com/foo/bar",
			expectedRepo:      "github.com/foo/bar",
			expectedTag:       "latest",
			expectedName:      "bar",
			expectedLatestURL: "https://proxy.golang.org/github.com/foo/bar/@latest",
		},
		{
			name:              "with tag",
			path:              "github.com/foo/bar@v1.2.3",
			expectedRepo:      "github.com/foo/bar",
			expectedTag:       "v1.2.3",
			expectedName:      "bar",
			expectedLatestURL: "https://proxy.golang.org/github.com/foo/bar/@latest",
		},
		{
			name:              "subpackage with tag",
			path:              "github.com/foo/bar/cmd/baz@v1.0",
			expectedRepo:      "github.com/foo/bar/cmd/baz",
			expectedTag:       "v1.0",
			expectedName:      "baz",
			expectedLatestURL: "https://proxy.golang.org/github.com/foo/bar/cmd/baz/@latest",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repo, tag, name, latestURL := parseRepo(test.path)
			if repo != test.expectedRepo {
				t.Errorf("expected repo %q, got %q", test.expectedRepo, repo)
			}
			if tag != test.expectedTag {
				t.Errorf("expected tag %q, got %q", test.expectedTag, tag)
			}
			if name != test.expectedName {
				t.Errorf("expected name %q, got %q", test.expectedName, name)
			}
			if latestURL != test.expectedLatestURL {
				t.Errorf("expected latestURL %q, got %q", test.expectedLatestURL, latestURL)
			}
		})
	}
}

func TestNewGoInstall(t *testing.T) {
	p, err := newGoInstall("goinstall://github.com/foo/bar@v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gi, ok := p.(*goinstall)
	if !ok {
		t.Fatalf("expected *goinstall, got %T", p)
	}
	if gi.repo != "github.com/foo/bar" {
		t.Errorf("expected repo github.com/foo/bar, got %q", gi.repo)
	}
	if gi.tag != "v1.0" {
		t.Errorf("expected tag v1.0, got %q", gi.tag)
	}
	if gi.name != "bar" {
		t.Errorf("expected name bar, got %q", gi.name)
	}
	if gi.GetID() != "goinstall" {
		t.Errorf("expected id goinstall, got %s", gi.GetID())
	}
}
