package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/caarlos0/log"
	"github.com/coreos/go-semver/semver"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/marcosnils/bin/pkg/httpclient"
	"github.com/marcosnils/bin/pkg/options"
)

const (
	releasesURLBase = "https://releases.hashicorp.com"
)

type hashiCorp struct {
	client  *http.Client
	repo    string
	baseURL *url.URL
}

func (g *hashiCorp) buildHashiCorpAPIURL(args ...string) string {
	apiURL := &url.URL{}
	*apiURL = *g.baseURL

	args = append(args, "index.json")
	apiURL.Path = path.Join(args...)

	return apiURL.String()
}

func (g *hashiCorp) getRelease(repoName, version string) (*hashiCorpRelease, error) {
	releaseURL := g.buildHashiCorpAPIURL(repoName, version)
	resp, err := g.client.Get(releaseURL)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var release hashiCorpRelease
	if err := decoder.Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (g *hashiCorp) listReleases(repoName string) (*hashiCorpRepo, error) {
	repoURL := g.buildHashiCorpAPIURL(repoName)
	resp, err := g.client.Get(repoURL)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var repo hashiCorpRepo
	if err := decoder.Decode(&repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// latestRelease finds the highest non-prerelease semver version, asking the
// user to disambiguate ties, and fetches its release metadata.
func (g *hashiCorp) latestRelease() (*hashiCorpRelease, error) {
	releases, err := g.listReleases(g.repo)
	if err != nil {
		return nil, err
	}
	if len(releases.Versions) == 0 {
		return nil, fmt.Errorf("no releases found for %s", g.repo)
	}
	var svs semver.Versions
	for _, version := range releases.Versions {
		sv, err := semver.NewVersion(version.Version)
		if err != nil {
			log.Debugf("unable to parse %q as a semantic version: %+v", version.Version, err)
			continue
		}
		if sv.PreRelease == "" && sv.Metadata == "" {
			svs = append(svs, sv)
		}
	}
	if len(svs) == 0 {
		return nil, fmt.Errorf("no semver versions found for %s", g.repo)
	}
	sort.Sort(svs)
	highestVersion := svs[len(svs)-1]
	tied := map[string]*semver.Version{}
	for i := len(svs) - 1; i >= 0; i-- {
		sv := svs[i]
		if sv.Compare(*highestVersion) == 0 {
			tied[sv.String()] = sv
		}
	}
	if len(tied) > 1 {
		tiedKeys := []string{}
		for key := range tied {
			tiedKeys = append(tiedKeys, key)
		}
		sort.Strings(tiedKeys)
		generic := make([]fmt.Stringer, 0)
		for _, key := range tiedKeys {
			generic = append(generic, tied[key])
		}
		choice, err := options.Select("Select file to download:", generic)
		if err != nil {
			return nil, err
		}
		highestVersion = choice.(*semver.Version)
	}
	return g.getRelease(g.repo, highestVersion.String())
}

func (g *hashiCorp) fetchRelease(version string) (string, []*assets.Asset, error) {
	var release *hashiCorpRelease
	var err error
	if version == "" {
		release, err = g.latestRelease()
	} else {
		release, err = g.getRelease(g.repo, version)
	}
	if err != nil {
		return "", nil, err
	}

	candidates := make([]*assets.Asset, 0, len(release.Builds))
	for _, link := range release.Builds {
		candidates = append(candidates, &assets.Asset{Name: link.Filename, URL: link.URL})
	}

	return release.Version, candidates, nil
}

// latestVersion checks the latest repo release and
// returns the corresponding name and url to fetch the version
func (g *hashiCorp) latestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s", g.repo)

	release, err := g.latestRelease()
	if err != nil {
		return "", "", err
	}

	return release.Version, g.buildHashiCorpAPIURL(g.repo, release.Version), nil
}

func newHashiCorp(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 2 {
		return nil, fmt.Errorf("Error parsing HashiCorp releases URL %s, can't find repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if len(s) >= 3 {
		tag = s[2]
	}

	baseURL, _ := url.Parse(releasesURLBase)

	src := &hashiCorp{client: httpclient.Client, repo: s[1], baseURL: baseURL}
	return &httpReleaseProvider{id: "hashicorp", tag: tag, src: src}, nil
}
