package providers

import (
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
	u, _ := url.Parse("https://github.com/helm/helm/releases/tag/v3.16.3")
	p, err := newHelm(u)
	if err != nil {
		t.Fatal(err)
	}
	h := p.(*helm)
	if h.tag != "v3.16.3" {
		t.Errorf("tag = %q, want v3.16.3", h.tag)
	}
}

func TestHelmCandidates(t *testing.T) {
	u, _ := url.Parse("https://get.helm.sh")
	p, _ := newHelm(u)
	h := p.(*helm)

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
