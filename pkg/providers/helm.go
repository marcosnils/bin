package providers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
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

// downloadName builds the release archive filename for a version and platform.
func (p helmPlatform) downloadName(version string) string {
	return fmt.Sprintf("helm-%s-%s-%s.%s", version, p.os, p.arch, p.ext)
}

// candidates builds the list of downloadable assets for a given version.
func (h *helm) candidates(version string) []*assets.Asset {
	cs := make([]*assets.Asset, 0, len(helmPlatforms))
	for _, p := range helmPlatforms {
		name := p.downloadName(version)
		cs = append(cs, &assets.Asset{
			Name: name,
			URL:  fmt.Sprintf("%s/%s", helmDownloadBase, name),
		})
	}
	return cs
}

// downloadURL returns the release download URL for the running platform,
// falling back to linux/amd64. It is the URL shape latestVersion hands back
// for updates, which re-resolve the platform when installing anyway.
func helmDownloadURL(version string) string {
	platform := helmPlatforms[3] // linux/amd64
	for _, p := range helmPlatforms {
		if p.os == runtime.GOOS && p.arch == runtime.GOARCH {
			platform = p
			break
		}
	}
	return fmt.Sprintf("%s/%s", helmDownloadBase, platform.downloadName(version))
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

// latestVersion returns the latest Helm version and its download URL.
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

	return version, helmDownloadURL(version), nil
}

// normalizeHelmVersion ensures the version carries the leading "v" that Helm's
// release tags and download filenames use (e.g. v3.16.3).
func normalizeHelmVersion(version string) string {
	if version == "" || strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// parseHelmTag extracts the pinned version from a release download URL such
// as get.helm.sh/helm-v3.16.3-linux-amd64.tar.gz. get.helm.sh serves no
// other installable URLs, so any other non-empty path is rejected.
func parseHelmTag(u *url.URL) (string, error) {
	tag := strings.Trim(u.Path, "/")
	if tag == "" {
		return "", nil
	}
	if v, ok := strings.CutPrefix(tag, "helm-"); ok {
		for _, p := range helmPlatforms {
			suffix := fmt.Sprintf("-%s-%s.%s", p.os, p.arch, p.ext)
			if strings.HasSuffix(v, suffix) {
				return normalizeHelmVersion(strings.TrimSuffix(v, suffix)), nil
			}
		}
	}
	return "", fmt.Errorf("invalid get.helm.sh URL %s, to install a specific version use the full release URL, e.g. %s", u.String(), helmDownloadURL("v3.16.3"))
}

func newHelm(u *url.URL) (Provider, error) {
	tag, err := parseHelmTag(u)
	if err != nil {
		return nil, err
	}
	return &httpReleaseProvider{
		id:  "helm",
		tag: tag,
		src: &helm{client: httpclient.Client},
	}, nil
}
