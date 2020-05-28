package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
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
	tag    string
}

func (g *gitHub) Fetch() (*File, error) {

	var release *github.RepositoryRelease

	// If we have a tag, let's fetch from there
	var err error
	if len(g.tag) > 0 {
		log.Infof("Getting %s release for %s/%s", g.tag, g.owner, g.repo)
		release, _, err = g.client.Repositories.GetReleaseByTag(context.TODO(), g.owner, g.repo, g.tag)
	} else {
		//TODO handle case when repo doesn't have releases?
		log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
		release, _, err = g.client.Repositories.GetLatestRelease(context.TODO(), g.owner, g.repo)
	}

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

func newGitHub(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 2 {
		return nil, fmt.Errorf("Error parsing Github URL %s, can't find owner and repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if strings.Contains(u.Path, "/releases/") {
		tag = filepath.Base(u.Path)

	}
	client := github.NewClient(nil)
	return &gitHub{url: u, client: client, owner: s[1], repo: s[2], tag: tag}, nil
}
