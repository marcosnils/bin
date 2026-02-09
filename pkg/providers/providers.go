package providers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var ErrInvalidProvider = errors.New("invalid provider")

type File struct {
	Data        io.Reader
	Name        string
	Version     string
	Length      int64
	PackagePath string
}

func (f *File) Hash() ([]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, f.Data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

type FetchOpts struct {
	All            bool
	PackageName    string
	PackagePath    string
	SkipPatchCheck bool
	Version        string
}

type Provider interface {
	// Fetch returns the file metadata to retrieve a specific binary given
	// for a provider
	Fetch(*FetchOpts) (*File, error)
	// GetLatestVersion returns the version and the URL of the
	// latest version for this binary
	GetLatestVersion() (string, string, error)

	// GetID returns the unique identiifer of this provider
	GetID() string
}

// Cleaner is an optional interface that providers can implement to perform
// cleanup when a binary is removed. This allows providers that install
// supporting files (libraries, completers, etc.) to clean up after themselves.
type Cleaner interface {
	// Cleanup removes any supporting files or directories installed by the provider.
	// It should log warnings for non-critical failures rather than returning errors.
	Cleanup() error
}

var (
	httpUrlPrefix      = regexp.MustCompile("^https?://")
	dockerUrlPrefix    = regexp.MustCompile("^docker://")
	goinstallUrlPrefix = regexp.MustCompile("^goinstall://")
	awscliUrlPrefix    = regexp.MustCompile("^awscli://")
)

func New(u, provider string) (Provider, error) {
	if dockerUrlPrefix.MatchString(u) {
		return newDocker(u)
	}
	if goinstallUrlPrefix.MatchString(u) || provider == "goinstall" {
		return newGoInstall(u)
	}
	if awscliUrlPrefix.MatchString(u) || provider == "awscli" {
		return newAWSCLI(u)
	}
	if !httpUrlPrefix.MatchString(u) {
		u = fmt.Sprintf("https://%s", u)
	}

	purl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	if strings.Contains(purl.Host, "github") || provider == "github" {
		return newGitHub(purl)
	}

	if strings.Contains(purl.Host, "gitlab") || provider == "gitlab" {
		return newGitLab(purl)
	}

	if strings.Contains(purl.Host, "codeberg") || provider == "codeberg" {
		return newCodeberg(purl)
	}

	if strings.Contains(purl.Host, "releases.hashicorp.com") || provider == "hashicorp" {
		return newHashiCorp(purl)
	}

	return nil, fmt.Errorf("Can't find provider for url %s", u)
}
