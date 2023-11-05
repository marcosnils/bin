package providers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/assets"
)

type generic struct {
	baseURL   string
	name      string
	version   string
	os        string
	arch      string
	ext       string
	latestURL string
}

func (g *generic) GetID() string {
	return "generic"
}

func (g *generic) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s", g.name)

	resp, err := http.Get(g.latestURL)
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

	version := strings.TrimSuffix(string(body), "\n")
	fname := fmt.Sprintf("%s-%s-%s-%s.%s", g.name, version, g.os, g.arch, g.ext)
	link := fmt.Sprintf("%s/%s", g.baseURL, fname)

	return version, link, nil
}

func (g *generic) Fetch(opts *FetchOpts) (*File, error) {
	var version string
	if len(g.version) > 0 {
		log.Infof("Getting release %s for %s", g.version, g.name)
		version = g.version
	} else {
		latest, _, err := g.GetLatestVersion()
		if err != nil {
			return nil, err
		}
		version = latest
	}

	fname := fmt.Sprintf("%s-%s-%s-%s.%s", g.name, version, g.os, g.arch, g.ext)
	link := fmt.Sprintf("%s/%s", g.baseURL, fname)

	candidates := []*assets.Asset{
		{
			Name: fname,
			URL:  link,
		},
	}

	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck})
	gf, err := f.FilterAssets(g.name, candidates)
	if err != nil {
		return nil, err
	}

	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	return &File{Data: outFile.Source, Name: assets.SanitizeName(outFile.Name, version), Hash: sha256.New(), Version: version}, nil
}

func newGeneric(u *url.URL, latestURL string) (Provider, error) {
	r := regexp.MustCompile(`^(.+)\/(.+)-(v\d*\.\d*\.\d*)-([a-z]+)-([a-z0-9]+).(.*)$`)

	m := r.FindStringSubmatch(u.String())
	if len(m) != 7 {
		return nil, errors.New("Failed to parse specified URL")
	}

	if len(latestURL) == 0 {
		return nil, errors.New("Latest URL must be specified for generic provider")
	}

	return &generic{baseURL: m[1], name: m[2], version: m[3], os: m[4], arch: m[5], ext: m[6], latestURL: latestURL}, nil
}
