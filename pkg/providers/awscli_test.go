package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAWSCLI(t *testing.T) {
	cases := []struct {
		name            string
		url             string
		expectedVersion string
	}{
		{name: "no version", url: "awscli://", expectedVersion: ""},
		{name: "explicit latest", url: "awscli://latest", expectedVersion: ""},
		{name: "with version", url: "awscli://2.15.0", expectedVersion: "2.15.0"},
		{name: "with version spaces", url: "awscli://  2.15.0  ", expectedVersion: "2.15.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newAWSCLI(tc.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			a := p.(*awsCLI)
			if a.version != tc.expectedVersion {
				t.Errorf("expected version %q, got %q", tc.expectedVersion, a.version)
			}
		})
	}
}

func TestAWSCLIGetID(t *testing.T) {
	p, _ := newAWSCLI("awscli://")
	if p.GetID() != "awscli" {
		t.Errorf("expected ID 'awscli', got '%s'", p.GetID())
	}
}

func TestBuildDownloadURL(t *testing.T) {
	cases := []struct {
		name     string
		version  string
		goos     string
		goarch   string
		expected string
		wantErr  bool
	}{
		{
			name:     "linux amd64 latest",
			version:  "",
			goos:     "linux",
			goarch:   "amd64",
			expected: "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip",
		},
		{
			name:     "linux amd64 versioned",
			version:  "2.15.0",
			goos:     "linux",
			goarch:   "amd64",
			expected: "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.15.0.zip",
		},
		{
			name:     "linux arm64 latest",
			version:  "",
			goos:     "linux",
			goarch:   "arm64",
			expected: "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip",
		},
		{
			name:     "linux arm64 versioned",
			version:  "2.0.30",
			goos:     "linux",
			goarch:   "arm64",
			expected: "https://awscli.amazonaws.com/awscli-exe-linux-aarch64-2.0.30.zip",
		},
		{
			name:     "darwin latest",
			version:  "",
			goos:     "darwin",
			goarch:   "arm64",
			expected: "https://awscli.amazonaws.com/AWSCLIV2.pkg",
		},
		{
			name:     "darwin versioned",
			version:  "2.15.0",
			goos:     "darwin",
			goarch:   "amd64",
			expected: "https://awscli.amazonaws.com/AWSCLIV2-2.15.0.pkg",
		},
		{
			name:    "unsupported os",
			version: "",
			goos:    "windows",
			goarch:  "amd64",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildDownloadURL(tc.version, tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	tags := []struct {
		Name string `json:"name"`
	}{
		{Name: "2.15.0"},
		{Name: "2.14.6"},
		{Name: "1.32.0"},
		{Name: "2.13.0"},
		{Name: "invalid-tag"},
		{Name: "2.15.1"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tags)
	}))
	defer server.Close()

	a := &awsCLI{
		version: "",
		tagsURL: server.URL,
	}

	ver, url, err := a.GetLatestVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "2.15.1" {
		t.Errorf("expected version '2.15.1', got '%s'", ver)
	}
	if url != "awscli://2.15.1" {
		t.Errorf("expected url 'awscli://2.15.1', got '%s'", url)
	}
}

func TestGetLatestVersionNoV2Tags(t *testing.T) {
	tags := []struct {
		Name string `json:"name"`
	}{
		{Name: "1.32.0"},
		{Name: "1.31.0"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tags)
	}))
	defer server.Close()

	a := &awsCLI{
		version: "",
		tagsURL: server.URL,
	}

	_, _, err := a.GetLatestVersion()
	if err == nil {
		t.Error("expected error for no v2 tags, got nil")
	}
}

func TestGetLatestVersionRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	a := &awsCLI{
		version: "",
		tagsURL: server.URL,
	}

	_, _, err := a.GetLatestVersion()
	if err == nil {
		t.Error("expected error for rate limit, got nil")
	}
}
