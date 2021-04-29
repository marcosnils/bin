package assets

import (
	"fmt"
	"strings"
	"testing"
)

type mockOSResolver struct {
	OS                   []string
	Arch                 []string
	OSSpecificExtensions []string
}

func (m *mockOSResolver) GetOS() []string {
	return m.OS
}

func (m *mockOSResolver) GetArch() []string {
	return m.Arch
}

func (m *mockOSResolver) GetOSSpecificExtensions() []string {
	return m.OSSpecificExtensions
}

var (
	testLinuxAMDResolver   = &mockOSResolver{OS: []string{"linux"}, Arch: []string{"amd64", "x86_64", "x64", "64"}, OSSpecificExtensions: []string{"AppImage"}}
	testWindowsAMDResolver = &mockOSResolver{OS: []string{"windows", "win"}, Arch: []string{"amd64", "x86_64", "x64", "64"}, OSSpecificExtensions: []string{"exe"}}
)

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		in       string
		v        string
		out      string
		resolver platformResolver
	}{
		{"bin_amd64_linux", "v0.0.1", "bin", testLinuxAMDResolver},
		{"bin_0.0.1_amd64_linux", "0.0.1", "bin", testLinuxAMDResolver},
		{"bin_0.0.1_amd64_linux", "v0.0.1", "bin", testLinuxAMDResolver},
		{"gitlab-runner-linux-amd64", "v13.2.1", "gitlab-runner", testLinuxAMDResolver},
		{"jq-linux64", "jq-1.5", "jq", testLinuxAMDResolver},
		{"launchpad-linux-x64", "1.2.0-rc.1", "launchpad", testLinuxAMDResolver},
		{"launchpad-win-x64.exe", "1.2.0-rc.1", "launchpad.exe", testWindowsAMDResolver},
		{"bin_0.0.1_Windows_x86_64.exe", "0.0.1", "bin.exe", testWindowsAMDResolver},
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
	cases := []struct {
		in       args
		out      string
		resolver platformResolver
	}{
		{args{"bin", []*Asset{
			{Name: "bin_0.0.1_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_x86_64"},
			{Name: "bin_0.0.1_Linux_i386", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Linux_i386"},
			{Name: "bin_0.0.1_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Darwin_x86_64"},
		}}, "bin_0.0.1_Linux_x86_64", testLinuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.1.0_Linux_x86_64", testLinuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.1.0_Windows_i386.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Windows_i386.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.1.0_Linux_x86_64", testLinuxAMDResolver},
		{args{"gitlab-runner", []*Asset{
			{Name: "Windows 64 bits", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-windows-amd64.zip"},
			{Name: "linux amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-linux-amd64"},
			{Name: "macOS", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-darwin-amd64"},
		}}, "gitlab-runner-linux-amd64", testLinuxAMDResolver},
		{args{"yq", []*Asset{
			{Name: "yq_freebsd_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_freebsd_amd64"},
			{Name: "yq_linux_amd64", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_linux_amd64"},
			{Name: "yq_windows_amd64.exe", URL: "https://github.com/mikefarah/yq/releases/download/3.3.2/yq_windows_amd64.exe"},
		}}, "yq_linux_amd64", testLinuxAMDResolver},
		{args{"jq", []*Asset{
			{Name: "jq-win64.exe", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-win64.exe"},
			{Name: "jq-linux64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64"},
			{Name: "jq-osx-amd64", URL: "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64"},
		}}, "jq-linux64", testLinuxAMDResolver},
		{args{"bin", []*Asset{
			{Name: "bin_0.0.1_Windows_x86_64.exe", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.0.1_Windows_x86_64.exe"},
			{Name: "bin_0.1.0_Linux_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Linux_x86_64"},
			{Name: "bin_0.1.0_Darwin_x86_64", URL: "https://github.com/marcosnils/bin/releases/download/v0.0.1/bin_0.1.0_Darwin_x86_64"},
		}}, "bin_0.0.1_Windows_x86_64.exe", testWindowsAMDResolver},
		{args{"tezos", []*Asset{
			{Name: "x86_64-linux-tezos-binaries.tar.gz", URL: "https://gitlab.com/api/v4/projects/3836952/packages/generic/tezos/8.2.0/x86_64-linux-tezos-binaries.tar.gz"},
		}}, "x86_64-linux-tezos-binaries.tar.gz", testLinuxAMDResolver},
		{args{"launchpad", []*Asset{
			{Name: "launchpad-linux-x64", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-linux-x64"},
			{Name: "launchpad-win-x64.exe", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-win-x64.exe"},
		}}, "launchpad-linux-x64", testLinuxAMDResolver},
		{args{"launchpad", []*Asset{
			{Name: "launchpad-linux-x64", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-linux-x64"},
			{Name: "launchpad-win-x64.exe", URL: "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-win-x64.exe"},
		}}, "launchpad-win-x64.exe", testWindowsAMDResolver},
		{args{"Cura", []*Asset{
			{Name: "Ultimaker_Cura-4.7.1-Darwin.dmg", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-Darwin.dmg"},
			{Name: "Ultimaker_Cura-4.7.1-win64.exe", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-win64.exe"},
			{Name: "Ultimaker_Cura-4.7.1-win64.msi", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-win64.msi"},
			{Name: "Ultimaker_Cura-4.7.1.AppImage", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1.AppImage"},
			{Name: "Ultimaker_Cura-4.7.1.AppImage.asc", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1.AppImage.asc"},
		}}, "Ultimaker_Cura-4.7.1.AppImage", testLinuxAMDResolver},
		{args{"Cura", []*Asset{
			{Name: "Ultimaker_Cura-4.7.1-Darwin.dmg", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-Darwin.dmg"},
			{Name: "Ultimaker_Cura-4.7.1-win64.exe", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-win64.exe"},
			{Name: "Ultimaker_Cura-4.7.1-win64.msi", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1-win64.msi"},
			{Name: "Ultimaker_Cura-4.7.1.AppImage", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1.AppImage"},
			{Name: "Ultimaker_Cura-4.7.1.AppImage.asc", URL: "https://github.com/Ultimaker/Cura/releases/download/4.7.1/Ultimaker_Cura-4.7.1.AppImage.asc"},
		}}, "Ultimaker_Cura-4.7.1-win64.exe", testWindowsAMDResolver},
		{args{"usql", []*Asset{
			{Name: "usql-0.8.2-darwin-amd64.tar.bz2", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-darwin-amd64.tar.bz2"},
			{Name: "usql-0.8.2-linux-amd64.tar.bz2", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-linux-amd64.tar.bz2"},
			{Name: "usql-0.8.2-windows-amd64.zip", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-windows-amd64.zip"},
		}}, "usql-0.8.2-linux-amd64.tar.bz2", testLinuxAMDResolver},
		{args{"usql", []*Asset{
			{Name: "usql-0.8.2-darwin-amd64.tar.bz2", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-darwin-amd64.tar.bz2"},
			{Name: "usql-0.8.2-linux-amd64.tar.bz2", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-linux-amd64.tar.bz2"},
			{Name: "usql-0.8.2-windows-amd64.zip", URL: "https://github.com/xo/usql/releases/download/v0.8.2/usql-0.8.2-windows-amd64.zip"},
		}}, "usql-0.8.2-windows-amd64.zip", testWindowsAMDResolver},
		{args{"cli", []*Asset{
			{Name: "dapr", URL: ""},
		}}, "dapr", testLinuxAMDResolver},
	}

	f := NewFilter(&FilterOpts{})
	for _, c := range cases {
		resolver = c.resolver
		if n, err := f.FilterAssets(c.in.repoName, c.in.as); err != nil {
			t.Fatalf("Error filtering assets %v", err)
		} else if n.Name != c.out {
			t.Fatalf("Error filtering %+v: %+v does not match %s", c.in, n, c.out)
		}
	}

}

func TestIsSupportedExt(t *testing.T) {
	cases := []struct {
		in  string
		out bool
	}{
		{
			"Ultimaker_Cura-4.8.0.AppImage",
			true,
		},
		{
			"Ultimaker_Cura-4.7.1-win64.msi",
			false,
		},
	}

	for _, c := range cases {
		result := isSupportedExt(c.in)
		if result != c.out {
			t.Fatalf("Expected result for extension %v to be %v, but got result %v", c.in, c.out, result)
		}
	}

}
