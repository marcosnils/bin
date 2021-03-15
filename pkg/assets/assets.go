package assets

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/cheggaaa/pb"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/krolaw/zipstream"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
	"github.com/xi2/xz"
)

type Asset struct {
	Name        string
	DisplayName string
	URL         string
}

func (g Asset) String() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.Name
}

type FilteredAsset struct {
	RepoName     string
	Name         string
	DisplayName  string
	URL          string
	score        int
	ExtraHeaders map[string]string
}

type platformResolver interface {
	GetOS() []string
	GetArch() []string
}

type runtimeResolver struct{}

func (runtimeResolver) GetOS() []string {
	return config.GetOS()
}

func (runtimeResolver) GetArch() []string {
	return config.GetArch()
}

var resolver platformResolver = runtimeResolver{}

func (g FilteredAsset) String() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.Name
}

// FilterAssets receives a slice of GL assets and tries to
// select the proper one and ask the user to manually select one
// in case it can't determine it
func FilterAssets(repoName string, as []*Asset) (*FilteredAsset, error) {
	matches := []*FilteredAsset{}
	scores := map[string]int{}
	scores[repoName] = 1
	for _, os := range resolver.GetOS() {
		scores[os] = 10
	}
	for _, arch := range resolver.GetArch() {
		scores[arch] = 5
	}
	scoreKeys := []string{}
	for key := range scores {
		scoreKeys = append(scoreKeys, key)
	}
	for _, a := range as {
		lowerName := strings.ToLower(a.Name)
		lowerURLPathBasename := path.Base(strings.ToLower(a.URL))
		highestScoreForAsset := 0
		gf := &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL, score: 0}
		for _, candidate := range []string{lowerName, lowerURLPathBasename} {
			candidateScore := 0
			if bstrings.ContainsAny(candidate, scoreKeys) &&
				isSupportedExt(candidate) {
				for toMatch, score := range scores {
					if strings.Contains(candidate, strings.ToLower(toMatch)) {
						candidateScore += score
					}
				}
				if candidateScore > highestScoreForAsset {
					highestScoreForAsset = candidateScore
					gf.Name = candidate
					gf.score = candidateScore
				}
			}
		}
		if highestScoreForAsset > 0 {
			matches = append(matches, gf)
		}
	}
	highestAssetScore := 0
	for i := range matches {
		if matches[i].score > highestAssetScore {
			highestAssetScore = matches[i].score
		}
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i].score < highestAssetScore {
			log.Debugf("Removing %v (URL %v) with score %v lower than %v", matches[i].Name, matches[i].URL, matches[i].score, highestAssetScore)
			matches = append(matches[:i], matches[i+1:]...)
		} else {
			log.Debugf("Keeping %v (URL %v) with highest score %v", matches[i].Name, matches[i].URL, matches[i].score)
		}
	}

	var gf *FilteredAsset
	if len(matches) == 0 {
		return nil, fmt.Errorf("Could not find any compatible files")
	} else if len(matches) > 1 {
		generic := make([]fmt.Stringer, 0)
		for _, f := range matches {
			generic = append(generic, f)
		}
		choice, err := options.Select("Multiple matches found, please select one:", generic)
		if err != nil {
			return nil, err
		}
		gf = choice.(*FilteredAsset)
		//TODO make user select the proper file
	} else {
		gf = matches[0]
	}

	return gf, nil

}

// SanitizeName removes irrelevant information from the
// file name in case it exists
func SanitizeName(name, version string) string {
	name = strings.ToLower(name)
	replacements := []string{}

	// TODO maybe instead of doing this put everything in a map (set) and then
	// generate the replacements? IDK.
	firstPass := true
	for _, osName := range resolver.GetOS() {
		for _, archName := range resolver.GetArch() {
			replacements = append(replacements, "_"+osName+archName, "")
			replacements = append(replacements, "-"+osName+archName, "")
			replacements = append(replacements, "."+osName+archName, "")

			if firstPass {
				replacements = append(replacements, "_"+archName, "")
				replacements = append(replacements, "-"+archName, "")
				replacements = append(replacements, "."+archName, "")
			}
		}

		replacements = append(replacements, "_"+osName, "")
		replacements = append(replacements, "-"+osName, "")
		replacements = append(replacements, "."+osName, "")

		firstPass = false

	}

	replacements = append(replacements, "_"+version, "")
	replacements = append(replacements, "_"+strings.TrimPrefix(version, "v"), "")
	replacements = append(replacements, "-"+version, "")
	replacements = append(replacements, "-"+strings.TrimPrefix(version, "v"), "")
	r := strings.NewReplacer(replacements...)
	return r.Replace(name)
}

// ProcessURL processes a FilteredAsset by uncompressing/unarchiving the URL of the asset.
func ProcessURL(gf *FilteredAsset) (string, io.Reader, error) {
	// We're not closing the body here since the caller is in charge of that
	req, err := http.NewRequest(http.MethodGet, gf.URL, nil)
	if err != nil {
		return "", nil, err
	}
	for name, value := range gf.ExtraHeaders {
		req.Header.Add(name, value)
	}
	log.Debugf("Checking binary from %s", gf.URL)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return "", nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.URL)
	}

	// We're caching the whole file into memory so we can prompt
	// the user which file they want to download

	log.Infof("Starting download of %s", gf.URL)
	bar := pb.Full.Start64(res.ContentLength)
	barReader := bar.NewProxyReader(res.Body)
	defer bar.Finish()
	buf := new(bytes.Buffer)
	io.Copy(buf, barReader)
	if err != nil {
		return "", nil, err
	}
	bar.Finish()
	return processReader(gf.RepoName, gf.Name, buf)
}

func processReader(repoName string, name string, r io.Reader) (string, io.Reader, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)

	t, err := filetype.MatchReader(tee)
	if err != nil {
		return "", nil, err
	}

	var outputFile = io.MultiReader(&buf, r)

	type processorFunc func(repoName string, r io.Reader) (string, io.Reader, error)
	var processor processorFunc
	switch t {
	case matchers.TypeGz:
		processor = processGz
	case matchers.TypeTar:
		processor = processTar
	case matchers.TypeXz:
		processor = processXz
	case matchers.TypeZip:
		processor = processZip
	}
	if processor != nil {
		// log.Debugf("Processing %s file %s with %s", repoName, name, runtime.FuncForPC(reflect.ValueOf(processor).Pointer()).Name())
		name, outputFile, err = processor(repoName, outputFile)
		if err != nil {
			return "", nil, err
		}
		// In case of e.g. a .tar.gz, process the uncompressed archive by calling recursively
		return processReader(repoName, name, outputFile)
	}

	return name, outputFile, err
}

// processGz receives a tar.gz file and returns the
// correct file for bin to download
func processGz(name string, r io.Reader) (string, io.Reader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return "", nil, err
	}

	return gr.Name, gr, nil
}

func processTar(name string, r io.Reader) (string, io.Reader, error) {
	tr := tar.NewReader(r)
	tarFiles := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		if header.Typeflag == tar.TypeReg {
			bs, err := ioutil.ReadAll(tr)
			if err != nil {
				return "", nil, err
			}
			tarFiles[header.Name] = bs
		}
	}
	if len(tarFiles) == 0 {
		return "", nil, errors.New("No files found in tar archive")
	}

	as := make([]*Asset, 0)
	for f := range tarFiles {
		as = append(as, &Asset{Name: f, URL: ""})
	}
	choice, err := FilterAssets(name, as)
	if err != nil {
		return "", nil, err
	}
	selectedFile := choice.String()

	tf := tarFiles[selectedFile]
	return filepath.Base(selectedFile), bytes.NewReader(tf), nil
}

func processXz(name string, r io.Reader) (string, io.Reader, error) {
	xr, err := xz.NewReader(r, 0)
	if err != nil {
		return "", nil, err
	}

	return "", xr, nil
}

func processZip(name string, r io.Reader) (string, io.Reader, error) {
	zr := zipstream.NewReader(r)

	zipFiles := map[string][]byte{}
	for {
		header, err := zr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		bs, err := ioutil.ReadAll(zr)
		if err != nil {
			return "", nil, err
		}

		zipFiles[header.Name] = bs
	}
	if len(zipFiles) == 0 {
		return "", nil, errors.New("No files found in zip archive")
	}

	generic := make([]fmt.Stringer, 0)
	for f := range zipFiles {
		generic = append(generic, options.LiteralStringer(f))
	}
	choice, err := options.Select("Select file to extract:", generic)
	if err != nil {
		return "", nil, err
	}
	selectedFile := choice.(fmt.Stringer).String()

	fr := bytes.NewReader(zipFiles[selectedFile])

	// return base of selected file since tar
	// files usually have folders inside
	return filepath.Base(selectedFile), fr, nil
}

// isSupportedExt checks if this provider supports
// dealing with this specific file extension
func isSupportedExt(filename string) bool {
	if ext := strings.TrimPrefix(filepath.Ext(filename), "."); len(ext) > 0 {
		switch filetype.GetType(ext) {
		case matchers.TypeGz, types.Unknown, matchers.TypeZip, matchers.TypeXz, matchers.TypeTar, matchers.TypeExe:
			break
		default:
			return false
		}
	}

	return true
}
