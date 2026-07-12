package providers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// newTestHashiCorp returns a hashiCorp source backed by an httptest server
// serving the given index.json payloads keyed by request path.
func newTestHashiCorp(t *testing.T, responses map[string]string) *hashiCorp {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &hashiCorp{client: srv.Client(), repo: "terraform", baseURL: baseURL}
}

const terraformRepoIndex = `{
	"name": "terraform",
	"versions": {
		"1.5.0": {"name": "terraform", "version": "1.5.0"},
		"1.5.7": {"name": "terraform", "version": "1.5.7"},
		"1.6.0-beta1": {"name": "terraform", "version": "1.6.0-beta1"}
	}
}`

const terraformRelease157 = `{
	"name": "terraform",
	"version": "1.5.7",
	"builds": [
		{"os": "linux", "arch": "amd64", "filename": "terraform_1.5.7_linux_amd64.zip", "url": "https://releases.hashicorp.com/terraform/1.5.7/terraform_1.5.7_linux_amd64.zip"},
		{"os": "darwin", "arch": "arm64", "filename": "terraform_1.5.7_darwin_arm64.zip", "url": "https://releases.hashicorp.com/terraform/1.5.7/terraform_1.5.7_darwin_arm64.zip"}
	]
}`

const terraformRelease150 = `{
	"name": "terraform",
	"version": "1.5.0",
	"builds": [
		{"os": "linux", "arch": "amd64", "filename": "terraform_1.5.0_linux_amd64.zip", "url": "https://releases.hashicorp.com/terraform/1.5.0/terraform_1.5.0_linux_amd64.zip"}
	]
}`

func TestHashiCorpFetchReleaseLatest(t *testing.T) {
	// prereleases (1.6.0-beta1) must be skipped when picking the latest
	g := newTestHashiCorp(t, map[string]string{
		"/terraform/index.json":       terraformRepoIndex,
		"/terraform/1.5.7/index.json": terraformRelease157,
	})

	version, candidates, err := g.fetchRelease("")
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.5.7" {
		t.Errorf("version = %q, want 1.5.7", version)
	}
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(candidates))
	}
	if candidates[0].Name != "terraform_1.5.7_linux_amd64.zip" {
		t.Errorf("candidate name = %q, want terraform_1.5.7_linux_amd64.zip", candidates[0].Name)
	}
}

func TestHashiCorpFetchReleaseSpecificVersion(t *testing.T) {
	g := newTestHashiCorp(t, map[string]string{
		"/terraform/1.5.0/index.json": terraformRelease150,
	})

	version, candidates, err := g.fetchRelease("1.5.0")
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.5.0" {
		t.Errorf("version = %q, want 1.5.0", version)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}
}

func TestHashiCorpLatestVersionURL(t *testing.T) {
	g := newTestHashiCorp(t, map[string]string{
		"/terraform/index.json":       terraformRepoIndex,
		"/terraform/1.5.7/index.json": terraformRelease157,
	})

	version, versionURL, err := g.latestVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.5.7" {
		t.Errorf("version = %q, want 1.5.7", version)
	}
	want := g.baseURL.String() + "/terraform/1.5.7/index.json"
	if versionURL != want {
		t.Errorf("url = %q, want %q", versionURL, want)
	}
}

// TestHashiCorpProviderURLRoundTrip ensures the URL shape returned by
// latestVersion resolves back to the hashicorp provider with the same tag,
// since the update command re-instantiates providers from that URL.
func TestHashiCorpProviderURLRoundTrip(t *testing.T) {
	p, err := New("https://releases.hashicorp.com/terraform/1.5.7/index.json", "")
	if err != nil {
		t.Fatal(err)
	}
	hp, ok := p.(*httpReleaseProvider)
	if !ok || hp.GetID() != "hashicorp" {
		t.Fatalf("got %T (%s), want hashicorp httpReleaseProvider", p, p.GetID())
	}
	if hp.tag != "1.5.7" {
		t.Errorf("tag = %q, want 1.5.7", hp.tag)
	}
}
