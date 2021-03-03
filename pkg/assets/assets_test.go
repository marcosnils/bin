package assets

import (
	"runtime"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	type sample struct {
		in  string
		v   string
		out string
	}
	var cases []sample
	//TODO: For sure there will be a better option to divide/adapt tests based on OS
	//with this first iteration we can declare scenarios for windows and other OS
	//without modifiying the core code of the test.
	if runtime.GOOS == "windows" {
		cases = []sample{
			{"bin_0.0.1_Windows_x86_64.exe","0.0.1","bin.exe"},
		}
	} else {
		cases = []sample{
			{"bin_amd64_linux", "v0.0.1", "bin"},
			{"bin_0.0.1_amd64_linux", "0.0.1", "bin"},
			{"bin_0.0.1_amd64_linux", "v0.0.1", "bin"},
			{"gitlab-runner-linux-amd64", "v13.2.1", "gitlab-runner"},
			{"jq-linux64", "jq-1.5", "jq"},

		}
	}

	for _, c := range cases {
		if n := SanitizeName(c.in, c.v); n != c.out {
			t.Fatalf("Error replacing %s: %s does not match %s", c.in, n, c.out)
		}
	}

}

func TestFilterAssets(t *testing.T) {
	type args struct {
		repoName string
		as       []*Asset
	}
	type sample struct {
		in  args
		out string
	}
	var cases []sample
	//TODO: For sure there will be a better option to divide/adapt tests based on OS
	//with this first iteration we can declare scenarios for windows and other OS
	//without modifiying the core code of the test.
	if runtime.GOOS == "windows"{
		cases = []sample {
			{args{"bin", []*Asset{
				{Name: "bin_0.0.1_Windows_x86_64.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Windows_x86_64.exe"},
				{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
				{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
			}}, "bin_0.0.1_windows_x86_64.exe"},
		}
	} else {
		cases = []sample{
			{args{"bin", []*Asset{
				{Name: "bin_0.0.1_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_x86_64"},
				{Name: "bin_0.0.1_Linux_i386", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_i386"},
				{Name: "bin_0.0.1_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Darwin_x86_64"},
			}}, "bin_0.0.1_linux_x86_64"},
			{args{"bin", []*Asset{
				{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
				{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
				{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
			}}, "bin_0.1.0_linux_x86_64"},
			{args{"bin", []*Asset{
				{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
				{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
				{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
			}}, "bin_0.1.0_linux_x86_64"},
			{args{"gitlab-runner", []*Asset{
				{Name: "Windows 64 bits", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-windows-amd64.zip"},
				{Name: "linux amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-linux-amd64"},
				{Name: "macOS", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-darwin-amd64"},
			}}, "gitlab-runner-linux-amd64"},
			{args{"yq", []*Asset{
				{Name: "yq_freebsd_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_freebsd_amd64"},
				{Name: "yq_linux_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_linux_amd64"},
				{Name: "yq_windows_amd64.exe", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_windows_amd64.exe"},
			}}, "yq_linux_amd64"},
			{args{"jq", []*Asset{
				{Name: "jq-win64.exe", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-win64.exe"},
				{Name: "jq-linux64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64"},
				{Name: "jq-osx-amd64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64"},
			}}, "jq-linux64"},
		}
	}

	for _, c := range cases {
		if n, _ := FilterAssets(c.in.repoName, c.in.as); n.Name != c.out {
			t.Fatalf("Error filtering %+v: %+v does not match %s", c.in, n, c.out)
		}
	}

}
