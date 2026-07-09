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
	helmReleasesURL   = "https://github.com/helm/helm/releases/tag"
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
	url    *url.URL
	client *http.Client
	repo   string
	tag    string
}

func (h *helm) GetID() string {
	return "helm"
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

func (h *helm) Fetch(opts *FetchOpts) (*File, error) {
	version := h.tag
	if len(opts.Version) > 0 {
		// this is used by the `ensure` command
		version = opts.Version
	}

	if version == "" {
		var err error
		version, _, err = h.GetLatestVersion()
		if err != nil {
			return nil, err
		}
	}
	version = normalizeHelmVersion(version)

	log.Infof("Getting %s release for %s", version, h.repo)

	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck, PackageName: opts.PackageName, NamePattern: opts.NamePattern})

	gf, err := f.FilterAssets(h.repo, h.candidates(version))
	if err != nil {
		return nil, err
	}

	// The Helm archives contain an os-arch/ directory (e.g. linux-amd64/helm),
	// which ProcessURL unpacks, keeping the executable file.
	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	return &File{Data: outFile.Source, Name: outFile.Name, Version: version, PackagePath: outFile.PackagePath}, nil
}

// GetLatestVersion returns the latest Helm version and a URL to its release notes.
func (h *helm) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s", h.repo)

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

	return version, fmt.Sprintf("%s/%s", helmReleasesURL, version), nil
}

// normalizeHelmVersion ensures the version carries the leading "v" that Helm's
// release tags and download filenames use (e.g. v3.16.3).
func normalizeHelmVersion(version string) string {
	if version == "" || strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func newHelm(u *url.URL) (Provider, error) {
	// Support explicit release URLs such as
	// github.com/helm/helm/releases/tag/v3.16.3
	var tag string
	if strings.Contains(u.Path, "/releases/") {
		ps := strings.Split(u.Path, "/")
		for i, p := range ps {
			if p == "releases" && i+2 < len(ps) {
				tag = strings.Join(ps[i+2:], "/")
			}
		}
	}

	return &helm{url: u, client: httpclient.Client, repo: "helm", tag: tag}, nil
}
