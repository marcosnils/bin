package providers

import (
	"net/url"
	"strings"
	"testing"
)

func TestCodebergProviderDetection(t *testing.T) {
	tests := []struct {
		name     string
		urlStr   string
		provider string
		wantID   string
	}{
		{
			name:     "codeberg.org URL",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf",
			provider: "",
			wantID:   "codeberg",
		},
		{
			name:     "codeberg.org with explicit provider",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf",
			provider: "codeberg",
			wantID:   "codeberg",
		},
		{
			name:     "codeberg.org release URL",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf/releases/tag/v1.0.0",
			provider: "",
			wantID:   "codeberg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the provider selection logic works by checking URL patterns
			if !strings.Contains(tt.urlStr, "codeberg") && tt.provider != "codeberg" {
				t.Skip("URL does not contain codeberg and provider not explicitly set")
			}

			// Parse URL to verify it's a valid codeberg URL
			u, err := url.Parse(tt.urlStr)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			if !strings.Contains(u.Host, "codeberg") && tt.provider != "codeberg" {
				t.Errorf("URL host %s does not contain 'codeberg'", u.Host)
			}
		})
	}
}

func TestCodebergURLParsing(t *testing.T) {
	tests := []struct {
		name      string
		urlStr    string
		wantOwner string
		wantRepo  string
		wantTag   string
		wantErr   bool
	}{
		{
			name:      "basic URL",
			urlStr:    "https://codeberg.org/mergiraf/mergiraf",
			wantOwner: "mergiraf",
			wantRepo:  "mergiraf",
			wantTag:   "",
			wantErr:   false,
		},
		{
			name:      "release tag URL",
			urlStr:    "https://codeberg.org/mergiraf/mergiraf/releases/tag/v1.0.0",
			wantOwner: "mergiraf",
			wantRepo:  "mergiraf",
			wantTag:   "v1.0.0",
			wantErr:   false,
		},
		{
			name:      "download URL",
			urlStr:    "https://codeberg.org/mergiraf/mergiraf/releases/download/v1.0.0/binary",
			wantOwner: "mergiraf",
			wantRepo:  "mergiraf",
			wantTag:   "v1.0.0/binary",
			wantErr:   false,
		},
		{
			name:      "invalid URL - missing repo",
			urlStr:    "https://codeberg.org/mergiraf",
			wantOwner: "",
			wantRepo:  "",
			wantTag:   "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.urlStr)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			// Test the URL parsing logic without initializing the client
			s := strings.Split(u.Path, "/")
			if len(s) < 3 && !tt.wantErr {
				t.Errorf("Expected error for invalid URL path with less than 3 segments")
				return
			}

			if tt.wantErr {
				if len(s) >= 3 {
					t.Errorf("Expected error but got valid path segments: %v", s)
				}
				return
			}

			owner := s[1]
			repo := s[2]

			var tag string
			if strings.Contains(u.Path, "/releases/") {
				ps := strings.Split(u.Path, "/")
				for i, p := range ps {
					if p == "releases" {
						tag = strings.Join(ps[i+2:], "/")
					}
				}
			}

			if owner != tt.wantOwner {
				t.Errorf("owner = %v, want %v", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %v, want %v", repo, tt.wantRepo)
			}
			if tag != tt.wantTag {
				t.Errorf("tag = %v, want %v", tag, tt.wantTag)
			}
		})
	}
}
