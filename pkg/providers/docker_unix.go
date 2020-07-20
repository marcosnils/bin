// +build !windows

package providers

import "strings"

const (
	// TODO: this probably won't work on windows so we might need how we mount
	// TODO: there might be a way were users can configure a template for the
	// actual execution since some CLIs require some other folders to be mounted
	// or networks to be shared
	sh = `docker run --rm -i -t -v ${PWD}:/tmp/cmd -w /tmp/cmd %s:%s "$@"`
)

// getImageName gets the name of the image from the image repo.
func getImageName(repo string) string {
	image := strings.Split(repo, "/")
	return image[len(image)-1]
}
