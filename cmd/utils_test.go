package cmd

import (
	"testing"

	"github.com/marcosnils/bin/pkg/config"
)

func TestCheckBinExists(t *testing.T) {
	tests := []struct {
		name string
		url  string
		bins map[string]*config.Binary
		want string
	}{
		{
			name: "Should return bin name if binary exists in config",
			url:  "https://github.com/mikefarah/yq",
			bins: map[string]*config.Binary{
				"bin/yq": {
					RemoteName: "yq",
				},
				"bin/terraform": {
					RemoteName: "terraform",
				},
			},
			want: "yq",
		},
		{
			name: "Should empty string if binary doesn't exist in config",
			url:  "https://github.com/mikefarah/notexists",
			bins: map[string]*config.Binary{
				"bin/yq": {
					RemoteName: "yq",
				},
				"bin/terraform": {
					RemoteName: "terraform",
				},
			},
			want: "",
		},
		{
			name: "Should return empty string if url is with version tag",
			url:  "github.com/kubernetes-sigs/kind/releases/tag/v0.8.0",
			bins: map[string]*config.Binary{
				"bin/yq": {
					RemoteName: "yq",
				},
				"bin/kind": {
					RemoteName: "kind",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBinExistsInConfig(tt.url, tt.bins)
			if got != tt.want {
				t.Errorf("checkBinExists() = %v, want %v", got, tt.want)
			}
		})
	}

}
