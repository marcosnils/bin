package assets

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/caarlos0/log"
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

var (
	msiType = filetype.AddType("msi", "application/octet-stream")
	ascType = filetype.AddType("asc", "text/plain")
)

type Asset struct {
	Name string
	// Some providers (like gitlab) have non-descriptive names for files,
	// so we're using this DisplayName as a helper to produce prettier
	// outputs for bin
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

type finalFile struct {
	Source      io.Reader
	Name        string
	PackagePath string
}

type platformResolver interface {
	GetOS() []string
	GetArch() []string
	GetOSSpecificExtensions() []string
}

type Filter struct {
	opts            *FilterOpts
	repoName        string
	name            string
	packagePath     string
	namePatternUsed bool
}

type FilterOpts struct {
	SkipScoring   bool
	SkipPathCheck bool

	// In case of updates, we're sending the previous version package path
	// so in case it's the same one, we can re-use it.
	PackageName string

	// If target file is in a package format (tar, zip,etc) use this
	// variable to filter the resulting outputs. This is very useful
	// so we don't prompt the user to pick the file again on updates
	PackagePath string

	// NamePattern is a glob pattern for selecting assets. If it contains a
	// slash, the part before the slash matches top-level release asset names
	// and the part after matches files inside archives. Without a slash the
	// whole pattern matches top-level asset names only.
	NamePattern string
}

type runtimeResolver struct{}

func (runtimeResolver) GetOS() []string {
	return config.GetOS()
}

func (runtimeResolver) GetArch() []string {
	return config.GetArch()
}

func (runtimeResolver) GetOSSpecificExtensions() []string {
	return config.GetOSSpecificExtensions()
}

var resolver platformResolver = runtimeResolver{}

func (g FilteredAsset) String() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.Name
}

func NewFilter(opts *FilterOpts) *Filter {
	return &Filter{opts: opts}
}

// FilterAssets receives a slice of assets and tries to select the proper one,
// prompting the user to choose manually when it can't determine a single match.
func (f *Filter) FilterAssets(repoName string, as []*Asset) (*FilteredAsset, error) {
	if f.opts.NamePattern != "" && !f.namePatternUsed {
		var err error
		as, err = f.applyNamePattern(as)
		if err != nil {
			return nil, err
		}
	}

	var matches []*FilteredAsset
	switch {
	case len(as) == 1:
		a := as[0]
		matches = []*FilteredAsset{{RepoName: repoName, Name: a.Name, URL: a.URL}}
	case f.opts.SkipScoring:
		log.Debugf("--all flag was supplied, skipping scoring")
		matches = toFilteredAssets(repoName, as)
	default:
		matches = f.scoreAssets(repoName, as)
	}

	return selectCandidate(matches)
}

// applyNamePattern filters assets to those matching the asset portion of
// NamePattern (the part before the first slash, if any).
func (f *Filter) applyNamePattern(as []*Asset) ([]*Asset, error) {
	f.namePatternUsed = true
	pattern := f.opts.NamePattern
	if idx := strings.Index(pattern, "/"); idx >= 0 {
		pattern = pattern[:idx]
	}
	var matches []*Asset
	for _, a := range as {
		matched, err := filepath.Match(pattern, a.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid name pattern %q: %w", pattern, err)
		}
		if matched {
			matches = append(matches, a)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no assets matching pattern %q", pattern)
	}
	return matches, nil
}

// toFilteredAssets converts a raw asset slice to FilteredAssets with no scoring.
func toFilteredAssets(repoName string, as []*Asset) []*FilteredAsset {
	out := make([]*FilteredAsset, len(as))
	for i, a := range as {
		out[i] = &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL}
	}
	return out
}

// scoreAssets scores each asset by OS/arch/extension relevance and returns
// only those tied for the highest score.
func (f *Filter) scoreAssets(repoName string, as []*Asset) []*FilteredAsset {
	scores := map[string]int{repoName: 1}
	for _, os := range resolver.GetOS() {
		scores[os] = 10
	}
	for _, arch := range resolver.GetArch() {
		scores[arch] = 5
	}
	for _, ext := range resolver.GetOSSpecificExtensions() {
		scores[ext] = 15
	}

	scoreKeys := make([]string, 0, len(scores))
	for key := range scores {
		scoreKeys = append(scoreKeys, strings.ToLower(key))
	}

	var matches []*FilteredAsset
	for _, a := range as {
		if s := scoreAsset(a.Name, scores, scoreKeys); s > 0 {
			matches = append(matches, &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL, score: s})
		}
	}
	return keepHighestScored(matches)
}

// scoreAsset returns the total score for a single asset name, or 0 if it
// doesn't qualify (unsupported extension or no keyword matches).
func scoreAsset(name string, scores map[string]int, scoreKeys []string) int {
	if !bstrings.ContainsAny(strings.ToLower(name), scoreKeys) || !isSupportedExt(name) {
		return 0
	}
	total := 0
	for toMatch, score := range scores {
		if strings.Contains(strings.ToLower(name), strings.ToLower(toMatch)) {
			log.Debugf("Candidate %s contains %s. Adding score %d", name, toMatch, score)
			total += score
		}
	}
	return total
}

// keepHighestScored filters matches down to those tied for the top score.
func keepHighestScored(matches []*FilteredAsset) []*FilteredAsset {
	highest := 0
	for _, m := range matches {
		if m.score > highest {
			highest = m.score
		}
	}
	var out []*FilteredAsset
	for _, m := range matches {
		if m.score >= highest {
			log.Debugf("Keeping %v (URL %v) with highest score %v", m.Name, m.URL, m.score)
			out = append(out, m)
		} else {
			log.Debugf("Removing %v (URL %v) with score %v lower than %v", m.Name, m.URL, m.score, highest)
		}
	}
	return out
}

// selectCandidate returns the single best match, or prompts the user when
// multiple candidates remain. Returns an error if there are no candidates.
func selectCandidate(matches []*FilteredAsset) (*FilteredAsset, error) {
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("Could not find any compatible files")
	case 1:
		return matches[0], nil
	}

	generic := make([]fmt.Stringer, len(matches))
	for i, m := range matches {
		generic[i] = m
	}
	sort.SliceStable(generic, func(i, j int) bool {
		return generic[i].String() < generic[j].String()
	})
	choice, err := options.Select("Multiple matches found, please select one:", generic)
	if err != nil {
		return nil, err
	}
	return choice.(*FilteredAsset), nil
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
func (f *Filter) ProcessURL(gf *FilteredAsset) (*finalFile, error) {
	f.name = gf.Name
	// We're not closing the body here since the caller is in charge of that
	req, err := http.NewRequest(http.MethodGet, gf.URL, nil)
	if err != nil {
		return nil, err
	}
	for name, value := range gf.ExtraHeaders {
		req.Header.Add(name, value)
	}
	log.Debugf("Checking binary from %s", gf.URL)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.URL)
	}

	// We're caching the whole file into memory so we can prompt
	// the user which file they want to download

	log.Infof("Starting download of %s", gf.URL)
	bar := pb.Full.Start64(res.ContentLength)
	barReader := bar.NewProxyReader(res.Body)
	defer bar.Finish()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, barReader)
	if err != nil {
		return nil, err
	}
	bar.Finish()
	return f.processReader(buf)
}

func (f *Filter) processReader(r io.Reader) (*finalFile, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)

	t, err := filetype.MatchReader(tee)
	if err != nil {
		return nil, err
	}

	outputFile := io.MultiReader(&buf, r)

	type processorFunc func(repoName string, r io.Reader) (*finalFile, error)
	var processor processorFunc
	switch t {
	case matchers.TypeGz:
		processor = f.processGz
	case matchers.TypeTar:
		processor = f.processTar
	case matchers.TypeXz:
		processor = f.processXz
	case matchers.TypeBz2:
		processor = f.processBz2
	case matchers.TypeZip:
		processor = f.processZip
	}

	if processor != nil {
		// log.Debugf("Processing %s file %s with %s", repoName, name, runtime.FuncForPC(reflect.ValueOf(processor).Pointer()).Name())
		outFile, err := processor(f.repoName, outputFile)
		if err != nil {
			return nil, err
		}

		outputFile = outFile.Source

		f.name = outFile.Name
		f.packagePath = outFile.PackagePath

		// In case of e.g. a .tar.gz, process the uncompressed archive by calling recursively
		return f.processReader(outputFile)
	}

	return &finalFile{Source: outputFile, Name: f.name, PackagePath: f.packagePath}, err
}

// processGz receives a tar.gz file and returns the
// correct file for bin to download
func (f *Filter) processGz(name string, r io.Reader) (*finalFile, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &finalFile{Source: gr, Name: gr.Name}, nil
}

func (f *Filter) processTar(name string, r io.Reader) (*finalFile, error) {
	tr := tar.NewReader(r)
	tarFiles := map[string][]byte{}
	if len(f.opts.PackagePath) > 0 {
		log.Debugf("Processing tag with PackagePath %s\n", f.opts.PackagePath)
	}
	execFiles := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if header.FileInfo().IsDir() {
			continue
		}

		if !f.opts.SkipPathCheck && len(f.opts.PackagePath) > 0 && header.Name != f.opts.PackagePath {
			continue
		}

		if header.Typeflag == tar.TypeReg {
			// TODO we're basically reading all the files
			// isn't there a way just to store the reference
			// where this data is so we don't have to do this or
			// re-scan the archive twice afterwards?
			bs, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			tarFiles[header.Name] = bs
			if header.FileInfo().Mode()&0o111 != 0 {
				execFiles[header.Name] = bs
			}
		}
	}
	if len(execFiles) > 0 {
		log.Debugf("Filtering tar candidates to %d executable file(s)", len(execFiles))
		tarFiles = execFiles
	} else {
		log.Debugf("No executable files found in tar archive, considering all files")
	}
	if len(tarFiles) == 0 {
		return nil, fmt.Errorf("no files found in tar archive, use -p flag to manually select . PackagePath [%s]", f.opts.PackagePath)
	}

	var err error
	if tarFiles, err = f.applyFilePattern(tarFiles); err != nil {
		return nil, err
	}

	as := make([]*Asset, 0)
	for f := range tarFiles {
		as = append(as, &Asset{Name: f, URL: ""})
	}
	choice, err := f.FilterAssets(name, as)
	if err != nil {
		return nil, err
	}
	selectedFile := choice.String()

	tf := tarFiles[selectedFile]

	return &finalFile{Source: bytes.NewReader(tf), Name: filepath.Base(selectedFile), PackagePath: selectedFile}, nil
}

func (f *Filter) processBz2(name string, r io.Reader) (*finalFile, error) {
	br := bzip2.NewReader(r)

	return &finalFile{Source: br, Name: name}, nil
}

func (f *Filter) processXz(name string, r io.Reader) (*finalFile, error) {
	xr, err := xz.NewReader(r, 0)
	if err != nil {
		return nil, err
	}

	return &finalFile{Source: xr, Name: name}, nil
}

func (f *Filter) processZip(name string, r io.Reader) (*finalFile, error) {
	zr := zipstream.NewReader(r)

	zipFiles := map[string][]byte{}
	zipExecFiles := map[string][]byte{}
	if len(f.opts.PackagePath) > 0 {
		log.Debugf("Processing tag with PackagePath %s\n", f.opts.PackagePath)
	}
	for {
		header, err := zr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if header.Mode().IsDir() {
			continue
		}

		if !f.opts.SkipPathCheck && len(f.opts.PackagePath) > 0 && header.Name != f.opts.PackagePath {
			continue
		}

		// TODO we're basically reading all the files
		// isn't there a way just to store the reference
		// where this data is so we don't have to do this or
		// re-scan the archive twice afterwards?
		bs, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}

		zipFiles[header.Name] = bs
		if header.Mode()&0o111 != 0 {
			zipExecFiles[header.Name] = bs
		}
	}
	if len(zipExecFiles) > 0 {
		log.Debugf("Filtering zip candidates to %d executable file(s)", len(zipExecFiles))
		zipFiles = zipExecFiles
	} else {
		log.Debugf("No executable files found in zip archive, considering all files")
	}
	if len(zipFiles) == 0 {
		return nil, fmt.Errorf("No files found in zip archive. PackagePath [%s]", f.opts.PackagePath)
	}

	var err error
	if zipFiles, err = f.applyFilePattern(zipFiles); err != nil {
		return nil, err
	}

	as := make([]*Asset, 0)
	for f := range zipFiles {
		as = append(as, &Asset{Name: f, URL: ""})
	}
	choice, err := f.FilterAssets(name, as)
	if err != nil {
		return nil, err
	}
	selectedFile := choice.String()

	fr := bytes.NewReader(zipFiles[selectedFile])

	// return base of selected file since tar
	// files usually have folders inside
	return &finalFile{Name: filepath.Base(selectedFile), Source: fr, PackagePath: selectedFile}, nil
}

// applyFilePattern filters files by the path portion of NamePattern (the part
// after the first slash). Each entry is matched against both its full path and
// its base name, so "mytool" matches "dir/mytool". If NamePattern has no slash
// the map is returned unchanged.
func (f *Filter) applyFilePattern(files map[string][]byte) (map[string][]byte, error) {
	idx := strings.Index(f.opts.NamePattern, "/")
	if idx < 0 {
		return files, nil
	}
	filePattern := f.opts.NamePattern[idx+1:]
	filtered := make(map[string][]byte)
	for n, data := range files {
		matched, err := filepath.Match(filePattern, n)
		if err != nil {
			return nil, fmt.Errorf("invalid path pattern %q: %w", filePattern, err)
		}
		if !matched {
			if matched, err = filepath.Match(filePattern, filepath.Base(n)); err != nil {
				return nil, fmt.Errorf("invalid path pattern %q: %w", filePattern, err)
			}
		}
		if matched {
			filtered[n] = data
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no files in archive matching pattern %q", filePattern)
	}
	return filtered, nil
}

// isSupportedExt checks if this provider supports
// dealing with this specific file extension
// nonBinaryExts lists extensions that are never installable binaries or archives.
// These are excluded from scoring so they don't compete with real assets.
var nonBinaryExts = map[string]bool{
	"txt":    true,
	"sha256": true,
	"sha512": true,
	"sha1":   true,
	"md5":    true,
	"b3":     true,
	"sum":    true,
	"sig":    true,
	"pem":    true,
	"json":   true,
	"yaml":   true,
	"yml":    true,
	"sbom":   true,
}

func isSupportedExt(filename string) bool {
	if ext := strings.TrimPrefix(filepath.Ext(filename), "."); len(ext) > 0 {
		if nonBinaryExts[strings.ToLower(ext)] {
			log.Debugf("Filename %s doesn't have a supported extension", filename)
			return false
		}
		switch filetype.GetType(ext) {
		case msiType, matchers.TypeDeb, matchers.TypeRpm, ascType:
			log.Debugf("Filename %s doesn't have a supported extension", filename)
			return false
		case matchers.TypeGz, types.Unknown, matchers.TypeZip, matchers.TypeXz, matchers.TypeTar, matchers.TypeBz2, matchers.TypeExe:
			break
		default:
			log.Debugf("Filename %s doesn't have a supported extension", filename)
			return false
		}
	}

	return true
}
