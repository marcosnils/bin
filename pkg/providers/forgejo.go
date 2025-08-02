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

	if err != nil {
		return nil, err
	}

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
