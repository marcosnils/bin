package providers

import (
	"hash"
	"io"
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

func New(url string) (Provider, error) {
	return newGitHub(url)
}
