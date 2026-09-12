package providers

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoInstallURL(t *testing.T) {
	cases := []struct {
		name, url, importPath, ref string
	}{
		{"no ref", "goinstall://github.com/foo/bar", "github.com/foo/bar", "latest"},
		{"with tag", "goinstall://github.com/foo/bar@v1.2.3", "github.com/foo/bar", "v1.2.3"},
		{"subpackage with tag", "goinstall://github.com/foo/bar/cmd/baz@v1.0", "github.com/foo/bar/cmd/baz", "v1.0"},
		{"branch", "goinstall://github.com/foo/bar@main", "github.com/foo/bar", "main"},
		{"major version suffix", "goinstall://github.com/foo/bar/v2", "github.com/foo/bar/v2", "latest"},
		{"no scheme", "github.com/foo/bar@v1", "github.com/foo/bar", "v1"},
		{"empty ref", "goinstall://github.com/foo/bar@", "github.com/foo/bar", "latest"},
		{"trailing slash", "goinstall://github.com/foo/bar/", "github.com/foo/bar", "latest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			importPath, ref := parseGoInstallURL(tc.url)
			if importPath != tc.importPath {
				t.Errorf("importPath: want %q, got %q", tc.importPath, importPath)
			}
			if ref != tc.ref {
				t.Errorf("ref: want %q, got %q", tc.ref, ref)
			}
		})
	}
}

func TestNewGoInstall(t *testing.T) {
	p, err := newGoInstall("goinstall://github.com/foo/bar/cmd/baz@v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gi, ok := p.(*goinstall)
	if !ok {
		t.Fatalf("expected *goinstall, got %T", p)
	}
	if gi.importPath != "github.com/foo/bar/cmd/baz" || gi.ref != "v1.0" {
		t.Errorf("unexpected parse: %+v", gi)
	}
	if gi.url != "goinstall://github.com/foo/bar/cmd/baz@v1.0" {
		t.Errorf("original url not kept: %q", gi.url)
	}
	if gi.GetID() != "goinstall" {
		t.Errorf("expected id goinstall, got %s", gi.GetID())
	}
	if _, err := newGoInstall("goinstall://"); err == nil {
		t.Error("expected error for empty path")
	}
}

// stubGo routes goCommand through this test binary (see TestMain) so the
// provider can be exercised without a toolchain or network. The stub
// records every invocation in a log file under dir.
func stubGo(t *testing.T) (calls func() []string) {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	orig := goCommand
	goCommand = func(args ...string) *exec.Cmd {
		cmd := exec.Command(exe, args...)
		cmd.Env = append(os.Environ(), "BIN_TEST_GO_STUB=1", "BIN_TEST_GO_LOG="+logFile)
		return cmd
	}
	t.Cleanup(func() { goCommand = orig })
	return func() []string {
		b, _ := os.ReadFile(logFile)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

// goStubMain emulates the subset of the go toolchain the provider uses.
// `go list -m` answers for a fixed set of modules; `go install` copies this
// test binary (which carries real build info) into GOBIN.
func goStubMain() {
	args := os.Args[1:]
	if f, err := os.OpenFile(os.Getenv("BIN_TEST_GO_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	modules := map[string]string{
		"github.com/foo/bar@latest": "v1.5.0",
		"github.com/foo/bar@main":   "v1.5.1-0.20260101000000-abcdef123456",
		"github.com/foo/bar@v1.2.3": "v1.2.3",
	}
	switch {
	case len(args) >= 5 && args[0] == "list" && args[1] == "-m":
		target := args[4]
		version, ok := modules[target]
		if !ok {
			fmt.Fprintf(os.Stderr, "go: module %s: not found\n", target)
			os.Exit(1)
		}
		switch args[3] {
		case "{{.Path}}":
			fmt.Println(strings.SplitN(target, "@", 2)[0])
		case "{{.Version}}":
			fmt.Println(version)
		}
	case len(args) == 2 && args[0] == "install":
		if strings.Contains(args[1], "broken") {
			fmt.Fprintln(os.Stderr, "go: github.com/foo/broken@latest: module not found")
			os.Exit(1)
		}
		exe, _ := os.Executable()
		src, err := os.Open(exe)
		if err != nil {
			os.Exit(1)
		}
		defer src.Close()
		name := filepath.Base(strings.SplitN(args[1], "@", 2)[0])
		dst, err := os.Create(filepath.Join(os.Getenv("GOBIN"), name))
		if err != nil {
			os.Exit(1)
		}
		if _, err := io.Copy(dst, src); err != nil {
			os.Exit(1)
		}
		dst.Close()
	default:
		fmt.Fprintf(os.Stderr, "stub: unsupported args %v\n", args)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestMain(m *testing.M) {
	if os.Getenv("BIN_TEST_GO_STUB") == "1" {
		goStubMain()
	}
	os.Exit(m.Run())
}

func TestGoInstallFetch(t *testing.T) {
	calls := stubGo(t)
	p, _ := newGoInstall("goinstall://github.com/foo/bar/cmd/baz")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "baz" {
		t.Errorf("name: want baz, got %q", f.Name)
	}
	// the stub copies this test binary, so the version is whatever its
	// build info says; it must at least be populated
	if f.Version == "" {
		t.Error("version not read from build info")
	}
	data, err := io.ReadAll(f.Data)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(data)) != f.Length || f.Length == 0 {
		t.Errorf("length: want %d, got %d", f.Length, len(data))
	}
	tr := f.Data.(*tempDirReader)
	if _, err := os.Stat(tr.dir); !os.IsNotExist(err) {
		t.Errorf("temp dir %s not removed after read", tr.dir)
	}
	if got := calls(); len(got) != 1 || got[0] != "install github.com/foo/bar/cmd/baz@latest" {
		t.Errorf("unexpected go calls: %v", got)
	}
}

func TestGoInstallFetchUsesOptsVersion(t *testing.T) {
	calls := stubGo(t)
	p, _ := newGoInstall("goinstall://github.com/foo/bar")
	f, err := p.Fetch(&FetchOpts{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	_, _ = io.Copy(io.Discard, f.Data)
	if got := calls(); len(got) != 1 || got[0] != "install github.com/foo/bar@v1.2.3" {
		t.Errorf("unexpected go calls: %v", got)
	}
}

func TestGoInstallFetchError(t *testing.T) {
	stubGo(t)
	p, _ := newGoInstall("goinstall://github.com/foo/broken")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "module not found") {
		t.Fatalf("expected toolchain stderr in error, got %v", err)
	}
}

func TestGoInstallGetLatestVersionFromBuildInfo(t *testing.T) {
	calls := stubGo(t)
	exe, _ := os.Executable()
	// the test binary's build info names this repo's module, so seed the
	// stub's table with it via a fresh provider pointing at a subpackage
	orig := goCommand
	goCommand = func(args ...string) *exec.Cmd {
		for i, a := range args {
			if strings.HasPrefix(a, "github.com/marcosnils/bin@") {
				args[i] = "github.com/foo/bar@" + strings.SplitN(a, "@", 2)[1]
			}
		}
		return orig(args...)
	}

	p, _ := newGoInstall("goinstall://github.com/marcosnils/bin/pkg/providers@main")
	p.(BinaryPathSetter).SetBinaryPath(exe)
	v, u, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "v1.5.1-0.20260101000000-abcdef123456" {
		t.Errorf("version: got %q", v)
	}
	if u != "goinstall://github.com/marcosnils/bin/pkg/providers@main" {
		t.Errorf("url must be the original goinstall url, got %q", u)
	}
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "list -m -f {{.Version}} ") {
		t.Errorf("expected a single go list call, got %v", got)
	}
}

func TestGoInstallGetLatestVersionWalksUpWithoutBinary(t *testing.T) {
	calls := stubGo(t)
	p, _ := newGoInstall("goinstall://github.com/foo/bar/cmd/baz")
	v, u, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "v1.5.0" {
		t.Errorf("version: got %q", v)
	}
	if u != "goinstall://github.com/foo/bar/cmd/baz" {
		t.Errorf("url: got %q", u)
	}
	want := []string{
		"list -m -f {{.Path}} github.com/foo/bar/cmd/baz@latest",
		"list -m -f {{.Path}} github.com/foo/bar/cmd@latest",
		"list -m -f {{.Path}} github.com/foo/bar@latest",
		"list -m -f {{.Version}} github.com/foo/bar@latest",
	}
	if got := calls(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("go calls:\n got %v\nwant %v", got, want)
	}
}

func TestGoInstallGetLatestVersionUnknownModule(t *testing.T) {
	stubGo(t)
	p, _ := newGoInstall("goinstall://example.com/nope/x")
	if _, _, err := p.GetLatestVersion(); err == nil || !strings.Contains(err.Error(), "could not find a module") {
		t.Fatalf("expected walk-up failure, got %v", err)
	}
}

func TestGoInstallBinaryWithoutBuildInfoFallsBack(t *testing.T) {
	calls := stubGo(t)
	notABinary := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(notABinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := newGoInstall("goinstall://github.com/foo/bar@v1.2.3")
	p.(BinaryPathSetter).SetBinaryPath(notABinary)
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "v1.2.3" {
		t.Errorf("version: got %q", v)
	}
	if got := calls(); len(got) != 2 {
		t.Errorf("expected path walk-up then version lookup, got %v", got)
	}
}
