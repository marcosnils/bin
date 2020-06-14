package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/moby/moby/client"
)

const (
	sh = `docker run --rm -i -t -v ${PWD}:/tmp/cmd -w /tmp/cmd %s/%s:%s "$@"`
)

type docker struct {
	client *client.Client
	url    *url.URL
}

func (d *docker) Fetch() (*File, error) {
	owner, name, version := getImageDesc(d.url.Path)
	out, err := d.client.ImagePull(context.Background(), fmt.Sprintf("docker.io/%s/%s:%s", owner, name, version), types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	defer out.Close()

	if _, err := io.Copy(ioutil.Discard, out); err != nil {
		return nil, err
	}

	return &File{
		Data:    ioutil.NopCloser(strings.NewReader(fmt.Sprintf(sh, owner, name, version))),
		Name:    name,
		Version: version,
		Hash:    sha256.New(),
	}, nil
}

// TODO: implement
func (d *docker) GetLatestVersion(name string) (string, string, error) {
	return "", "", nil
}

// getImageDesc gest the image owner, name and version from the path.
func getImageDesc(path string) (string, string, string) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "r/")
	var (
		owner, image, version string
		imageDesc             = strings.Split(path, "/")
	)
	if len(imageDesc) == 1 {
		owner, image = "library", imageDesc[0]
	} else {
		owner, image = imageDesc[0], imageDesc[1]
	}

	version = "latest"
	imageVersion := strings.Split(image, ":")
	if len(imageVersion) > 1 {
		image, version = imageVersion[0], imageVersion[1]
	}

	return owner, image, version
}

func newDocker(u *url.URL) (Provider, error) {
	if u.Path == "" || len(strings.Split(strings.TrimPrefix("/r", u.Path), "/")) > 3 {
		return nil, fmt.Errorf("Error parsing registry URL. %s is not a valid image name and version", u.Path)
	}
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &docker{url: u, client: client}, nil
}
