package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/xanzy/go-gitlab"
	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type gitLab struct {
	url    *url.URL
	client *gitlab.Client
	owner  string
	repo   string
	tag    string
}

type gitlabFileInfo struct {
	url, name string
	score     int
}

func (g *gitlabFileInfo) String() string {
	return g.name
}

func (g *gitLab) Fetch() (*File, error) {

	var release *gitlab.Release

	// If we have a tag, let's fetch from there
	var err error
	projectPath := fmt.Sprintf("%s/%s", g.owner, g.repo)
	if len(g.tag) > 0 {
		log.Infof("Getting %s release for %s/%s", g.tag, g.owner, g.repo)
		release, _, err = g.client.Releases.GetRelease(projectPath, g.tag)
	} else {
		//TODO handle case when repo doesn't have releases?
		log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
		releases, _, err := g.client.Releases.ListReleases(projectPath, &gitlab.ListReleasesOptions{
			PerPage: 1,
		})
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("no releases found for %s/%s", g.owner, g.repo)
		}
		release, _, err = g.client.Releases.GetRelease(projectPath, releases[0].TagName, gitlab.WithContext(context.TODO()))
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	for _, link := range release.Assets.Links {
		candidates = append(candidates, &assets.Asset{Name: link.Name, URL: link.URL})
	}

	node := goldmark.DefaultParser().Parse(text.NewReader([]byte(release.Description)))
	walker := func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		if n.Type() == goldast.TypeInline && n.Kind() == goldast.KindLink {
			link := n.(*goldast.Link)
			name := string(link.Title)
			url := string(link.Destination)
			candidates = append(candidates, &assets.Asset{Name: name, URL: url})
		}
		return goldast.WalkContinue, nil
	}
	if err := goldast.Walk(node, walker); err != nil {
		return nil, err
	}

	gf, err := assets.FilterAssets(g.repo, candidates)

	if err != nil {
		return nil, err
	}

	name, outputFile, err := assets.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	version := release.TagName

	//TODO calculate file hash. Not sure if we can / should do it here
	//since we don't want to read the file unnecesarily. Additionally, sometimes
	//releases have .sha256 files, so it'd be nice to check for those also
	f := &File{Data: outputFile, Name: assets.SanitizeName(name, version), Hash: sha256.New(), Version: version}

	return f, nil
}

//GetLatestVersion checks the latest repo release and
//returns the corresponding name and url to fetch the version
func (g *gitLab) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", g.owner, g.repo)
	projectPath := fmt.Sprintf("%s/%s", g.owner, g.repo)
	releases, _, err := g.client.Releases.ListReleases(projectPath, &gitlab.ListReleasesOptions{
		PerPage: 1,
	})
	if err != nil {
		return "", "", err
	}
	release, _, err := g.client.Releases.GetRelease(projectPath, releases[0].TagName)
	if err != nil {
		return "", "", err
	}

	return release.TagName, release.Commit.WebURL, nil
}

func newGitLab(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 2 {
		return nil, fmt.Errorf("Error parsing GitLab URL %s, can't find owner and repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if strings.Contains(u.Path, "/releases/") {
		//For release URL's, the
		//path is usually /releases/v0.1.
		ps := strings.Split(u.Path, "/")
		for i, p := range ps {
			if p == "releases" {
				tag = strings.Join(ps[i+1:], "/")
			}
		}

	}

	token := os.Getenv("GITLAB_TOKEN")
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(fmt.Sprintf("https://%s/api/v4", u.Hostname())))
	if err != nil {
		return nil, err
	}
	return &gitLab{url: u, client: client, owner: s[1], repo: s[2], tag: tag}, nil
}
