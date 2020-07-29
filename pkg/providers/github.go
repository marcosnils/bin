package providers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/google/go-github/v31/github"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/marcosnils/bin/pkg/assets"
	"golang.org/x/oauth2"
)

type gitHub struct {
	url    *url.URL
	client *github.Client
	owner  string
	repo   string
	tag    string
}

type githubFileInfo struct{ url, name string }

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

	candidates := []*assets.Asset{}
	for _, a := range release.Assets {
		candidates = append(candidates, &assets.Asset{Name: a.GetName(), URL: a.GetBrowserDownloadURL()})
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

	// TODO: validating the type of the file will eventually be
	// handled by each provider since it's impossible to make it generic enough
	// if t != matchers.TypeElf && t != matchers.TypeGz {
	// 	return fmt.Errorf("File type [%v] not supported", t)
	// }

	var name = gf.Name

	if t == matchers.TypeGz {
		fileName, file, err := assets.ProcessTarGz(outputFile)
		if err != nil {
			return nil, err
		}
		outputFile = file
		name = fileName

	}

	version := release.GetTagName()

	//TODO calculate file hash. Not sure if we can / should do it here
	//since we don't want to read the file unnecesarily. Additionally, sometimes
	//releases have .sha256 files, so it'd be nice to check for those also
	f := &File{Data: outputFile, Name: assets.SanitizeName(name, version), Hash: sha256.New(), Version: version, Length: res.ContentLength}

	return f, nil
}

//GetLatestVersion checks the latest repo release and
//returns the corresponding name and url to fetch the version
func (g *gitHub) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", g.owner, g.repo)
	release, _, err := g.client.Repositories.GetLatestRelease(context.TODO(), g.owner, g.repo)
	if err != nil {
		return "", "", err
	}

	return release.GetTagName(), release.GetHTMLURL(), nil
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
				tag = strings.Join(ps[i+2:], "/")
			}
		}

	}

	token := os.Getenv("GITHUB_AUTH_TOKEN")
	var tc *http.Client
	if token != "" {
		tc = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		))
	}
	client := github.NewClient(tc)
	return &gitHub{url: u, client: client, owner: s[1], repo: s[2], tag: tag}, nil
}
