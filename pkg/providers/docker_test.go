package providers

import (
	"testing"
)

func TestParseImage(t *testing.T) {
	cases := []struct {
		name                      string
		imageURL                  string
		expectedRepo, expectedTag string
		withErr                   bool
	}{
		{name: "no host, no version", imageURL: "postgres", expectedRepo: "library/postgres", expectedTag: "latest"},
		{name: "no host, with version", imageURL: "postgres:1.2.3", expectedRepo: "library/postgres", expectedTag: "1.2.3"},
		{name: "with host, no version", imageURL: "quay.io/calico/node", expectedRepo: "quay.io/calico/node", expectedTag: "latest"},
		{name: "with host, with version", imageURL: "quay.io/calico/node:1.2.3", expectedRepo: "quay.io/calico/node", expectedTag: "1.2.3"},
		{name: "no host, with version and owner", imageURL: "hashicorp/terraform:1.2.3", expectedRepo: "hashicorp/terraform", expectedTag: "1.2.3"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repo, tag, err := parseImage(test.imageURL)
			switch {
			case test.expectedRepo != repo:
				t.Errorf("expected repo was %s, got %s", test.expectedRepo, repo)
			case test.expectedTag != tag:
				t.Errorf("expected tag was %s, got %s", test.expectedTag, tag)
			case test.withErr != (err != nil):
				t.Errorf("expected err != nil to be %v", test.withErr)
			}
		})
	}
}
