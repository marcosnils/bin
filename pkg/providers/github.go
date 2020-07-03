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
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
)

type gitHub struct {
	url    *url.URL
	client *github.Client
	owner  string
	repo   string
	tag    string
}

type githubFileInfo struct{ url, name string }

func (g *githubFileInfo) String() string { return g.name }

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
	matches := []interface{}{}
	for _, a := range release.Assets {
		lowerName := strings.ToLower(*a.Name)
		if bstrings.ContainsAny(lowerName, config.GetOS()) && bstrings.ContainsAny(lowerName, config.GetArch()) {
			matches = append(matches, &githubFileInfo{a.GetBrowserDownloadURL(), a.GetName()})
		}
	}

	var gf *githubFileInfo
	if len(matches) == 0 {
		return nil, fmt.Errorf("Could not find any compatbile files")
	} else if len(matches) > 1 {
		gf = options.Select("Multiple matches found, please select one:", matches).(*githubFileInfo)
		//TODO make user select the proper file
	} else {
		gf = matches[0].(*githubFileInfo)
	}

	// We're not closing the body here since the caller is in charge of that
	res, err := http.Get(gf.url)
	log.Debugf("Checking binary form %s", gf.url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.url)
	}

	//TODO calculate file hash. Not sure if we can / should do it here
	//since we don't want to read the file unnecesarily. Additionally, sometimes
	//releases have .sha256 files, so it'd be nice to check for those also
	f = &File{Data: res.Body, Name: gf.name, Hash: sha256.New(), Version: getVersion(gf.url)}
	return f, nil
}

//GetLatestVersion returns the version and the URL of the
//specified asset name
func (g *gitHub) GetLatestVersion(name string) (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", g.owner, g.repo)
	release, _, err := g.client.Repositories.GetLatestRelease(context.TODO(), g.owner, g.repo)
	if err != nil {
		return "", "", err
	}

	var newDownloadUrl string
	//TODO if asset can be found with the same name it had before,
	//we should prompt the user if he wants to change the asset
	for _, a := range release.Assets {
		if a.GetName() == name {
			newDownloadUrl = a.GetBrowserDownloadURL()
		}

	}
	return release.GetTagName(), newDownloadUrl, nil
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
		//For release and download URL's, the
		//path is usually /releases/tag/v0.1
		// or /releases/download/v0.1.
		ps := strings.Split(u.Path, "/")
		for i, p := range ps {
			if p == "releases" {
				tag = ps[i+2]
			}
		}

	}
	client := github.NewClient(nil)
	return &gitHub{url: u, client: client, owner: s[1], repo: s[2], tag: tag}, nil
}
