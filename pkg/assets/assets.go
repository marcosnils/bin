package assets

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
)

type Asset struct {
	Name string
	URL  string
}

type FilteredAsset struct {
	Name  string
	URL   string
	score int
}

func (g *FilteredAsset) String() string { return g.Name }

// FilterAssets receives a slice of GL assets and tries to
// select the proper one and ask the user to manually select one
// in case it can't determine it
func FilterAssets(repoName string, as []*Asset) (*FilteredAsset, error) {
	matches := []*FilteredAsset{}
	scores := map[string]int{}
	scores[repoName] = 1
	for _, os := range config.GetOS() {
		scores[os] = 10
	}
	for _, arch := range config.GetArch() {
		scores[arch] = 5
	}
	scoreKeys := []string{}
	for key := range scores {
		scoreKeys = append(scoreKeys, key)
	}
	for _, a := range as {
		lowerName := strings.ToLower(a.Name)
		lowerURLPathBasename := path.Base(strings.ToLower(a.URL))
		filetype.GetType(lowerName)
		highestScoreForAsset := 0
		gf := &FilteredAsset{Name: a.Name, URL: a.URL, score: 0}
		for _, candidate := range []string{lowerName, lowerURLPathBasename} {
			if bstrings.ContainsAny(candidate, scoreKeys) &&
				isSupportedExt(candidate) {
				for toMatch, score := range scores {
					if strings.Contains(candidate, toMatch) {
						gf.score += score
					}
				}
				if gf.score > highestScoreForAsset {
					highestScoreForAsset = gf.score
					gf.Name = candidate
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
			log.Debugf("Removing %v with score %v lower than %v", matches[i].Name, matches[i].score, highestAssetScore)
			matches = append(matches[:i], matches[i+1:]...)
		} else {
			log.Debugf("Keeping %v with highest score %v", matches[i].Name, matches[i].score)
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
		gf = options.Select("Multiple matches found, please select one:", generic).(*FilteredAsset)
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

// ProcessTarGz receives a tar.gz file and returns the
// correct file for bin to download
func ProcessTarGz(r io.Reader) (string, io.Reader, error) {
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
	tarFiles := []string{}
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

	generic := make([]fmt.Stringer, 0)
	for _, f := range tarFiles {
		generic = append(generic, options.LiteralStringer(f))
	}
	selectedFile := options.Select("Select file to download:", generic).(fmt.Stringer).String()

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
