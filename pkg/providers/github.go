package providers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/google/go-github/v31/github"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
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

func (g *githubFileInfo) String() string { return g.name }

// filterAssets receives a slice of GH assets and tries to
// select the proper one and ask the user to manually select one
// in case it can't determine it
func filterAssets(as []*github.ReleaseAsset) (*githubFileInfo, error) {
	matches := []interface{}{}
	for _, a := range as {
		lowerName := strings.ToLower(*a.Name)
		filetype.GetType(lowerName)
		if bstrings.ContainsAny(lowerName, config.GetOS()) &&
			bstrings.ContainsAny(lowerName, config.GetArch()) &&
			isSupportedExt(lowerName) {
			matches = append(matches, &githubFileInfo{a.GetBrowserDownloadURL(), a.GetName()})
		}
	}
	// If we don't match any resources using the "standard" strategy,
	// try to be a bit more flexible to find better alternatives.
	// I guess that ideally we'd have to build a prioritization
	// list instead of doing this that seems a hack :D.
	if len(matches) == 0 {
		for _, a := range as {
			lowerName := strings.ToLower(*a.Name)
			if isSupportedExt(lowerName) && (bstrings.ContainsAny(lowerName, config.GetOS()) || bstrings.ContainsAny(lowerName, config.GetArch())) {
				matches = append(matches, &githubFileInfo{a.GetBrowserDownloadURL(), a.GetName()})
			}
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

	return gf, nil

}

// isSupportedExt checks if this provider supports
// dealing with this specific file extension
func isSupportedExt(filename string) bool {
	if ext := strings.TrimPrefix(filepath.Ext(filename), "."); len(ext) > 0 {
		switch filetype.GetType(ext) {
		case matchers.TypeGz, types.Unknown:
			break
		default:
			return false
		}
	}

	return true
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

	gf, err := filterAssets(release.Assets)

	if err != nil {
		return nil, err
	}

	// We're not closing the body here since the caller is in charge of that
	res, err := http.Get(gf.url)
	log.Debugf("Checking binary from %s", gf.url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.url)
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

	var name = gf.name

	if t == matchers.TypeGz {
		fileName, file, err := processTarGz(outputFile)
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
	f := &File{Data: outputFile, Name: sanitizeName(name, version), Hash: sha256.New(), Version: version, Length: res.ContentLength}

	return f, nil
}

// sanitizeName removes irrelevant information from the
// file name in case it exists
func sanitizeName(name, version string) string {
	name = strings.ToLower(name)
	replacements := []string{}

	// TODO maybe instead of doing this put everything in a map (set) and then

	// generate the replacements? IDK.
	firstPass := true
	for _, osName := range config.GetOS() {
		for _, archName := range config.GetArch() {
			replacements = append(replacements, "_"+osName+archName, "")
			replacements = append(replacements, "-"+osName+archName, "")

			if firstPass {
				replacements = append(replacements, "_"+archName, "")
				replacements = append(replacements, "-"+archName, "")
			}
		}

		replacements = append(replacements, "_"+osName, "")
		replacements = append(replacements, "-"+osName, "")

		firstPass = false

	}

	replacements = append(replacements, "_"+version, "")
	replacements = append(replacements, "_"+strings.TrimPrefix(version, "v"), "")
	replacements = append(replacements, "-"+version, "")
	replacements = append(replacements, "-"+strings.TrimPrefix(version, "v"), "")
	r := strings.NewReplacer(replacements...)
	return r.Replace(name)
}

// processTar receives a tar.gz file and returns the
// correct file for bin to download
func processTarGz(r io.Reader) (string, io.Reader, error) {
	// We're caching the whole file into memory so we can prompt
	// the user which file they want to download

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	br := bytes.NewReader(b)

	gr, err := gzip.NewReader(br)
	if err != nil {
		return "", nil, err
	}

	tr := tar.NewReader(gr)
	tarFiles := []interface{}{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		if header.Typeflag == tar.TypeReg {
			tarFiles = append(tarFiles, header.Name)
		}
	}
	if len(tarFiles) == 0 {
		return "", nil, errors.New("No files found in tar archive")
	}

	selectedFile := options.Select("Select file to download:", tarFiles).(string)

	// Reset readers so we can scan the tar file
	// again to get the correct file reader
	br.Seek(0, io.SeekStart)
	gr, err = gzip.NewReader(br)
	if err != nil {
		return "", nil, err
	}
	tr = tar.NewReader(gr)

	var fr io.Reader
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		if header.Name == selectedFile {
			fr = tr
			break
		}
	}
	// return base of selected file since tar
	// files usually have folders inside
	return filepath.Base(selectedFile), fr, nil

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
