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
	Data    io.ReadCloser
	Name    string
	Hash    hash.Hash
	Version string
}

type Provider interface {
	Fetch() (*File, error)
	GetLatestVersion(string) (string, string, error)
}

var httpUrlPrefix = regexp.MustCompile("^https?://")

func New(u string) (Provider, error) {
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

	if strings.Contains(purl.Host, "hub.docker.com") || strings.Contains(purl.Host, "docker.io") {
		return newDocker(purl)
	}

	return nil, fmt.Errorf("Can't find provider for url %s", u)
}
