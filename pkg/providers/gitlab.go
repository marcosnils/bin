package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/coreos/go-semver/semver"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/xanzy/go-gitlab"
	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type gitLab struct {
	url    *url.URL
	client *gitlab.Client
	token  string
	owner  string
	repo   string
	tag    string
}

type gitlabFileInfo struct {
	url, name, displayName string
	score                  int
}

func (g *gitlabFileInfo) String() string {
	if g.displayName != "" {
		return g.displayName
	}
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
		// TODO: handle case when repo doesn't have releases?
		log.Infof("Getting latest release for %s/%s", g.owner, g.repo)
		name, _, err := g.GetLatestVersion()
		if err != nil {
			return nil, err
		}
		release, _, err = g.client.Releases.GetRelease(projectPath, name, gitlab.WithContext(context.TODO()))
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	candidateURLs := map[string]struct{}{}

	project, _, err := g.client.Projects.GetProject(projectPath, &gitlab.GetProjectOptions{})
	if err != nil {
		return nil, err
	}
	if project.PackagesEnabled {
		packages, _, err := g.client.Packages.ListProjectPackages(projectPath, &gitlab.ListProjectPackagesOptions{
			OrderBy: gitlab.String("version"),
			Sort:    gitlab.String("desc"),
		})
		if err != nil {
			return nil, err
		}
		tagVersion := strings.TrimPrefix(release.TagName, "v")
		for _, v := range packages {
			if strings.TrimPrefix(v.Version, "v") == tagVersion {
				totalPages := -1
				for page := 1; page != totalPages; page++ {
					packageFiles, resp, err := g.client.Packages.ListPackageFiles(projectPath, v.ID, &gitlab.ListPackageFilesOptions{
						Page: page,
					})
					if err != nil {
						return nil, err
					}
					totalPages = resp.TotalPages
					for _, f := range packageFiles {
						assetURL := fmt.Sprintf("%sprojects/%s/packages/%s/%s/%s/%s",
							g.client.BaseURL().String(),
							url.PathEscape(projectPath),
							v.PackageType,
							v.Name,
							v.Version,
							f.FileName,
						)
						if _, exists := candidateURLs[assetURL]; !exists {
							asset := &assets.Asset{
								Name:        f.FileName,
								DisplayName: fmt.Sprintf("%s (%s package)", f.FileName, v.PackageType),
								URL:         assetURL,
							}
							candidates = append(candidates, asset)
							log.Debugf("Adding %s with URL %s", asset, asset.URL)
						}
						candidateURLs[assetURL] = struct{}{}
					}
				}
			}
		}
	}

	projectUploadsURL := fmt.Sprintf("%s/uploads/", project.WebURL)
	projectIsPublic := g.token == "" || project.Visibility == "" || project.Visibility == gitlab.PublicVisibility
	log.Debugf("Project is public: %v", projectIsPublic)
	for _, link := range release.Assets.Links {
		if projectIsPublic || !strings.HasPrefix(link.URL, projectUploadsURL) {
			if _, exists := candidateURLs[link.URL]; !exists {
				asset := &assets.Asset{
					Name:        link.Name,
					DisplayName: fmt.Sprintf("%s (asset link)", link.Name),
					URL:         link.URL,
				}
				candidates = append(candidates, asset)
				log.Debugf("Adding %s with URL %s", asset, asset.URL)
			}
			candidateURLs[link.URL] = struct{}{}
		}
	}

	node := goldmark.DefaultParser().Parse(text.NewReader([]byte(release.Description)))
	walker := func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		if n.Type() == goldast.TypeInline && n.Kind() == goldast.KindLink {
			link := n.(*goldast.Link)
			name := string(link.Title)
			assetURL := string(link.Destination)
			if projectIsPublic || !strings.HasPrefix(assetURL, projectUploadsURL) {
				if _, exists := candidateURLs[assetURL]; !exists {
					asset := &assets.Asset{
						Name:        name,
						DisplayName: fmt.Sprintf("%s (from release description)", name),
						URL:         assetURL,
					}
					candidates = append(candidates, asset)
					log.Debugf("Adding %s with URL %s", asset, asset.URL)
				}
				candidateURLs[assetURL] = struct{}{}
			}
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

	if g.token != "" {
		if gf.ExtraHeaders == nil {
			gf.ExtraHeaders = map[string]string{}
		}
		gf.ExtraHeaders["PRIVATE-TOKEN"] = g.token
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
		PerPage: 100,
	})
	if err != nil {
		return "", "", err
	}
	if len(releases) == 0 {
		return "", "", fmt.Errorf("no releases found for %s/%s", g.owner, g.repo)
	}
	highestTagName := releases[0].TagName
	var svs semver.Versions
	svToTagName := map[string]string{}
	tagNameToRelease := map[string]*gitlab.Release{}
	for _, release := range releases {
		tagName := release.TagName
		if strings.HasPrefix(tagName, "v") {
			tagName = strings.TrimPrefix(tagName, "v")
		}
		sv, err := semver.NewVersion(tagName)
		if err != nil {
			continue
		}
		if sv.PreRelease == "" && sv.Metadata == "" {
			svs = append(svs, sv)
			svToTagName[sv.String()] = release.TagName
			tagNameToRelease[release.TagName] = release
		}
	}
	if len(svs) > 0 {
		sort.Sort(svs)
		highestTagName = svToTagName[svs[len(svs)-1].String()]
	}

	return highestTagName, tagNameToRelease[highestTagName].Commit.WebURL, nil
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
	hostnameSpecificEnvVarName := fmt.Sprintf("GITLAB_TOKEN_%s", strings.ReplaceAll(u.Hostname(), `.`, "_"))
	hostnameSpecificToken := os.Getenv(hostnameSpecificEnvVarName)
	if hostnameSpecificToken != "" {
		token = hostnameSpecificToken
	}
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(fmt.Sprintf("https://%s/api/v4", u.Hostname())))
	if err != nil {
		return nil, err
	}
	return &gitLab{url: u, client: client, token: token, owner: s[1], repo: s[2], tag: tag}, nil
}
