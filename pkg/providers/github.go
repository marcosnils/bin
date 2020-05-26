package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/apex/log"
	"github.com/google/go-github/v31/github"
	"github.com/marcosnils/bin/pkg/config"
)

type gitHub struct {
	url    *url.URL
	client *github.Client
	owner  string
	repo   string
}

func (g *gitHub) Fetch() (*File, error) {
	//TODO handle case when repo doesn't have releases?
	log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
	release, _, err := g.client.Repositories.GetLatestRelease(context.TODO(), g.owner, g.repo)
	if err != nil {
		return nil, err
	}

	var f *File
	for _, a := range release.Assets {
		lowerName := strings.ToLower(*a.Name)
		if strings.Contains(lowerName, config.GetOS()) && strings.Contains(lowerName, config.GetArch()) {
			// We're not closing the body here since the caller is in charge of that
			res, err := http.Get(*a.BrowserDownloadURL)
			log.Debugf("Downloading binary form %s", *a.BrowserDownloadURL)
			if err != nil {
				return nil, err
			}
			//TODO calculate file hash
			f = &File{Data: res.Body, Name: *a.Name, Hash: sha256.New(), Version: getVersion(*a.BrowserDownloadURL)}
			break
		}
	}
	return f, nil
}

// getVersion returns the asset version given the
// browser download URL
func getVersion(url string) string {
	s := strings.Split(url, "/")
	return s[len(s)-2]
}

func newGitHub(u string) (Provider, error) {
	purl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	s := strings.Split(purl.Path, "/")
	if len(s) < 2 {
		return nil, fmt.Errorf("Error parsing Github URL %s, can't find owner and repo", u)
	}
	client := github.NewClient(nil)
	return &gitHub{url: purl, client: client, owner: s[1], repo: s[2]}, nil
}
