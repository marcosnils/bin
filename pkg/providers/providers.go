package providers

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var ErrInvalidProvider = errors.New("invalid provider")

type File struct {
	Data        io.Reader
	Name        string
	Hash        hash.Hash
	Version     string
	Length      int64
	PackagePath string
}

type FetchOpts struct {
	All            bool
	PackageName    string
	PackagePath    string
	SkipPatchCheck bool
	AutoSelect     string
}

type Provider interface {
	// Fetch returns the file metadata to retrieve a specific binary given
	// for a provider
	Fetch(*FetchOpts) (*File, error)
	// GetLatestVersion returns the version and the URL of the
	// latest version for this binary
	GetLatestVersion() (string, string, error)

	// GetID returns the unique identifier of this provider
	GetID() string
}

var (
	httpUrlPrefix   = regexp.MustCompile("^https?://")
	dockerUrlPrefix = regexp.MustCompile("^docker://")
)

func New(u, provider string) (Provider, error) {
	if dockerUrlPrefix.MatchString(u) {
		return newDocker(u)
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

	if strings.Contains(purl.Host, "releases.hashicorp.com") || provider == "hashicorp" {
		return newHashiCorp(purl)
	}

	return nil, fmt.Errorf("can't find provider for url %s", u)
}
