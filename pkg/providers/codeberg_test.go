package providers

import (
	"net/url"
	"testing"
)

func TestCodebergProviderDetection(t *testing.T) {
	tests := []struct {
		name     string
		urlStr   string
		provider string
		wantErr  bool
	}{
		{
			name:     "codeberg.org URL",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf",
			provider: "",
			wantErr:  false,
		},
		{
			name:     "codeberg.org with explicit provider",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf",
			provider: "codeberg",
			wantErr:  false,
		},
		{
			name:     "codeberg.org release URL",
			urlStr:   "https://codeberg.org/mergiraf/mergiraf/releases/tag/v1.0.0",
			provider: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.urlStr, tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && p == nil {
				t.Error("Expected provider to be created, got nil")
				return
			}
			if !tt.wantErr && p.GetID() != "codeberg" {
				t.Errorf("Expected provider ID 'codeberg', got '%s'", p.GetID())
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

			p, err := newCodeberg(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("newCodeberg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			cb, ok := p.(*codeberg)
			if !ok {
				t.Fatal("Expected provider to be of type *codeberg")
			}

			if cb.owner != tt.wantOwner {
				t.Errorf("owner = %v, want %v", cb.owner, tt.wantOwner)
			}
			if cb.repo != tt.wantRepo {
				t.Errorf("repo = %v, want %v", cb.repo, tt.wantRepo)
			}
			if cb.tag != tt.wantTag {
				t.Errorf("tag = %v, want %v", cb.tag, tt.wantTag)
			}
		})
	}
}
