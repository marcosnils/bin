package providers

import (
	"fmt"
	"net/url"
	"testing"
)

func TestHelmProviderRouting(t *testing.T) {
	cases := []struct {
		url      string
		provider string
	}{
		{"get.helm.sh", ""},
		{"https://get.helm.sh/v3.16.3", ""},
		{"https://get.helm.sh/helm-v3.16.3-linux-amd64.tar.gz", ""},
		{"get.helm.sh", "helm"},
	}
	for _, c := range cases {
		p, err := New(c.url, c.provider)
		if err != nil {
			t.Fatalf("New(%q, %q) returned error: %v", c.url, c.provider, err)
		}
		if p.GetID() != "helm" {
			t.Errorf("New(%q, %q) = provider %q, want helm", c.url, c.provider, p.GetID())
		}
	}
}

func TestHelmProviderDoesNotHijackGitHub(t *testing.T) {
	// GitHub URLs stay on the github provider; only get.helm.sh (or an
	// explicit --provider helm) selects the helm provider.
	p, err := New("github.com/helm/helm", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetID() != "github" {
		t.Errorf("github.com/helm/helm routed to %q, want github", p.GetID())
	}
}

func TestHelmTagExtraction(t *testing.T) {
	cases := map[string]string{
		"https://get.helm.sh/v3.16.3": "v3.16.3",
		// tags are normalized to carry the leading "v"
		"https://get.helm.sh/3.16.3": "v3.16.3",
		// release download URLs pin their version, whatever the platform
		"https://get.helm.sh/helm-v3.16.3-linux-amd64.tar.gz":      "v3.16.3",
		"https://get.helm.sh/helm-v3.16.3-windows-amd64.zip":       "v3.16.3",
		"https://get.helm.sh/helm-v3.17.0-rc.1-linux-arm64.tar.gz": "v3.17.0-rc.1",
		"https://get.helm.sh":                     "",
		"https://get.helm.sh/helm-latest-version": "",
	}
	for in, want := range cases {
		u, _ := url.Parse(in)
		if got := parseHelmTag(u); got != want {
			t.Errorf("parseHelmTag(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHelmLatestVersionURLRoundTrip ensures the URL shape returned by
// latestVersion resolves back to the helm provider with the same tag, since
// the update command re-instantiates providers from that URL.
func TestHelmLatestVersionURLRoundTrip(t *testing.T) {
	u := fmt.Sprintf("%s/%s", helmDownloadBase, "v4.2.2")
	p, err := New(u, "")
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", u, err)
	}
	hp, ok := p.(*httpReleaseProvider)
	if !ok || hp.GetID() != "helm" {
		t.Fatalf("New(%q) = %T (%s), want helm httpReleaseProvider", u, p, p.GetID())
	}
	if hp.tag != "v4.2.2" {
		t.Errorf("tag = %q, want v4.2.2", hp.tag)
	}
}

func TestHelmCandidates(t *testing.T) {
	h := &helm{}

	cs := h.candidates("v3.16.3")
	if len(cs) != len(helmPlatforms) {
		t.Fatalf("got %d candidates, want %d", len(cs), len(helmPlatforms))
	}

	want := "https://get.helm.sh/helm-v3.16.3-linux-amd64.tar.gz"
	found := false
	for _, c := range cs {
		if c.URL == want {
			found = true
		}
	}
	if !found {
		t.Errorf("candidates missing %q", want)
	}
}

func TestNormalizeHelmVersion(t *testing.T) {
	cases := map[string]string{
		"3.16.3":  "v3.16.3",
		"v3.16.3": "v3.16.3",
		"":        "",
	}
	for in, want := range cases {
		if got := normalizeHelmVersion(in); got != want {
			t.Errorf("normalizeHelmVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
