package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/reference"
	"github.com/moby/moby/client"
)

const (
	sh = `docker run --rm -i -t -v ${PWD}:/tmp/cmd -w /tmp/cmd %s:%s "$@"`
)

type docker struct {
	client    *client.Client
	repo, tag string
}

func (d *docker) Fetch() (*File, error) {
	out, err := d.client.ImagePull(context.Background(), fmt.Sprintf("%s:%s", d.repo, d.tag), types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	defer out.Close()

	if _, err := io.Copy(ioutil.Discard, out); err != nil {
		return nil, err
	}

	return &File{
		Data:    ioutil.NopCloser(strings.NewReader(fmt.Sprintf(sh, d.repo, d.tag))),
		Name:    getImageName(d.repo),
		Version: d.tag,
		Hash:    sha256.New(),
	}, nil
}

// getImageName gets the name of the image from the image repo.
func getImageName(repo string) string {
	image := strings.Split(repo, "/")
	return image[len(image)-1]
}

// TODO: implement
func (d *docker) GetLatestVersion(name string) (string, string, error) {
	return "", "", nil
}

func newDocker(imageURL string) (Provider, error) {
	imageURL = strings.TrimPrefix(imageURL, "docker://")

	repo, tag, err := parseImage(imageURL)
	if err != nil {
		return nil, err
	}

	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &docker{repo: repo, tag: tag, client: client}, nil
}

// parseImage parses the image returning the repository, tag
// and an error if it fails. ParseImage handles non-canonical
// URLs like `hashicorp/terraform`.
func parseImage(imageURL string) (string, string, error) {
	repo, tag, err := reference.Parse(imageURL)
	if err == nil {
		return repo, tag, nil
	}

	if err != distreference.ErrNameNotCanonical {
		return "", "", fmt.Errorf("image %s is invalid: %w", imageURL, err)
	}

	image := imageURL
	tag = "latest"
	if i := strings.LastIndex(imageURL, ":"); i > -1 {
		image = imageURL[:i]
		tag = imageURL[i+1:]
	}

	if strings.Count(imageURL, "/") == 0 {
		image = "library/" + image
	}

	return fmt.Sprintf("docker.io/%s", image), tag, nil
}
