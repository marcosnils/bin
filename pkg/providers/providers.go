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
	Data    io.Reader
	Name    string
	Hash    hash.Hash
	Version string
}

type Provider interface {
	// Fetch returns the file metadata to retrieve a specific binary given
	// for a provider
	Fetch() (*File, error)
	// GetLatestVersion returns the version and the URL of the
	// latest version for this binary
	GetLatestVersion() (string, string, error)
}

var (
	httpUrlPrefix   = regexp.MustCompile("^https?://")
	dockerUrlPrefix = regexp.MustCompile("^docker://")
)

func New(u string) (Provider, error) {
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

	if strings.Contains(purl.Host, "github") {
		return newGitHub(purl)
	}

	return nil, fmt.Errorf("Can't find provider for url %s", u)
}
