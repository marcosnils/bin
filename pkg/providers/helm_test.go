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
		{"github.com/helm/helm", ""},
		{"https://github.com/helm/helm/releases/tag/v3.16.3", ""},
		{"get.helm.sh", ""},
		{"github.com/helm/helm", "helm"},
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

func TestHelmProviderDoesNotHijackOtherRepos(t *testing.T) {
	// Repos merely prefixed with "helm/helm" must not resolve to the helm
	// provider (e.g. helm/helm-mapkubeapis ships assets on GitHub).
	p, err := New("github.com/helm/helm-mapkubeapis", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetID() == "helm" {
		t.Errorf("helm/helm-mapkubeapis incorrectly routed to helm provider")
	}
}

func TestHelmTagExtraction(t *testing.T) {
	cases := map[string]string{
		"https://github.com/helm/helm/releases/tag/v3.16.3": "v3.16.3",
		// tags are normalized to carry the leading "v"
		"https://github.com/helm/helm/releases/tag/3.16.3": "v3.16.3",
		"https://github.com/helm/helm":                     "",
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
	u := fmt.Sprintf("%s/%s", helmReleasesURL, "v4.2.2")
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
