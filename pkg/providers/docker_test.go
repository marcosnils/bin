package providers

import "testing"

func TestGetImageDesc(t *testing.T) {
	cases := []struct {
		name            string
		path            string
		expectedOwner   string
		expectedName    string
		expectedVersion string
	}{
		{"no owner no version", "/alpine", "library", "alpine", "latest"},
		{"no owner with version", "/alpine:3.0.9", "library", "alpine", "3.0.9"},
		{"with owner and no version", "/hashicorp/terraform", "hashicorp", "terraform", "latest"},
		{"with owner with version", "/hashicorp/terraform:light", "hashicorp", "terraform", "light"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			owner, name, version := getImageDesc(test.path)
			switch {
			case test.expectedOwner != owner:
				t.Errorf("expected owner was %s got %s", test.expectedOwner, owner)
			case test.expectedName != name:
				t.Errorf("expected name was %s got %s", test.expectedName, name)
			case test.expectedVersion != version:
				t.Errorf("expected version was %s got %s", test.expectedVersion, version)
			}
		})
	}
}
