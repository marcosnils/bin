package assets

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
			{Name: "gitlab-runner-windows-amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-windows-amd64.zip"},
			{Name: "gitlab-runner-linux-amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-linux-amd64"},
			{Name: "gitlab-runner-darwin-amd64", URL: "https://gitlab-runner-downloads.s3.amazonaws.com/v13.2.1/binaries/gitlab-runner-darwin-amd64"},
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

	f := NewFilter(&FilterOpts{SkipScoring: false})
	for _, c := range cases {
		resolver = c.resolver
		if n, err := f.FilterAssets(c.in.repoName, c.in.as); err != nil {
			for _, a := range c.in.as {
				fmt.Println(a.Name, c.resolver)
			}
			t.Fatalf("Error filtering assets %v", err)
		} else if n.Name != c.out {
			t.Fatalf("Error filtering %+v: %+v does not match %s", c.in, n, c.out)
		}
	}

}

func TestFilterAssetsNamePattern(t *testing.T) {
	as := []*Asset{
		{Name: "foo-linux-amd64"},
		{Name: "foo-darwin-amd64"},
		{Name: "foo-windows-amd64.exe"},
		{Name: "bar-linux-amd64"},
	}

	cases := []struct {
		pattern  string
		wantName string
		wantErr  bool
	}{
		{"foo-linux*", "foo-linux-amd64", false},
		{"bar*", "bar-linux-amd64", false},
		{"baz*", "", true}, // no match
	}

	for _, c := range cases {
		f := NewFilter(&FilterOpts{NamePattern: c.pattern})
		got, err := f.FilterAssets("repo", as)
		if c.wantErr {
			if err == nil {
				t.Errorf("pattern %q: expected error, got nil", c.pattern)
			}
			continue
		}
		if err != nil {
			t.Errorf("pattern %q: unexpected error: %v", c.pattern, err)
			continue
		}
		if got.Name != c.wantName {
			t.Errorf("pattern %q: got %q, want %q", c.pattern, got.Name, c.wantName)
		}
	}
}

// TestFilterAssetsPreferred verifies that, on upgrades, the artefact chosen
// previously is offered as the prompt default even though release asset names
// embed the (changing) version. The prompt still appears; with no input (EOF in
// the test) SelectWithDefault returns the default, so FilterAssets yields the
// previously selected artefact.
func TestFilterAssetsPreferred(t *testing.T) {
	resolver = testLinuxAMDResolver

	cases := []struct {
		name             string
		as               []*Asset
		preferredAsset   string
		preferredVersion string
		currentVersion   string
		want             string
	}{
		{
			// musl and gnu variants score identically (both linux+amd64); the
			// previous musl choice is re-selected across the version bump.
			name: "re-selects same variant across versions",
			as: []*Asset{
				{Name: "tool-1.1.0-linux-amd64-musl.tar.gz"},
				{Name: "tool-1.1.0-linux-amd64-gnu.tar.gz"},
			},
			preferredAsset:   "tool-1.0.0-linux-amd64-musl.tar.gz",
			preferredVersion: "1.0.0",
			currentVersion:   "1.1.0",
			want:             "tool-1.1.0-linux-amd64-musl.tar.gz",
		},
		{
			// preference overrides scoring: the raw binary and the archive both
			// match the platform, but the previously selected archive wins.
			name: "preference overrides scoring tie",
			as: []*Asset{
				{Name: "tool_1.1.0_linux_amd64"},
				{Name: "tool_1.1.0_linux_amd64.tar.gz"},
			},
			preferredAsset:   "tool_1.0.0_linux_amd64.tar.gz",
			preferredVersion: "1.0.0",
			currentVersion:   "1.1.0",
			want:             "tool_1.1.0_linux_amd64.tar.gz",
		},
	}

	for _, c := range cases {
		f := NewFilter(&FilterOpts{
			PreferredAsset:   c.preferredAsset,
			PreferredVersion: c.preferredVersion,
			CurrentVersion:   c.currentVersion,
		})
		got, err := f.FilterAssets("tool", c.as)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.name, err)
		}
		if got.Name != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got.Name, c.want)
		}
	}
}

// TestDefaultIndex verifies the default-selection helper used to pre-select the
// previously used artefact in the interactive prompt.
func TestDefaultIndex(t *testing.T) {
	resolver = testLinuxAMDResolver

	opts := []fmt.Stringer{
		&FilteredAsset{Name: "tool-2.0.0-linux-amd64-gnu.tar.gz"},
		&FilteredAsset{Name: "tool-2.0.0-linux-amd64-musl.tar.gz"},
	}

	// Preferred musl variant from a previous version resolves to index 1.
	want := SanitizeName("tool-1.0.0-linux-amd64-musl.tar.gz", "1.0.0")
	if got := defaultIndex(opts, want, "2.0.0"); got != 1 {
		t.Errorf("defaultIndex match: got %d, want 1", got)
	}

	// No preference yields no default.
	if got := defaultIndex(opts, "", "2.0.0"); got != -1 {
		t.Errorf("defaultIndex no-preference: got %d, want -1", got)
	}

	// A preference that no longer exists yields no default.
	gone := SanitizeName("tool-1.0.0-linux-amd64-static.tar.gz", "1.0.0")
	if got := defaultIndex(opts, gone, "2.0.0"); got != -1 {
		t.Errorf("defaultIndex missing: got %d, want -1", got)
	}
}

// makeTar builds an in-memory tar archive where every entry has mode 0755.
func makeTar(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		data := []byte(content)
		_ = tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0755})
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	return buf.Bytes()
}

// makeZip builds an in-memory zip archive where every entry has mode 0755.
func makeZip(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		fh := &zip.FileHeader{Name: name, Method: zip.Store}
		fh.SetMode(0755)
		w, _ := zw.CreateHeader(fh)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestProcessTarNamePattern(t *testing.T) {
	data := makeTar(map[string]string{
		"tool-v1.0/mytool": "mytool binary",
		"tool-v1.0/helper": "helper binary",
	})

	cases := []struct {
		pattern  string
		wantName string
		wantErr  bool
	}{
		{"foo*/mytool", "mytool", false},           // match by basename
		{"foo*/tool-v1.0/mytool", "mytool", false}, // match by full path
		{"foo*/missing", "", true},                 // no match
	}

	for _, c := range cases {
		f := NewFilter(&FilterOpts{NamePattern: c.pattern})
		f.namePatternUsed = true // simulate top-level asset already selected
		result, err := f.processTar("repo", bytes.NewReader(data))
		if c.wantErr {
			if err == nil {
				t.Errorf("pattern %q: expected error, got nil", c.pattern)
			}
			continue
		}
		if err != nil {
			t.Errorf("pattern %q: unexpected error: %v", c.pattern, err)
			continue
		}
		if result.Name != c.wantName {
			t.Errorf("pattern %q: got name %q, want %q", c.pattern, result.Name, c.wantName)
		}
	}
}

func TestProcessZipNamePattern(t *testing.T) {
	data := makeZip(map[string]string{
		"tool-v1.0/mytool": "mytool binary",
		"tool-v1.0/helper": "helper binary",
	})

	cases := []struct {
		pattern  string
		wantName string
		wantErr  bool
	}{
		{"foo*/mytool", "mytool", false},           // match by basename
		{"foo*/tool-v1.0/mytool", "mytool", false}, // match by full path
		{"foo*/missing", "", true},                 // no match
	}

	for _, c := range cases {
		f := NewFilter(&FilterOpts{NamePattern: c.pattern})
		f.namePatternUsed = true // simulate top-level asset already selected
		result, err := f.processZip("repo", bytes.NewReader(data))
		if c.wantErr {
			if err == nil {
				t.Errorf("pattern %q: expected error, got nil", c.pattern)
			}
			continue
		}
		if err != nil {
			t.Errorf("pattern %q: unexpected error: %v", c.pattern, err)
			continue
		}
		if result.Name != c.wantName {
			t.Errorf("pattern %q: got name %q, want %q", c.pattern, result.Name, c.wantName)
		}
	}
}

// TestProcessTarPackagePathAcrossVersions verifies that a stored package path
// embedding the release version (e.g. ecapture-v2.5.2-linux-amd64/ecapture)
// still matches the corresponding entry of a newer release by comparing
// version-stripped forms.
func TestProcessTarPackagePathAcrossVersions(t *testing.T) {
	resolver = testLinuxAMDResolver

	data := makeTar(map[string]string{
		"tool-v2.5.3-linux-amd64/tool":   "tool binary",
		"tool-v2.5.3-linux-amd64/helper": "helper binary",
	})

	f := NewFilter(&FilterOpts{
		PackagePath:      "tool-v2.5.2-linux-amd64/tool",
		PreferredVersion: "v2.5.2",
		CurrentVersion:   "v2.5.3",
	})
	result, err := f.processTar("repo", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PackagePath != "tool-v2.5.3-linux-amd64/tool" {
		t.Errorf("got package path %q, want %q", result.PackagePath, "tool-v2.5.3-linux-amd64/tool")
	}
	if result.Name != "tool" {
		t.Errorf("got name %q, want %q", result.Name, "tool")
	}
}

// TestProcessZipPackagePathAcrossVersions is the zip counterpart of
// TestProcessTarPackagePathAcrossVersions.
func TestProcessZipPackagePathAcrossVersions(t *testing.T) {
	resolver = testLinuxAMDResolver

	data := makeZip(map[string]string{
		"tool-v2.5.3-linux-amd64/tool":   "tool binary",
		"tool-v2.5.3-linux-amd64/helper": "helper binary",
	})

	f := NewFilter(&FilterOpts{
		PackagePath:      "tool-v2.5.2-linux-amd64/tool",
		PreferredVersion: "v2.5.2",
		CurrentVersion:   "v2.5.3",
	})
	result, err := f.processZip("repo", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PackagePath != "tool-v2.5.3-linux-amd64/tool" {
		t.Errorf("got package path %q, want %q", result.PackagePath, "tool-v2.5.3-linux-amd64/tool")
	}
	if result.Name != "tool" {
		t.Errorf("got name %q, want %q", result.Name, "tool")
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
