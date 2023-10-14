package providers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/google/go-github/v31/github"
	"github.com/marcosnils/bin/pkg/assets"
	"golang.org/x/oauth2"
)

type gitHub struct {
	url    *url.URL
	client *github.Client
	owner  string
	repo   string
	tag    string
	token  string
}

func (g *gitHub) Fetch(opts *FetchOpts) (*File, error) {
	var release *github.RepositoryRelease

	// If we have a tag, let's fetch from there
	var err error
	if len(g.tag) > 0 {
		log.Infof("Getting %s release for %s/%s", g.tag, g.owner, g.repo)
		release, _, err = g.client.Repositories.GetReleaseByTag(context.TODO(), g.owner, g.repo, g.tag)
	} else {
		log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
		var tag string
		tag, _, err = g.GetLatestVersion()
		if err != nil {
			return nil, fmt.Errorf("repository %s/%s does not have releases", g.owner, g.repo)
		}
		release, _, err = g.client.Repositories.GetReleaseByTag(context.TODO(), g.owner, g.repo, tag)
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	for _, a := range release.Assets {
		candidates = append(candidates, &assets.Asset{Name: a.GetName(), URL: a.GetURL()})
	}
	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck})

	gf, err := f.FilterAssets(g.repo, candidates)
	if err != nil {
		return nil, err
	}

	gf.ExtraHeaders = map[string]string{"Accept": "application/octet-stream"}
	if g.token != "" {
		gf.ExtraHeaders["Authorization"] = fmt.Sprintf("token %s", g.token)
	}

	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	version := release.GetTagName()

	// TODO calculate file hash. Not sure if we can / should do it here
	// since we don't want to read the file unnecesarily. Additionally, sometimes
	// releases have .sha256 files, so it'd be nice to check for those also
	file := &File{Data: outFile.Source, Name: assets.SanitizeName(outFile.Name, version), Hash: sha256.New(), Version: version, PackagePath: outFile.PackagePath}

	return file, nil
}

// GetLatestVersion checks the latest repo release and
// returns the corresponding name and url to fetch the version
func (g *gitHub) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", g.owner, g.repo)

	release, resp, err := g.client.Repositories.GetLatestRelease(context.TODO(), g.owner, g.repo)
	if err == nil && resp.StatusCode == http.StatusOK {
		return release.GetTagName(), release.GetHTMLURL(), nil
	}
	latestResp, latestErr := resp, err

	// repo does not have a "latest" release, fallback to first release on the list
	var releases []*github.RepositoryRelease
	releases, resp, err = g.client.Repositories.ListReleases(context.TODO(), g.owner, g.repo, &github.ListOptions{
		Page:    1,
		PerPage: 1,
	})
	if err != nil || len(releases) == 0 {
		if latestErr != nil {
			latestErr = fmt.Errorf("error getting latest release: %w", latestErr)
		} else {
			latestErr = fmt.Errorf("error getting latest release. http status: %v", latestResp.StatusCode)
		}
		if err != nil {
			err = fmt.Errorf("error listing releases: %w", err)
		} else {
			err = fmt.Errorf("error listing releases. http status: %v", resp.StatusCode)
		}
		return "", "", errors.Join(latestErr, err)
	}
	release = releases[0]
	return release.GetTagName(), release.GetHTMLURL(), nil
}

func (g *gitHub) GetID() string {
	return "github"
}

func newGitHub(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 3 {
		return nil, fmt.Errorf("error parsing Github URL %s, can't find owner and repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if strings.Contains(u.Path, "/releases/") {
		// For release and download URL's, the
		// path is usually /releases/tag/v0.1
		// or /releases/download/v0.1.
		ps := strings.Split(u.Path, "/")
		for i, p := range ps {
			if p == "releases" {
				tag = strings.Join(ps[i+2:], "/")
			}
		}

	}

	token := os.Getenv("GITHUB_AUTH_TOKEN")
	if len(token) == 0 {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// GHES client
	gbu := os.Getenv("GHES_BASE_URL")
	guu := os.Getenv("GHES_UPLOAD_URL")
	gau := os.Getenv("GHES_AUTH_TOKEN")

	var tc *http.Client

	if len(gbu) > 0 && len(guu) > 0 && len(gau) > 0 {
		tc = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: gau},
		))
	} else if token != "" {
		tc = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		))
	}

	var client *github.Client
	var err error

	if len(gbu) > 0 && len(guu) > 0 && len(gau) > 0 {
		if client, err = github.NewEnterpriseClient(gbu, guu, tc); err != nil {
			return nil, fmt.Errorf("error initializing GHES client %v", err)
		}
	} else {
		client = github.NewClient(tc)
	}

	return &gitHub{url: u, client: client, owner: s[1], repo: s[2], tag: tag, token: token}, nil
}
