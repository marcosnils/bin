package providers

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/coreos/go-semver/semver"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/marcosnils/bin/pkg/assets"
)

const (
	releasesURLBase = "https://releases.hashicorp.com"
)

type hashiCorp struct {
	url    *url.URL
	client *http.Client
	owner  string
	repo   string
	tag    string
}

type hashiCorpFileInfo struct {
	url, name string
	score     int
}

type hashiCorpRelease struct {
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	Shasums          string           `json:"shasums"`
	ShasumsSignature string           `json:"shasums_signature"`
	Builds           []hashiCorpBuild `json:"builds"`
}

type hashiCorpRepo struct {
	Name     osName                      `json:"name"`
	Versions map[string]hashiCorpVersion `json:"versions"`
}

type hashiCorpVersion struct {
	Name             osName           `json:"name"`
	Version          string           `json:"version"`
	Shasums          string           `json:"shasums"`
	ShasumsSignature string           `json:"shasums_signature"`
	Builds           []hashiCorpBuild `json:"builds"`
}

type hashiCorpBuild struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	OS       osName `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type osName string

const (
	Darwin  osName = "darwin"
	Freebsd osName = "freebsd"
	Linux   osName = "linux"
	Openbsd osName = "openbsd"
	Solaris osName = "solaris"
	Windows osName = "windows"
)

func (g *hashiCorpFileInfo) String() string {
	return g.name
}

func buildHashiCorpAPIURL(args ...string) string {
	baseURL, _ := url.Parse(releasesURLBase)

	args = append(args, "index.json")
	baseURL.Path = path.Join(args...)

	return baseURL.String()
}

func (g *hashiCorp) getRelease(repoName, version string) (*hashiCorpRelease, error) {
	releaseURL := buildHashiCorpAPIURL(repoName, version)
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
	repoURL := buildHashiCorpAPIURL(repoName)
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

func (g *hashiCorp) Fetch() (*File, error) {

	var release *hashiCorpRelease

	// If we have a tag, let's fetch from there
	var err error
	projectPath := fmt.Sprintf("%s/%s", g.owner, g.repo)
	if len(g.tag) > 0 {
		log.Infof("Getting %s release for %s/%s", g.tag, g.owner, g.repo)
		release, err = g.getRelease(projectPath, g.tag)
	} else {
		//TODO handle case when repo doesn't have releases?
		log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
		releases, err := g.listReleases(projectPath)
		if err != nil {
			return nil, err
		}
		if len(releases.Versions) == 0 {
			return nil, fmt.Errorf("no releases found for %s/%s", g.owner, g.repo)
		}
		var svs semver.Versions
		for _, version := range releases.Versions {
			sv, err := semver.NewVersion(version.Version)
			if err != nil {
				return nil, err
			}
			svs = append(svs, sv)
		}
		sort.Sort(svs)
		highestVersion := svs[len(svs)-1]
		release, err = g.getRelease(projectPath, highestVersion.String())
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

	// We're not closing the body here since the caller is in charge of that
	res, err := http.Get(gf.URL)
	log.Debugf("Checking binary from %s", gf.URL)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.URL)
	}

	var buf bytes.Buffer
	tee := io.TeeReader(res.Body, &buf)

	t, err := filetype.MatchReader(tee)
	if err != nil {
		return nil, err
	}

	var outputFile = io.MultiReader(&buf, res.Body)

	var name = gf.Name

	type processorFunc func(r io.Reader) (string, io.Reader, error)
	var processor processorFunc
	switch t {
	case matchers.TypeZip:
		processor = assets.ProcessZip
	case matchers.TypeGz:
		processor = assets.ProcessTarGz

	}
	if processor != nil {
		name, outputFile, err = processor(outputFile)
		if err != nil {
			return nil, err
		}
	}

	version := release.Version

	//TODO calculate file hash. Not sure if we can / should do it here
	//since we don't want to read the file unnecesarily. Additionally, sometimes
	//releases have .sha256 files, so it'd be nice to check for those also
	f := &File{Data: outputFile, Name: assets.SanitizeName(name, version), Hash: sha256.New(), Version: version, Length: res.ContentLength}

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
	sort.Sort(svs)
	highestVersion := svs[len(svs)-1]
	release, err := g.getRelease(g.repo, highestVersion.String())
	if err != nil {
		return "", "", err
	}

	return release.Version, buildHashiCorpAPIURL(g.repo, release.Version), nil
}

func newHashiCorp(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 1 {
		return nil, fmt.Errorf("Error parsing HashiCorp releases URL %s, can't find repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if len(s) == 3 {
		tag = s[2]
	}

	return &hashiCorp{url: u, client: http.DefaultClient, owner: "", repo: s[1], tag: tag}, nil
}
