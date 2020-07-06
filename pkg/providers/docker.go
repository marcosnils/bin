package providers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/reference"
	"github.com/moby/moby/client"
	"github.com/moby/moby/pkg/jsonmessage"
)

const (
	// TODO: this probably won't work on windows so we might need how we mount
	// TODO: there might be a way were users can configure a template for the
	// actual execution since some CLIs require some other folders to be mounted
	// or networks to be shared
	sh = `docker run --rm -i -t -v ${PWD}:/tmp/cmd -w /tmp/cmd %s:%s "$@"`
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

// getImageName gets the name of the image from the image repo.
func getImageName(repo string) string {
	image := strings.Split(repo, "/")
	return image[len(image)-1]
}

// TODO: missing implementation here
func (d *docker) GetLatestVersion() (string, string, error) {
	return d.tag, "", nil
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
