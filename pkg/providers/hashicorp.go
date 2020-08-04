package providers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/coreos/go-semver/semver"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/marcosnils/bin/pkg/options"
)

const (
	releasesURLBase = "https://releases.hashicorp.com"
)

type hashiCorp struct {
	url     *url.URL
	client  *http.Client
	owner   string
	repo    string
	tag     string
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

type hashiCorpFileInfo struct {
	url, name string
	score     int
}

func (g *hashiCorpFileInfo) String() string {
	return g.name
}

func (g *hashiCorp) Fetch() (*File, error) {

	var release *hashiCorpRelease

	// If we have a tag, let's fetch from there
	var err error
	if len(g.tag) > 0 {
		log.Infof("Getting %s release for %s", g.tag, g.repo)
		release, err = g.getRelease(g.repo, g.tag)
	} else {
		version, _, err := g.GetLatestVersion()
		if err != nil {
			return nil, err
		}
		release, err = g.getRelease(g.repo, version)
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	for _, link := range release.Builds {
		candidates = append(candidates, &assets.Asset{Name: link.Filename, URL: link.URL})
	}

	gf, err := assets.FilterAssets(g.repo, candidates)

	if err != nil {
		return nil, err
	}

	name, outputFile, length, err := assets.ProcessURL(gf)

	version := release.Version

	//TODO calculate file hash. Not sure if we can / should do it here
	//since we don't want to read the file unnecesarily. Additionally, sometimes
	//releases have .sha256 files, so it'd be nice to check for those also
	f := &File{Data: outputFile, Name: assets.SanitizeName(name, version), Hash: sha256.New(), Version: version, Length: length}

	return f, nil
}

//GetLatestVersion checks the latest repo release and
//returns the corresponding name and url to fetch the version
func (g *hashiCorp) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s", g.repo)

	releases, err := g.listReleases(g.repo)
	if err != nil {
		return "", "", err
	}
	if len(releases.Versions) == 0 {
		return "", "", fmt.Errorf("no releases found for %s", g.repo)
	}
	var svs semver.Versions
	for _, version := range releases.Versions {
		sv, err := semver.NewVersion(version.Version)
		if err != nil {
			return "", "", err
		}
		svs = append(svs, sv)
	}
	if len(svs) == 0 {
		return "", "", fmt.Errorf("no semver versions found for %s", g.repo)
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
			return "", "", err
		}
		highestVersion = choice.(*semver.Version)
	}
	release, err := g.getRelease(g.repo, highestVersion.String())
	if err != nil {
		return "", "", err
	}

	return release.Version, g.buildHashiCorpAPIURL(g.repo, release.Version), nil
}

func newHashiCorp(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 1 {
		return nil, fmt.Errorf("Error parsing HashiCorp releases URL %s, can't find repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if len(s) >= 3 {
		tag = s[2]
	}

	baseURL, _ := url.Parse(releasesURLBase)

	return &hashiCorp{url: u, client: http.DefaultClient, owner: "", repo: s[1], tag: tag, baseURL: baseURL}, nil
}
