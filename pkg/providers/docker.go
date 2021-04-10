package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

type docker struct {
	client    *client.Client
	repo, tag string
}

func (d *docker) Fetch() (*File, error) {
	log.Infof("Pulling docker image %s:%s", d.repo, d.tag)
	out, err := d.client.ImageCreate(context.Background(), fmt.Sprintf("%s:%s", d.repo, d.tag), types.ImageCreateOptions{})
	if err != nil {
		return nil, err
	}
	defer out.Close()

	err = jsonmessage.DisplayJSONMessagesStream(
		out,
		os.Stderr,
		os.Stdout.Fd(),
		false,
		nil)

	if err != nil {
		return nil, err
	}

	return &File{
		Data:    strings.NewReader(fmt.Sprintf(sh, d.repo, d.tag)),
		Name:    getImageName(d.repo),
		Version: d.tag,
		Hash:    sha256.New(),
	}, nil
}

// TODO: missing implementation here
func (d *docker) GetLatestVersion() (string, string, error) {
	return d.tag, "", nil
}

func (d *docker) GetID() string {
	return "docker"
}

func newDocker(imageURL string) (Provider, error) {
	imageURL = strings.TrimPrefix(imageURL, "docker://")

	repo, tag := parseImage(imageURL)

	client, err := client.NewClientWithOpts()
	if err != nil {
		return nil, err
	}

	return &docker{repo: repo, tag: tag, client: client}, nil
}

// parseImage parses the image returning the repository and tag.
// It handles non-canonical URLs like `hashicorp/terraform`.
func parseImage(imageURL string) (string, string) {
	image := imageURL
	tag := "latest"
	if i := strings.LastIndex(imageURL, ":"); i > -1 {
		image = imageURL[:i]
		tag = imageURL[i+1:]
	}

	if strings.Count(imageURL, "/") == 0 {
		image = "library/" + image
	}

	return image, tag
}
