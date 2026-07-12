package providers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/marcosnils/bin/pkg/httpclient"
)

const (
	helmDownloadBase  = "https://get.helm.sh"
	helmLatestVersion = "https://get.helm.sh/helm-latest-version"
)

// helmPlatform is a single os/arch combination published on get.helm.sh.
type helmPlatform struct {
	os   string
	arch string
	ext  string
}

// helmPlatforms enumerates the binaries Helm publishes for every release.
// Helm does not attach binaries to its GitHub releases (only signatures and
// checksums) and get.helm.sh has no listing API, so bin builds this static
// candidate list and scores it the same way it scores assets from the other
// providers to pick the one matching the running platform.
var helmPlatforms = []helmPlatform{
	{"darwin", "amd64", "tar.gz"},
	{"darwin", "arm64", "tar.gz"},
	{"linux", "386", "tar.gz"},
	{"linux", "amd64", "tar.gz"},
	{"linux", "arm", "tar.gz"},
	{"linux", "arm64", "tar.gz"},
	{"linux", "loong64", "tar.gz"},
	{"linux", "ppc64le", "tar.gz"},
	{"linux", "riscv64", "tar.gz"},
	{"linux", "s390x", "tar.gz"},
	{"windows", "amd64", "zip"},
	{"windows", "arm64", "zip"},
}

type helm struct {
	client *http.Client
}

// candidates builds the list of downloadable assets for a given version.
func (h *helm) candidates(version string) []*assets.Asset {
	cs := make([]*assets.Asset, 0, len(helmPlatforms))
	for _, p := range helmPlatforms {
		name := fmt.Sprintf("helm-%s-%s-%s.%s", version, p.os, p.arch, p.ext)
		cs = append(cs, &assets.Asset{
			Name: name,
			URL:  fmt.Sprintf("%s/%s", helmDownloadBase, name),
		})
	}
	return cs
}

func (h *helm) fetchRelease(version string) (string, []*assets.Asset, error) {
	version = normalizeHelmVersion(version)
	if version == "" {
		var err error
		version, _, err = h.latestVersion()
		if err != nil {
			return "", nil, err
		}
	}

	// The Helm archives contain an os-arch/ directory (e.g. linux-amd64/helm),
	// which ProcessURL unpacks, keeping the executable file.
	return version, h.candidates(version), nil
}

// latestVersion returns the latest Helm version and a URL to its release notes.
func (h *helm) latestVersion() (string, string, error) {
	log.Debugf("Getting latest release for helm")

	resp, err := h.client.Get(helmLatestVersion)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%d response getting latest Helm version from %s", resp.StatusCode, helmLatestVersion)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	version := normalizeHelmVersion(strings.TrimSpace(string(b)))
	if version == "" {
		return "", "", fmt.Errorf("could not determine latest Helm version")
	}

	return version, fmt.Sprintf("%s/%s", helmDownloadBase, version), nil
}

// normalizeHelmVersion ensures the version carries the leading "v" that Helm's
// release tags and download filenames use (e.g. v3.16.3).
func normalizeHelmVersion(version string) string {
	if version == "" || strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// parseHelmTag extracts the version from get.helm.sh URLs, either a bare
// version path (get.helm.sh/v3.16.3) or a release download URL
// (get.helm.sh/helm-v3.16.3-linux-amd64.tar.gz).
func parseHelmTag(u *url.URL) string {
	tag := strings.Trim(u.Path, "/")
	if tag == "" || tag == "helm-latest-version" {
		return ""
	}
	if strings.HasPrefix(tag, "helm-") {
		tag = strings.TrimPrefix(tag, "helm-")
		for _, p := range helmPlatforms {
			suffix := fmt.Sprintf("-%s-%s.%s", p.os, p.arch, p.ext)
			if strings.HasSuffix(tag, suffix) {
				tag = strings.TrimSuffix(tag, suffix)
				break
			}
		}
	}
	return normalizeHelmVersion(tag)
}

func newHelm(u *url.URL) (Provider, error) {
	return &httpReleaseProvider{
		id:  "helm",
		tag: parseHelmTag(u),
		src: &helm{client: httpclient.Client},
	}, nil
}
