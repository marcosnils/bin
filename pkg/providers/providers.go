package providers

import (
	"hash"
	"io"
	"net/url"
)

type File struct {
	Data    io.ReadCloser
	Name    string
	Hash    hash.Hash
	Version string
}

type Provider interface {
	Fetch() (*File, error)
}

func New(url *url.URL) (Provider, error) {
	return newGitHub(url)
}
