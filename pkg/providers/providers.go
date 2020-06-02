package providers

import (
	"fmt"
	"hash"
	"io"
	"net/url"
	"strings"
)

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

func New(u string) (Provider, error) {
	purl, err := url.Parse(u)

	if err != nil {
		return nil, err
	}

	if strings.Contains(purl.Host, "github") {
		return newGitHub(purl)
	}

	return nil, fmt.Errorf("Can't find provider for url %s", u)
}
