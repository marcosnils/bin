package providers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/assets"
)

type codeberg struct {
	url    *url.URL
	client *gitea.Client
	owner  string
	repo   string
	tag    string
	token  string
}

func (c *codeberg) Fetch(opts *FetchOpts) (*File, error) {
	var release *gitea.Release

	// If we have a tag, let's fetch from there
	var err error
	var resp *gitea.Response
	if len(c.tag) > 0 || len(opts.Version) > 0 {
		if len(opts.Version) > 0 {
			// this is used by for the `ensure` command
			c.tag = opts.Version
		}
		log.Infof("Getting %s release for %s/%s", c.tag, c.owner, c.repo)
		release, _, err = c.client.GetReleaseByTag(c.owner, c.repo, c.tag)
	} else {
		log.Infof("Getting latest release for %s/%s", c.owner, c.repo)
		release, resp, err = c.client.GetLatestRelease(c.owner, c.repo)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			err = fmt.Errorf("repository %s/%s does not have releases", c.owner, c.repo)
		}
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	for _, a := range release.Attachments {
		candidates = append(candidates, &assets.Asset{Name: a.Name, URL: a.DownloadURL})
	}
	f := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck, PackageName: opts.PackageName})

	gf, err := f.FilterAssets(c.repo, candidates)
	if err != nil {
		return nil, err
	}

	gf.ExtraHeaders = map[string]string{"Accept": "application/octet-stream"}
	if c.token != "" {
		gf.ExtraHeaders["Authorization"] = fmt.Sprintf("token %s", c.token)
	}

	outFile, err := f.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	version := release.TagName

	// TODO calculate file hash. Not sure if we can / should do it here
	// since we don't want to read the file unnecesarily. Additionally, sometimes
	// releases have .sha256 files, so it'd be nice to check for those also
	file := &File{Data: outFile.Source, Name: outFile.Name, Version: version, PackagePath: outFile.PackagePath}

	return file, nil
}

// GetLatestVersion checks the latest repo release and
// returns the corresponding name and url to fetch the version
func (c *codeberg) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", c.owner, c.repo)
	release, _, err := c.client.GetLatestRelease(c.owner, c.repo)
	if err != nil {
		return "", "", err
	}

	return release.TagName, release.HTMLURL, nil
}

func (c *codeberg) GetID() string {
	return "codeberg"
}

func newCodeberg(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 3 {
		return nil, fmt.Errorf("error parsing Codeberg URL %s, can't find owner and repo", u.String())
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

	token := os.Getenv("CODEBERG_TOKEN")

	// Codeberg uses Gitea/Forgejo, use the Gitea SDK
	baseURL := fmt.Sprintf("https://%s/", u.Hostname())

	var client *gitea.Client
	var err error

	if token != "" {
		client, err = gitea.NewClient(baseURL, gitea.SetToken(token))
	} else {
		client, err = gitea.NewClient(baseURL)
	}

	if err != nil {
		return nil, fmt.Errorf("error initializing Codeberg client %v", err)
	}

	return &codeberg{url: u, client: client, owner: s[1], repo: s[2], tag: tag, token: token}, nil
}
