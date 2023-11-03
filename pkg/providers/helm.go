package providers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/apex/log"

	"github.com/marcosnils/bin/pkg/assets"
)

const (
	repo           = "helm"
	helmBaseURL    = "https://get.helm.sh"
	helmLatestPath = "/helm-latest-version"
)

var (
	helmOSArchSuffixes = []string{
		"darwin-amd64.tar.gz",
		"darwin-arm64.tar.gz",
		"linux-amd64.tar.gz",
		"linux-arm.tar.gz",
		"linux-arm64.tar.gz",
		"linux-386.tar.gz",
		"linux-ppc64le.tar.gz",
		"linux-s390x.tar.gz",
		"windows-amd64.zip",
	}
)

type helm struct {
	tag string
}

func (h *helm) GetID() string {
	return repo
}

func (h *helm) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest Helm release")

	resp, err := http.Get(helmBaseURL + helmLatestPath)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP response error: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("Error reading response body: %v", err)
	}

	return strings.TrimSuffix(string(body), "\n"), helmBaseURL, nil
}

func (h *helm) Fetch(opts *FetchOpts) (*File, error) {
	var version string
	if len(h.tag) > 0 {
		log.Infof("Getting release %s for Helm", h.tag)
		version = h.tag
	} else {
		latest, _, err := h.GetLatestVersion()
		if err != nil {
			return nil, err
		}
		version = latest
	}

	candidates := []*assets.Asset{}
	for _, suffix := range helmOSArchSuffixes {
		fname := fmt.Sprintf("%s-%s-%s", repo, version, suffix)
		link := fmt.Sprintf("%s/%s", helmBaseURL, fname)
		candidates = append(candidates, &assets.Asset{Name: fname, URL: link})
	}

	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck})
	gf, err := f.FilterAssets(repo, candidates)
	if err != nil {
		return nil, err
	}

	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	return &File{Data: outFile.Source, Name: assets.SanitizeName(outFile.Name, version), Hash: sha256.New(), Version: version}, nil
}

func newHelm(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")

	// it's a specific releases URL
	var tag string
	if len(s) >= 2 {
		tag = parseTagVersion(s[1])
	}

	return &helm{tag: tag}, nil
}

func parseTagVersion(s string) (v string) {
	r, err := regexp.Compile(`^.*(v((\d*)\.(\d*)\.(\d*))).*$`)
	if err != nil {
		return
	}

	m := r.FindStringSubmatch(s)
	if len(m) > 0 {
		v = m[1]
	}

	return
}
