package assets

import (
	"fmt"
	"strings"
	"testing"
)

type mockOSResolver struct {
	OS   []string
	Arch []string
}

func (m *mockOSResolver) GetOS() []string {
	return m.OS
}

func (m *mockOSResolver) GetArch() []string {
	return m.Arch
}

func TestSanitizeName(t *testing.T) {
	linuxAMDResolver := &mockOSResolver{OS: []string{"linux"}, Arch: []string{"amd64", "x86_64", "x64", "64"}}
	windowsAMDResolver := &mockOSResolver{OS: []string{"windows", "win"}, Arch: []string{"amd64", "x86_64", "x64", "64"}}
	cases := []struct {
		in       string
		v        string
		out      string
		resolver platformResolver
	}{
		{"bin_amd64_linux", "v0.0.1", "bin", linuxAMDResolver},
		{"bin_0.0.1_amd64_linux", "0.0.1", "bin", linuxAMDResolver},
		{"bin_0.0.1_amd64_linux", "v0.0.1", "bin", linuxAMDResolver},
		{"gitlab-runner-linux-amd64", "v13.2.1", "gitlab-runner", linuxAMDResolver},
		{"jq-linux64", "jq-1.5", "jq", linuxAMDResolver},
		{"launchpad-linux-x64", "1.2.0-rc.1", "launchpad", linuxAMDResolver},
		{"launchpad-win-x64.exe", "1.2.0-rc.1", "launchpad.exe", windowsAMDResolver},
		{"bin_0.0.1_Windows_x86_64.exe", "0.0.1", "bin.exe", windowsAMDResolver},
	}

	for _, c := range cases {
		resolver = c.resolver
		if n := SanitizeName(c.in, c.v); n != c.out {
			t.Fatalf("Error replacing %s: %s does not match %s", c.in, n, c.out)
		}
	}

}

type args struct {
	repoName string
	as       []*Asset
}

func (a args) String() string {
	assetStrings := []string{}
	for _, asset := range a.as {
		assetStrings = append(assetStrings, asset.String())
	}
	return fmt.Sprintf("%s (%v)", a.repoName, strings.Join(assetStrings, ","))
}

func TestFilterAssets(t *testing.T) {
	linuxAMDResolver := &mockOSResolver{OS: []string{"linux"}, Arch: []string{"amd64", "x86_64", "x64", "64"}}
	windowsAMDResolver := &mockOSResolver{OS: []string{"windows", "win"}, Arch: []string{"amd64", "x86_64", "x64", "64"}}
	cases := []struct {
		in       args
		out      string
		resolver platformResolver
	}{
		{args{"bin", []*Asset{
			{Name: "bin_0.0.1_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_x86_64"},
			{Name: "bin_0.0.1_Linux_i386", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_i386"},
			{Name: "bin_0.0.1_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Darwin_x86_64"},
		}}, "bin_0.0.1_linux_x86_64", linuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.1.0_linux_x86_64", linuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.1.0_linux_x86_64", linuxAMDResolver},
		{args{"gitlab-runner", []*Asset{
			{Name: "Windows 64 bits", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-windows-amd64.zip"},
			{Name: "linux amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-linux-amd64"},
			{Name: "macOS", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-darwin-amd64"},
		}}, "gitlab-runner-linux-amd64", linuxAMDResolver},
		{args{"yq", []*Asset{
			{Name: "yq_freebsd_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_freebsd_amd64"},
			{Name: "yq_linux_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_linux_amd64"},
			{Name: "yq_windows_amd64.exe", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_windows_amd64.exe"},
		}}, "yq_linux_amd64", linuxAMDResolver},
		{args{"jq", []*Asset{
			{Name: "jq-win64.exe", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-win64.exe"},
			{Name: "jq-linux64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64"},
			{Name: "jq-osx-amd64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64"},
		}}, "jq-linux64", linuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.0.1_Windows_x86_64.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Windows_x86_64.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.0.1_windows_x86_64.exe", windowsAMDResolver},
		{args{"tezos", []*Asset{
			{Name: "x86_64-linux-tezos-binaries.tar.gz", URL: "https://gitlab.com/api/v4/projects/3836952/packages/generic/tezos/8.2.0/x86_64-linux-tezos-binaries.tar.gz"},
		}}, "x86_64-linux-tezos-binaries.tar.gz", linuxAMDResolver},
		{args{"launchpad", []*Asset{
			{Name: "launchpad-linux-x64", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-linux-x64"},
			{Name: "launchpad-win-x64.exe", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-win-x64.exe"},
		}}, "launchpad-linux-x64", linuxAMDResolver},
		{args{"launchpad", []*Asset{
			{Name: "launchpad-linux-x64", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-linux-x64"},
			{Name: "launchpad-win-x64.exe", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-win-x64.exe"},
		}}, "launchpad-win-x64.exe", windowsAMDResolver},
	}

	for _, c := range cases {
		resolver = c.resolver
		if n, err := FilterAssets(c.in.repoName, c.in.as); err != nil {
			t.Fatalf("Error filtering assets %v", err)
		} else if n.Name != c.out {
			t.Fatalf("Error filtering %+v: %+v does not match %s", c.in, n, c.out)
		}
	}

}
