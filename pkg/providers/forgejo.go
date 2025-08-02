// Copied from the gitlab provider
package providers

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	forgejoSdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/assets"
)

type forgejo struct {
	url    *url.URL
	client *forgejoSdk.Client
	token  string
	owner  string
	repo   string
	tag    string
}

func (f *forgejo) Fetch(opts *FetchOpts) (*File, error) {
	var client = f.client
	var release *forgejoSdk.Release

	var err error
	// If we have a tag, let's fetch from there
	if len(f.tag) > 0 || len(opts.Version) > 0 {
		if len(opts.Version) > 0 {
			// this is used by for the `ensure` command
			f.tag = opts.Version
		}
		log.Infof("Getting %s release for %s/%s", f.tag, f.owner, f.repo)
		release, _, err = client.GetReleaseByTag(f.owner, f.repo, f.tag)
	} else {
		// TODO: handle case when repo doesn't have releases?
		log.Infof("Fetch: Getting latest release for %s/%s", f.owner, f.repo)
		var latestVersion string
		latestVersion, _, err = f.GetLatestVersion()
		if err != nil {
			return nil, err
		}
		release, _, err = client.GetReleaseByTag(f.owner, f.repo, latestVersion)
	}

	if err != nil {
		return nil, err
	}

	candidates := []*assets.Asset{}
	// candidateURLs := map[string]struct{}{}

	// repo, _, err := client.GetRepo(f.owner, f.repo)
	if err != nil {
		return nil, err
	}

	// repoIsPublic := f.token == "" || !repo.Private

	/* TODO: packages are different in forgejo, so for now ignore them
	log.Debugf("Project is public: %v", repoIsPublic)
	tryPackages := repoIsPublic || repo.HasPackages
	if tryPackages {
		packages, resp, err := client.ListPackages(owner, &forgejoSdk.ListPackagesOptions{
			Page: 1,
			PageSize: min(100, f.client.GetGlobalAPISettings.MaxResponseItems),
		})
		if err != nil && (resp == nil || resp.StatusCode != http.StatusForbidden) {
			return nil, err
		}
		tagVersion := strings.TrimPrefix(release.TagName, "v")
		for _, v := range packages {
			if strings.TrimPrefix(v.Version, "v") == tagVersion {
				totalPages := -1
				for page := 0; page != totalPages; page++ {
					packageFiles, resp, err := g.client.Packages.ListPackageFiles(projectPath, v.ID, &gitlab.ListPackageFilesOptions{
						Page: page + 1,
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
	*/

	/* Gitlab's project uploads might be forgejo's packages
	projectUploadsURL := fmt.Sprintf("%s/uploads/", project.WebURL)
	for _, link := range release.Assets.Links {
		if repoIsPublic || !strings.HasPrefix(link.URL, projectUploadsURL) {
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
	*/

	// TODO: It fails somewhere around here

	for _, attachment := range release.Attachments {
		asset := &assets.Asset{
			Name: attachment.Name,
			URL:  attachment.DownloadURL,
		}

		candidates = append(candidates, asset)
	}

	fi := assets.NewFilter(&assets.FilterOpts{SkipScoring: opts.All, PackagePath: opts.PackagePath, SkipPathCheck: opts.SkipPatchCheck})

	gf, err := fi.FilterAssets(f.repo, candidates)
	if err != nil {
		return nil, err
	}

	if f.token != "" {
		if gf.ExtraHeaders == nil {
			gf.ExtraHeaders = map[string]string{}
		}
		gf.ExtraHeaders["PRIVATE-TOKEN"] = f.token
	}

	outFile, err := fi.ProcessURL(gf)
	if err != nil {
		return nil, err
	}

	version := release.TagName

	// TODO calculate file hash. Not sure if we can / should do it here
	// since we don't want to read the file unnecesarily. Additionally, sometimes
	// releases have .sha256 files, so it'd be nice to check for those also
	file := &File{Data: outFile.Source, Name: outFile.Name, Version: version}

	return file, nil
}

func (f *forgejo) GetID() string {
	return "forgejo"
}

// GetLatestVersion checks the latest repo release and
// returns the corresponding name and url to fetch the version
func (f *forgejo) GetLatestVersion() (string, string, error) {
	log.Debugf("Getting latest release for %s/%s", f.owner, f.repo)

	var latestRelease, _, err = f.client.GetLatestRelease(f.owner, f.repo)
	if err != nil {
		return "", "", err
	}

	/*
		var globalApiSettings, _, err = f.client.GetGlobalAPISettings()
		if err != nil {
			return "", "", err
		}
		var maxResponseItems = globalApiSettings.MaxResponseItems

		var false1, false2 = false, false

		releases, _, err := f.client.ListReleases(f.owner, f.repo, forgejoSdk.ListReleasesOptions{
			ListOptions:  forgejoSdk.ListOptions{Page: 1, PageSize: min(100, maxResponseItems)},
			IsDraft:      &false1,
			IsPreRelease: &false2,
		})

		if err != nil {
			return "", "", err
		}
		if len(releases) == 0 {
			return "", "", fmt.Errorf("no releases found for %s/%s", f.owner, f.repo)
		}

		highestTagName := releases[0].TagName

		var svs semver.Versions
		svToTagName := map[string]string{}
		tagNameToRelease := map[string]*forgejoSdk.Release{}
		for _, release := range releases {
			tagName := strings.TrimPrefix(release.TagName, "v")
			sv, err := semver.NewVersion(tagName)
			if err != nil {
				fmt.Print(err, "\n")
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
		} else {
			// TODO: Semver didn't work, try a different method
			// Ideally, we'd ask the user to pick one (maybe bin can already do this?)
			// Failing that, just go with the latest non-pre release
			// (doesn't look like bin supports pre-releases, maybe TODO those too?)
			return "", "", fmt.Errorf("Could not determine latest release (try specifying one manually)")
		}
	*/

	return latestRelease.TagName, latestRelease.URL, nil
}

func newForgejo(u *url.URL) (Provider, error) {
	s := strings.Split(u.Path, "/")
	if len(s) < 3 {
		return nil, fmt.Errorf("Error parsing Forgejo URL %s, can't find owner and repo", u.String())
	}

	// it's a specific releases URL
	var tag string
	if strings.Contains(u.Path, "/releases/tag/") {
		// For release URLs, the
		// path is usually /releases/v0.1.
		ps := strings.Split(u.Path, "/")
		for i, p := range ps {
			if p == "tag" {
				tag = strings.Join(ps[i+1:], "/")
			}
		}

	}

	// Not familiar enough with forgejo (or bin) to know if this makes sense w/ forgejo
	token := os.Getenv("FORGEJO_TOKEN")
	hostnameSpecificEnvVarName := fmt.Sprintf("FORGEJO_TOKEN_%s", strings.ReplaceAll(u.Hostname(), `.`, "_"))
	hostnameSpecificToken := os.Getenv(hostnameSpecificEnvVarName)
	if hostnameSpecificToken != "" {
		token = hostnameSpecificToken
	}
	client, err := forgejoSdk.NewClient("https://"+u.Host, forgejoSdk.SetToken(token))
	if err != nil {
		return nil, err
	}
	return &forgejo{url: u, client: client, token: token, owner: s[1], repo: s[2], tag: tag}, nil
}
