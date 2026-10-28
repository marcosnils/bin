package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCargoInstallURL(t *testing.T) {
	cases := []struct {
		name, url, crateName, ref string
	}{
		{"no ref", "cargoinstall://ripgrep", "ripgrep", "latest"},
		{"with version", "cargoinstall://ripgrep@14.1.0", "ripgrep", "14.1.0"},
		{"caret", "cargoinstall://ripgrep@^14.1.0", "ripgrep", "^14.1.0"},
		{"tilde", "cargoinstall://ripgrep@~14.1.2", "ripgrep", "~14.1.2"},
		{"star", "cargoinstall://ripgrep@*", "ripgrep", "*"}, // normalized to latest in newCargoInstall
		{"pinned", "cargoinstall://ripgrep@=14.1.0", "ripgrep", "=14.1.0"},
		{"empty ref", "cargoinstall://ripgrep@", "ripgrep", "latest"},
		{"trailing slash", "cargoinstall://ripgrep/", "ripgrep", "latest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			crateName, ref := parseCargoInstallURL(tc.url)
			if crateName != tc.crateName {
				t.Errorf("crateName: want %q, got %q", tc.crateName, crateName)
			}
			if ref != tc.ref {
				t.Errorf("ref: want %q, got %q", tc.ref, ref)
			}
		})
	}
}

func TestNewCargoInstall(t *testing.T) {
	p, err := newCargoInstall("cargoinstall://ripgrep@=14.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, ok := p.(*cargoInstall)
	if !ok {
		t.Fatalf("expected *cargoInstall, got %T", p)
	}
	if c.crateName != "ripgrep" || c.ref != "=14.1.0" {
		t.Errorf("unexpected parse: %+v", c)
	}
	if c.url != "cargoinstall://ripgrep@=14.1.0" {
		t.Errorf("original url not kept: %q", c.url)
	}
	if c.GetID() != "cargoinstall" {
		t.Errorf("expected id cargoinstall, got %s", c.GetID())
	}
	if _, err := newCargoInstall("cargoinstall://"); err == nil {
		t.Error("expected error for empty crate name")
	}
	// refs cargo itself rejects must not reach the toolchain
	for _, bad := range []string{"14", "14.1", ">=14.1", "14.x"} {
		if _, err := newCargoInstall("cargoinstall://ripgrep@" + bad); err == nil {
			t.Errorf("expected error for ref %q", bad)
		}
	}
}

func TestNewCargoInstallRejectsInvalidCrateName(t *testing.T) {
	// names crates.io cannot serve: a leading dash would turn into a cargo
	// flag, the rest would corrupt the registry URL
	for _, bad := range []string{
		"--git=https://example.com/x",
		"-v",
		"rip grep",
		"ripgrep?x=1",
		"ripgrep/x",
		"ripgrep#frag",
	} {
		if _, err := newCargoInstall("cargoinstall://" + bad); err == nil {
			t.Errorf("expected error for crate name %q", bad)
		}
	}
	for _, ok := range []string{"ripgrep", "anyhow", "serde_json", "cargo-edit", "A1"} {
		if _, err := newCargoInstall("cargoinstall://" + ok); err != nil {
			t.Errorf("unexpected error for crate name %q: %v", ok, err)
		}
	}
}

// stubCratesIO serves a registry entry for the given crate for the
// duration of the test, 404ing every other name.
func stubCratesIO(t *testing.T, crateName string, info *cratesIOCrate) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/crates/"+crateName) {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("crates.io API requires a User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })
}

// fakeRipgrep is the registry entry used by most tests: stable 14.1.1 and
// 14.1.2, a yanked 14.0.0 and a 15.0.0 prerelease. The order is shuffled
// on purpose: the provider must not depend on registry ordering.
func fakeRipgrep() *cratesIOCrate {
	c := &cratesIOCrate{
		Versions: []cratesIOVersion{
			{Num: "14.1.1"},
			{Num: "15.0.0-beta.1"},
			{Num: "14.1.2"},
			{Num: "14.1.3+build.5"},
			{Num: "14.0.0", Yanked: true},
		},
	}
	c.Crate.MaxStableVersion = "14.1.2"
	return c
}

// stubCargo routes cargoCommand through this test binary (see
// cargoinstallStubMain, wired up in TestMain) so the provider can be
// exercised without a toolchain or network. The stub records every
// invocation in a log file under a temp dir.
func stubCargo(t *testing.T) (calls func() []string) {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	orig := cargoCommand
	cargoCommand = func(args ...string) *exec.Cmd {
		cmd := exec.Command(exe, args...)
		cmd.Env = append(os.Environ(), "BIN_TEST_CARGOINSTALL_STUB=1", "BIN_TEST_CARGOINSTALL_LOG="+logFile)
		return cmd
	}
	t.Cleanup(func() { cargoCommand = orig })
	return func() []string {
		b, _ := os.ReadFile(logFile)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

// cargoinstallStubMain emulates the subset of the cargo toolchain the
// provider uses: `cargo install <crate>@<req> --root <dir>` writes a fake
// binary into <dir>/bin plus a .crates2.json recording the version the
// req resolved to (pins install verbatim, everything else the highest
// 14.1.x). The binary keeps the crate name verbatim, hyphens included, as
// cargo does when the package declares no explicit [[bin]] name. Crates named
// "nobin" install nothing, and "nomanifest" skips .crates2.json, so the
// provider's error paths can be exercised.
func cargoinstallStubMain() {
	args := os.Args[1:]
	if f, err := os.OpenFile(os.Getenv("BIN_TEST_CARGOINSTALL_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	if len(args) != 4 || args[0] != "install" || args[2] != "--root" {
		fmt.Fprintf(os.Stderr, "stub: unsupported args %v\n", args)
		os.Exit(2)
	}
	crate, spec, _ := strings.Cut(args[1], "@")
	if strings.Contains(crate, "quiet-broken") {
		// fails silently: no stderr to relay
		os.Exit(1)
	}
	if strings.Contains(crate, "broken") {
		fmt.Fprintln(os.Stderr, "error: failed to select a version for the request `broken = _`")
		os.Exit(1)
	}
	version := spec
	switch {
	case spec == "":
		// no @version: cargo resolves latest itself, without the HTTP API
		version = "14.1.2"
	case strings.HasPrefix(spec, "^"):
		// caret over 14.1 resolves to the newest 14.x in the stub's world
		version = "14.1.2"
	}
	binDir := filepath.Join(args[3], "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		os.Exit(1)
	}
	switch crate {
	case "nobin", "dironly":
		// a crate that builds no binaries at all
	case "multi":
		for _, name := range []string{"alpha", "beta"} {
			if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
				os.Exit(1)
			}
		}
	default:
		if err := os.WriteFile(filepath.Join(binDir, crate), []byte("#!/bin/sh\n"), 0o755); err != nil {
			os.Exit(1)
		}
	}
	if crate == "nomanifest" {
		os.Exit(0)
	}
	if crate == "dironly" {
		// a directory in bin/ is not a binary
		if err := os.MkdirAll(filepath.Join(binDir, "subdir"), 0o755); err != nil {
			os.Exit(1)
		}
		manifest := `{"installs":{"dironly 1.0.0 (registry+x)":{}}}`
		if err := os.WriteFile(filepath.Join(args[3], ".crates2.json"), []byte(manifest), 0o644); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if crate == "othername" {
		// installs a binary but records a different package in the manifest
		manifest := `{"installs":{"something-else 9.9.9 (registry+x)":{}}}`
		if err := os.WriteFile(filepath.Join(args[3], ".crates2.json"), []byte(manifest), 0o644); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	manifest := fmt.Sprintf(`{"installs":{"%s %s (registry+https://github.com/rust-lang/crates.io-index)":{}}}`,
		crate, strings.TrimPrefix(version, "="))
	if err := os.WriteFile(filepath.Join(args[3], ".crates2.json"), []byte(manifest), 0o644); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// A bare triplet is an exact version in cargo, not a caret req, so it must
// behave exactly like an explicit pin: same argument passed through, and no
// registry round-trip on update.
func TestCargoFetchBareVersionIsExact(t *testing.T) {
	calls := stubCargo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a bare version must not hit the registry, got %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })

	p, _ := newCargoInstall("cargoinstall://ripgrep@14.1.0")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Version != "14.1.0" {
		t.Errorf("version: want 14.1.0, got %q", f.Version)
	}
	_, _ = io.Copy(io.Discard, f.Data)
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep@14.1.0 --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}

	// the same version an update resolves to: treating a bare triplet as a
	// caret req here made GetLatestVersion report a newer version, so update
	// reinstalled the pinned one forever while claiming it had moved on
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "14.1.0" {
		t.Errorf("GetLatestVersion: want 14.1.0, got %q", v)
	}
}

// latest must not need the crates.io HTTP API: cargo resolves it from the
// index it already has, so mirrored and air-gapped setups keep working.
func TestCargoFetchLatestNeedsNoRegistry(t *testing.T) {
	calls := stubCargo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("install of latest must not hit the crates.io API, got %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })

	p, _ := newCargoInstall("cargoinstall://ripgrep")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Version != "14.1.2" {
		t.Errorf("version: want 14.1.2 from the manifest, got %q", f.Version)
	}
	_, _ = io.Copy(io.Discard, f.Data)
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}
}

// The API tolerates aliases that cargo refuses, so a raw "could not find"
// should point at the canonical name instead of leaving the user guessing.
func TestCargoFetchAliasHint(t *testing.T) {
	stubCargo(t)
	info := fakeRipgrep()
	info.Crate.Name = "ripgrep-cli"
	stubCratesIO(t, "ripgrep-alias", info)

	p, _ := newCargoInstall("cargoinstall://ripgrep-alias@=1.0.0")
	// make cargo reject the name the way it does for a hyphen/underscore alias
	cargoCommandOrig := cargoCommand
	cargoCommand = func(args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo 'error: could not find `ripgrep-alias` in registry `crates-io` with version `=1.0.0`' >&2; exit 1")
	}
	t.Cleanup(func() { cargoCommand = cargoCommandOrig })

	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), `crates.io knows this crate as "ripgrep-cli"`) {
		t.Fatalf("expected an alias hint, got %v", err)
	}
}

// A build failure is not a name mismatch, so it must not pay for a registry
// round-trip: offline that would add the full fetch timeout to an install that
// already failed.
func TestCargoFetchBuildFailureSkipsAliasHint(t *testing.T) {
	stubCargo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a build failure must not query crates.io, got %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })

	p, _ := newCargoInstall("cargoinstall://ripgrep@=1.0.0")
	cargoCommandOrig := cargoCommand
	cargoCommand = func(args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo 'error[E0308]: mismatched types' >&2; exit 1")
	}
	t.Cleanup(func() { cargoCommand = cargoCommandOrig })

	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "mismatched types") {
		t.Fatalf("expected the cargo error, got %v", err)
	}
}

func TestCargoFetchPinnedVersion(t *testing.T) {
	calls := stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://ripgrep@=14.1.0")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "ripgrep" {
		t.Errorf("name: want ripgrep, got %q", f.Name)
	}
	if f.Version != "14.1.0" {
		t.Errorf("version: want 14.1.0, got %q", f.Version)
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
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep@=14.1.0 --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}
}

func TestCargoFetchHyphenatedCrate(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://cargo-binstall@=1.10.0")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// cargo names the binary after the package, hyphens kept, unless the
	// package declares an explicit [[bin]] name
	if f.Name != "cargo-binstall" {
		t.Errorf("name: want cargo-binstall, got %q", f.Name)
	}
	// drain so tempDirReader cleans up its temp dir
	_, _ = io.Copy(io.Discard, f.Data)
}

// update/ensure reinstall by the previously recorded name, exactly as cargo
// wrote it, so that name must resolve without a glob.
func TestCargoFetchHyphenatedCrateByPackageName(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://cargo-binstall@=1.10.0")
	f, err := p.Fetch(&FetchOpts{PackageName: "cargo-binstall"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "cargo-binstall" {
		t.Errorf("name: want cargo-binstall, got %q", f.Name)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

// A manifest that records only another package must not supply the version.
func TestInstalledVersionMatchesCrate(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"installs":{"other-crate 9.9.9 (registry+x)":{}}}`
	if err := os.WriteFile(filepath.Join(dir, ".crates2.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := installedVersion(dir, "ripgrep"); err == nil {
		t.Errorf("expected no-install error, got version %q", v)
	}
	if v, err := installedVersion(dir, "other-crate"); err != nil || v != "9.9.9" {
		t.Errorf("want 9.9.9 from other-crate, got %q, %v", v, err)
	}
}

func TestCargoFetchNoBinaries(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://nobin@=1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "produced no binaries") {
		t.Fatalf("expected no-binaries error, got %v", err)
	}
}

// A pinned install knows its version even when the manifest is unreadable.
func TestCargoFetchPinnedWithoutManifest(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://nomanifest@=1.2.3")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Version != "1.2.3" {
		t.Errorf("version: want 1.2.3, got %q", f.Version)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchUsesOptsVersion(t *testing.T) {
	calls := stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://ripgrep@^14.1.0")
	f, err := p.Fetch(&FetchOpts{Version: "14.1.1"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// ensure must reinstall the exact recorded version, never a caret
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep@=14.1.1 --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchCaretResolvesToInstalledVersion(t *testing.T) {
	calls := stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://ripgrep@^14.1.0")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// the config must record what cargo resolved, not the req itself
	if f.Version != "14.1.2" {
		t.Errorf("version: want 14.1.2 (resolved), got %q", f.Version)
	}
	_, _ = io.Copy(io.Discard, f.Data)
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep@^14.1.0 --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}
}

func TestCargoFetchLatestResolvesViaAPI(t *testing.T) {
	calls := stubCargo(t)
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Version != "14.1.2" {
		t.Errorf("version: want 14.1.2, got %q", f.Version)
	}
	// cargo resolves latest itself: no registry call, no pin. The version
	// still comes from cargo's own manifest.
	if got := calls(); len(got) != 1 || !strings.HasPrefix(got[0], "install ripgrep --root ") {
		t.Errorf("unexpected cargo calls: %v", got)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchError(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://broken@=1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "failed to select a version") {
		t.Fatalf("expected toolchain stderr in error, got %v", err)
	}
}

func TestCargoFetchMultipleBinaries(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "multiple binaries") {
		t.Fatalf("expected multi-binary error, got %v", err)
	}
}

func TestCargoFetchMultipleBinariesPicksByName(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
	f, err := p.Fetch(&FetchOpts{PackageName: "beta"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "beta" {
		t.Errorf("name: want beta, got %q", f.Name)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchMultipleBinariesPicksByPattern(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
	f, err := p.Fetch(&FetchOpts{NamePattern: "al*"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "alpha" {
		t.Errorf("name: want alpha, got %q", f.Name)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchMultipleBinariesBadPattern(t *testing.T) {
	// "al[" fails for every name, "a*[" only for names the * expands
	// into: the error must surface for both rather than be swallowed
	for _, pattern := range []string{"al[", "a*["} {
		t.Run(pattern, func(t *testing.T) {
			stubCargo(t)
			p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
			_, err := p.Fetch(&FetchOpts{NamePattern: pattern})
			if err == nil || !strings.Contains(err.Error(), "invalid name pattern") {
				t.Fatalf("expected malformed-pattern error, got %v", err)
			}
		})
	}
}

// A selector that matches nothing must not fall back to the only binary or
// to the "pick one" advice: update/ensure pass the previously installed
// name, and silently installing a different binary would overwrite the
// managed path with it.
func TestCargoFetchSelectorMatchesNothing(t *testing.T) {
	cases := []struct {
		name string
		opts *FetchOpts
		want string
	}{
		{"name", &FetchOpts{PackageName: "gamma"}, `none named "gamma"`},
		{"pattern", &FetchOpts{NamePattern: "ga*"}, `none matching pattern "ga*"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubCargo(t)
			p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
			_, err := p.Fetch(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestCargoFetchSelectorAmbiguous(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://multi@=1.0.0")
	_, err := p.Fetch(&FetchOpts{NamePattern: "*a"})
	if err == nil || !strings.Contains(err.Error(), "pick one with bin install --name") {
		t.Fatalf("expected ambiguous-pattern error, got %v", err)
	}
}

// The single-binary fallback still applies when no selector is given; only
// an explicit selector that misses becomes an error.
// A cargo that fails without writing anything to stderr must still produce a
// useful error: the exit status is all there is to report.
func TestCargoFetchFailureWithoutStderr(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://quiet-broken@=1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "exit status") {
		t.Fatalf("expected the exit status in the error, got %v", err)
	}
}

// A manifest naming another package leaves the version unknowable: guessing
// from the request would record a version that was never installed.
func TestCargoFetchManifestNamesAnotherCrate(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://othername@^1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "could not determine the installed version") {
		t.Fatalf("expected an installed-version error, got %v", err)
	}
}

// bin/ may contain a directory; only regular files are binaries.
func TestCargoFetchIgnoresNonRegularEntries(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://dironly@=1.0.0")
	_, err := p.Fetch(&FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "produced no binaries") {
		t.Fatalf("expected no-binaries error, got %v", err)
	}
}

// A bare version is an exact request, so it knows its version even when the
// manifest is unreadable - same as the explicit "=" spelling.
func TestCargoFetchBareVersionWithoutManifest(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://nomanifest@1.2.3")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Version != "1.2.3" {
		t.Errorf("version: want 1.2.3, got %q", f.Version)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoFetchSingleBinaryNoSelector(t *testing.T) {
	stubCargo(t)
	p, _ := newCargoInstall("cargoinstall://ripgrep@=1.0.0")
	f, err := p.Fetch(&FetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if f.Name != "ripgrep" {
		t.Errorf("name: want ripgrep, got %q", f.Name)
	}
	_, _ = io.Copy(io.Discard, f.Data)
}

func TestCargoGetLatestVersion(t *testing.T) {
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep")
	v, u, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "14.1.2" {
		t.Errorf("version: got %q", v)
	}
	if u != "cargoinstall://ripgrep" {
		t.Errorf("url must be the original cargo url, got %q", u)
	}
}

func TestCargoGetLatestVersionCaret(t *testing.T) {
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep@^14.1.0")
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	// ^14.1 matches any 14.x >= 14.1.0, so it must find 14.1.2 even
	// though 14.1.1 sorts first in the (shuffled) response
	if v != "14.1.2" {
		t.Errorf("version: got %q", v)
	}
}

func TestCargoGetLatestVersionStar(t *testing.T) {
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep@*")
	c := p.(*cargoInstall)
	if c.ref != "latest" {
		t.Fatalf("star must normalize to latest, got ref %q", c.ref)
	}
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "14.1.2" {
		t.Errorf("version: got %q", v)
	}
}

func TestCargoGetLatestVersionPinned(t *testing.T) {
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep@=14.1.1")
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "14.1.1" {
		t.Errorf("version: got %q", v)
	}
}

// A pin names its version, so neither the registry nor its yanked flag is
// consulted: 14.0.0 is yanked in the fixture and must still resolve.
func TestCargoGetLatestVersionPinnedYanked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("pinned version must not hit the registry, got %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })

	p, _ := newCargoInstall("cargoinstall://ripgrep@=14.0.0")
	v, _, err := p.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if v != "14.0.0" {
		t.Errorf("version: got %q", v)
	}
}

func TestCargoGetLatestVersionSkipsYankedAndPrerelease(t *testing.T) {
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://ripgrep@~14.0.0")
	if _, _, err := p.GetLatestVersion(); err == nil || !strings.Contains(err.Error(), "no non-yanked version") {
		t.Fatalf("expected no-match error, got %v", err)
	}
}

// A registry that answers with anything but 200/404 must surface as an error
// naming the status and the endpoint, not as a confusing decode failure.
func TestCargoGetLatestVersionRegistryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "registry is down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	orig := cratesIOBaseURL
	cratesIOBaseURL = srv.URL
	t.Cleanup(func() { cratesIOBaseURL = orig })

	p, _ := newCargoInstall("cargoinstall://ripgrep")
	_, _, err := p.GetLatestVersion()
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected a 503 error, got %v", err)
	}
}

// A crate with no stable release at all cannot be tracked as "latest": the
// registry answers, but max_stable_version is empty.
func TestCargoGetLatestVersionNoStableRelease(t *testing.T) {
	stubCratesIO(t, "ripgrep", &cratesIOCrate{
		Versions: []cratesIOVersion{{Num: "15.0.0-beta.1"}},
	})
	p, _ := newCargoInstall("cargoinstall://ripgrep")
	if _, _, err := p.GetLatestVersion(); err == nil || !strings.Contains(err.Error(), "no stable version") {
		t.Fatalf("expected a no-stable-version error, got %v", err)
	}
}

func TestCargoGetLatestVersionUnknownCrate(t *testing.T) {
	// the stub server 404s anything but ripgrep
	stubCratesIO(t, "ripgrep", fakeRipgrep())
	p, _ := newCargoInstall("cargoinstall://nonexistent")
	if _, _, err := p.GetLatestVersion(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// stableVersion is the single gate for "is this a candidate". Build-metadata
// and prerelease versions must never be selected by a range: cargo normalises
// 14.1.3+build.5 to 14.1.3 on install, so recording it would report an update
// that never arrives. The v prefix is tolerated because it costs nothing.
func TestStableVersion(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"14.1.2", true},
		{"v14.1.2", true},
		{"14.1.3+build.5", false},
		{"15.0.0-beta.1", false},
		{"not-a-version", false},
	}
	for _, tc := range cases {
		sv, ok := stableVersion(tc.in)
		if ok != tc.want {
			t.Errorf("stableVersion(%q): want ok=%v, got %v", tc.in, tc.want, ok)
			continue
		}
		if ok && sv.PreRelease != "" {
			t.Errorf("stableVersion(%q) returned a prerelease", tc.in)
		}
	}
}

func TestParseVersionReq(t *testing.T) {
	matchCases := []struct {
		version, ref string
		want         bool
	}{
		// caret form
		{"14.2.0", "^14.1.0", true},
		{"14.0.9", "^14.1.0", false},
		{"14.1.9", "~14.1.2", true},
		{"14.2.0", "~14.1.2", false},
		// 0.x caret: patch bumps only; ^0.0.x pins the patch
		{"0.5.4", "^0.5.3", true},
		{"0.6.0", "^0.5.3", false},
		{"0.0.3", "^0.0.3", true},
		{"0.0.4", "^0.0.3", false},
	}
	for _, tc := range matchCases {
		req, err := parseVersionReq(tc.ref)
		if err != nil {
			t.Errorf("parseVersionReq(%q): %v", tc.ref, err)
			continue
		}
		sv, ok := stableVersion(tc.version)
		if !ok {
			// prerelease and build-metadata versions are never candidates,
			// whatever the req; they only ever resolve from an exact pin
			if tc.want {
				t.Errorf("version %q: expected a stable candidate", tc.version)
			}
			continue
		}
		if got := req.matches(sv); got != tc.want {
			t.Errorf("req %q matches %q: want %v, got %v", tc.ref, tc.version, tc.want, got)
		}
	}
	// star is normalized to latest before it ever reaches the req world
	for _, bad := range []string{"14", "14.1", ">=14.1", "14.x", "", "14.1.0.1", "*"} {
		if _, err := parseVersionReq(bad); err == nil {
			t.Errorf("expected error for req %q", bad)
		}
	}
	// a range needs a full triplet: the bound is what carries the operator,
	// so ^14 and ~14.1 must fail the same way a bare one does
	for _, bad := range []string{"^14", "~14.1", "^14.1.0.1"} {
		if _, err := parseVersionReq(bad); err == nil || !strings.Contains(err.Error(), "want x.y.z") {
			t.Errorf("expected triplet error for req %q, got %v", bad, err)
		}
	}
	// a range only matches stable releases, so a prerelease bound can never
	// resolve; it must be rejected up front with a pinning hint instead of
	// failing on every update
	for _, bad := range []string{"^14.1.0-beta.1", "~14.1.0-rc.1"} {
		if _, err := parseVersionReq(bad); err == nil || !strings.Contains(err.Error(), "pin it exactly") {
			t.Errorf("expected prerelease-bound error for req %q, got %v", bad, err)
		}
	}
	// a component at the top of int64 would wrap negative when the range
	// bound is computed, so it must be an error rather than a panic
	for _, bad := range []string{
		"^9223372036854775807.0.0",
		"^0.9223372036854775807.0",
		"^0.0.9223372036854775807",
		"~1.9223372036854775807.0",
		// a bare version is a pin, so it only needs to parse, not to bound
		"=",
	} {
		if _, err := parseVersionReq(bad); err == nil {
			t.Errorf("expected out-of-range error for req %q", bad)
		}
	}
	// the largest valid bounds still parse
	for _, ok := range []string{"^9223372036854775806.0.0", "~1.9223372036854775806.0"} {
		if _, err := parseVersionReq(ok); err != nil {
			t.Errorf("unexpected error for req %q: %v", ok, err)
		}
	}
	// pins carry their version verbatim, prerelease included, and never go
	// through matches (which only handles ranges). A bare triplet pins too:
	// that is what `cargo install` does with it
	for _, pin := range []string{"=14.1.0", "=14.1.0-beta.1", "14.1.0", "14.1.0-beta.1"} {
		req, err := parseVersionReq(pin)
		if err != nil {
			t.Errorf("parseVersionReq(%q): %v", pin, err)
			continue
		}
		if want := strings.TrimPrefix(pin, "="); req.pin != want {
			t.Errorf("pin %q: want %q, got %q", pin, want, req.pin)
		}
	}
}
