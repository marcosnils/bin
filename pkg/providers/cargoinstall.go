package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/coreos/go-semver/semver"
	"github.com/marcosnils/bin/pkg/httpclient"
)

// cargoCommand builds the *exec.Cmd used to run the cargo toolchain. It is a
// variable so tests can swap in a helper process and exercise the provider
// without network access or a real toolchain.
var cargoCommand = func(args ...string) *exec.Cmd {
	return exec.Command("cargo", args...)
}

// cratesIOBaseURL is overridden in tests to point at a stub server.
var cratesIOBaseURL = "https://crates.io/api/v1"

// crateNameRE is the alphabet crates.io allows in a crate name, minus a
// leading hyphen: anything else is a typo or an attempt to smuggle a cargo
// flag (a leading `-`) or URL syntax (`?`, `#`, `/`) into the
// `cargo install` argv and the registry request.
var crateNameRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)

type cargoInstall struct {
	// url is the original cargoinstall:// URL, persisted back to the config
	// untouched so the ref the user asked for survives updates
	url string
	// crateName is the crate handed to `cargo install`. Unlike a Go
	// import path it is also the crate's identity in the registry
	crateName string
	// ref is the version selector: latest (default) or a semver req as
	// understood by `cargo install` (=1.2.3, ^1.2.3, ~1.2.3, 1.2.3).
	// A bare triplet without an operator is an exact version, not a caret
	// req, matching cargo itself. It acts as the update channel for
	// GetLatestVersion
	ref string
}

// parseCargoInstallURL splits cargoinstall://<crate>[@<version>] into its parts.
func parseCargoInstallURL(u string) (crateName, ref string) {
	path := strings.TrimPrefix(u, "cargoinstall://")
	if i := strings.LastIndex(path, "@"); i > -1 {
		ref = path[i+1:]
		path = path[:i]
	}

	if ref == "" {
		ref = "latest"
	}

	return strings.Trim(path, "/"), ref
}

func newCargoInstall(u string) (Provider, error) {
	crateName, ref := parseCargoInstallURL(u)
	if crateName == "" {
		return nil, fmt.Errorf("cargo: missing crate name in %q", u)
	}

	if !crateNameRE.MatchString(crateName) {
		return nil, fmt.Errorf("cargo: invalid crate name %q in %q (crates.io names are letters, digits, - and _)", crateName, u)
	}

	// `*` means any version, i.e. the same as latest
	if ref == "*" {
		ref = "latest"
	} else if ref != "latest" {
		// reject refs cargo itself would refuse, before any compilation;
		// parseVersionReq explains what is wrong with the ref in detail
		if _, err := parseVersionReq(ref); err != nil {
			return nil, fmt.Errorf("cargo: %w in %q", err, u)
		}
	}

	return &cargoInstall{url: u, crateName: crateName, ref: ref}, nil
}

// versionReq is the parsed form of a cargo version requirement: either an
// exact pin or a half-open range [lo, hi). Prerelease and build-metadata
// versions only ever match a pin.
type versionReq struct {
	pin string
	lo  *semver.Version
	hi  *semver.Version
}

// hasRangeOperator reports whether ref is a caret or tilde range rather than
// an exact version.
func hasRangeOperator(ref string) bool {
	return ref != "" && (ref[0] == '^' || ref[0] == '~')
}

// exactVersion strips the optional "=" from a ref naming one exact version
// and validates it. cargo accepts a bare 1.2.3 here, not only "=1.2.3".
func exactVersion(ref string) (string, error) {
	pin := strings.TrimPrefix(ref, "=")
	if _, err := semver.NewVersion(pin); err != nil {
		return "", fmt.Errorf("invalid pinned version %q", ref)
	}

	return pin, nil
}

// parseVersionReq understands the requirement forms `cargo install`
// accepts: =1.2.3 and a bare 1.2.3 (both exact pins), ^1.2.3 (caret) and
// ~1.2.3 (tilde). cargo treats a bare triplet as an exact version, not as a
// caret req the way dependencies are, so a range needs its operator spelled
// out. For 0.x caret keeps cargo's compatibility rules: ^0.2.3 allows patch
// bumps only, ^0.0.3 nothing beyond the exact patch.
func parseVersionReq(ref string) (*versionReq, error) {
	if !hasRangeOperator(ref) {
		pin, err := exactVersion(ref)
		if err != nil {
			return nil, err
		}

		return &versionReq{pin: pin}, nil
	}
	op, rest := ref[:1], ref[1:]

	// a range needs a full x.y.z bound. This is deliberately a subset of what
	// cargo accepts: cargo also takes ^14, ~14.1 and 1.*, but resolving those
	// ourselves would mean reimplementing cargo's partial-version rules
	lo, err := semver.NewVersion(rest)
	if err != nil {
		return nil, fmt.Errorf("invalid version req %q (want x.y.z)", ref)
	}

	// ranges only ever match stable releases, so a prerelease bound would
	// accept a ref that can never resolve; pins handle prereleases exactly
	if lo.PreRelease != "" || lo.Metadata != "" {
		return nil, fmt.Errorf("version req %q has a prerelease bound; pin it exactly instead (=%s)", ref, rest)
	}

	// the range ends at the next incompatible version
	var hiStr string
	switch {
	case op == "~":
		// ~1.2.3 allows patch bumps only
		hiStr = fmt.Sprintf("%d.%d.0", lo.Major, lo.Minor+1)
	case lo.Major > 0:
		// ^1.2.3 allows anything below the next major
		hiStr = fmt.Sprintf("%d.0.0", lo.Major+1)
	case lo.Minor > 0:
		// ^0.2.3 allows patch bumps only
		hiStr = fmt.Sprintf("0.%d.0", lo.Minor+1)
	default:
		// ^0.0.3 allows nothing beyond the exact patch
		hiStr = fmt.Sprintf("0.0.%d", lo.Patch+1)
	}

	// the components are int64, so a req at the top of the range wraps into
	// a negative number; semver.New would panic on that rather than report it
	hi, err := semver.NewVersion(hiStr)
	if err != nil || hi.Compare(*lo) <= 0 {
		return nil, fmt.Errorf("version req %q is out of range", ref)
	}

	return &versionReq{lo: lo, hi: hi}, nil
}

// stableVersion parses a registry version string into something a range can
// compare against. Crate versions never carry a v prefix, but tolerate one.
// Prerelease and build-metadata versions are never candidates: cargo only
// resolves them from an explicit pin.
func stableVersion(s string) (*semver.Version, bool) {
	sv, err := semver.NewVersion(strings.TrimPrefix(s, "v"))
	if err != nil || sv.PreRelease != "" || sv.Metadata != "" {
		return nil, false
	}

	return sv, true
}

// matches reports whether a registry version satisfies the req. Only ranges
// reach this: a pin is resolved without consulting the registry.
func (r *versionReq) matches(v *semver.Version) bool {
	return v.Compare(*r.lo) >= 0 && v.Compare(*r.hi) < 0
}

// cratesIOCrate is the subset of the crates.io API response of
// GET /api/v1/crates/{name} the provider needs.
type cratesIOCrate struct {
	Crate struct {
		MaxStableVersion string `json:"max_stable_version"`
		// Name is the canonical spelling: the API answers for aliases such
		// as serde-json, which cargo itself refuses
		Name string `json:"name"`
	} `json:"crate"`
	Versions []cratesIOVersion `json:"versions"`
}

type cratesIOVersion struct {
	Num    string `json:"num"`
	Yanked bool   `json:"yanked"`
}

// fetchCrate looks the crate up in the crates.io registry.
func (c *cargoInstall) fetchCrate() (*cratesIOCrate, error) {
	// PathEscape keeps the name in a single path segment: the validated
	// alphabet rules out the exotic cases, but this is what makes the request
	// URL safe by construction rather than by a regexp elsewhere
	endpoint := fmt.Sprintf("%s/crates/%s", cratesIOBaseURL, url.PathEscape(c.crateName))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// crates.io requires a User-Agent identifying the client
	req.Header.Set("User-Agent", "marcosnils/bin")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("crate %q not found in the registry", c.crateName)
	}

	if resp.StatusCode != http.StatusOK {
		// drain so the connection can be reused; this path is hit on
		// rate-limit and outage responses, exactly when retries follow
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("crates.io: unexpected status %s for %s", resp.Status, endpoint)
	}

	var info cratesIOCrate
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

// maxMatching returns the highest non-yanked registry version satisfying
// req, or "" if none does. The order of the versions array is not relied
// upon.
func maxMatching(info *cratesIOCrate, req *versionReq) string {
	var best *semver.Version
	for _, v := range info.Versions {
		if v.Yanked {
			continue
		}

		sv, ok := stableVersion(v.Num)
		if !ok || !req.matches(sv) {
			continue
		}

		if best == nil || sv.Compare(*best) > 0 {
			best = sv
		}
	}

	if best == nil {
		return ""
	}

	return best.String()
}

// installedVersion reads the version cargo actually installed from the
// .crates2.json manifest it writes under --root. Each install key is
// "<package> <version> (<source>)"; the entry is matched by package name so a
// crate whose build installs something else is not reported under the wrong
// version.
func installedVersion(root, crateName string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, ".crates2.json"))
	if err != nil {
		return "", err
	}

	var meta struct {
		Installs map[string]json.RawMessage `json:"installs"`
	}

	if err := json.Unmarshal(b, &meta); err != nil {
		return "", err
	}

	for k := range meta.Installs {
		fields := strings.Fields(k)
		if len(fields) >= 2 && strings.EqualFold(fields[0], crateName) {
			return fields[1], nil
		}
	}

	return "", fmt.Errorf("no install of %s recorded under %s", crateName, root)
}

// pickBinName selects which of the binaries the crate built: an exact
// PackageName (the previously installed binary, used by update/ensure)
// wins, otherwise a NamePattern glob (`bin install --name`). Empty means no
// selector was given, so the caller decides; a selector that matches nothing
// or several binaries is an error rather than a silent fallback, which keeps
// update/ensure from installing a different binary than the one recorded.
// opts must not be nil, matching every other provider.
func pickBinName(names []string, opts *FetchOpts) (string, error) {
	if opts.PackageName != "" {
		for _, n := range names {
			if n == opts.PackageName {
				return n, nil
			}
		}

		return "", fmt.Errorf("built %s, none named %q", strings.Join(names, ", "), opts.PackageName)
	}

	if opts.NamePattern == "" {
		return "", nil
	}

	// a malformed glob is only reported for some names, so Match is checked
	// per name instead of once against ""; the pattern is fixed, so the
	// first error is the one worth reporting
	var matched []string
	for _, n := range names {
		ok, err := filepath.Match(opts.NamePattern, n)
		if err != nil {
			return "", fmt.Errorf("invalid name pattern %q: %w", opts.NamePattern, err)
		}

		if ok {
			matched = append(matched, n)
		}
	}

	switch len(matched) {
	case 0:
		return "", fmt.Errorf("built %s, none matching pattern %q", strings.Join(names, ", "), opts.NamePattern)
	case 1:
		return matched[0], nil
	default:
		return "", fmt.Errorf("pattern %q matches %s; pick one with bin install --name <bin>",
			opts.NamePattern, strings.Join(matched, ", "))
	}
}

// nameResolutionError reports whether cargo failed to resolve the crate name,
// as opposed to failing to fetch or build it. Only this failure is a name
// mismatch worth explaining, and only it justifies an extra request: probing
// crates.io after a compile error or with the registry unreachable adds
// nothing the user can act on and, offline, costs the full fetch timeout.
func nameResolutionError(detail string) bool {
	return strings.Contains(detail, "could not find `") &&
		strings.Contains(detail, "in registry `")
}

// aliasHint explains a cargo failure that is really a name mismatch. The
// crates.io API answers for aliases such as serde-json (canonical serde_json)
// and for any letter case, but cargo install is strict, so a user can reach a
// raw "could not find" with a crate that looked fine everywhere else.
func (c *cargoInstall) aliasHint(detail string) string {
	if !nameResolutionError(detail) {
		return ""
	}

	info, err := c.fetchCrate()
	if err != nil || info.Crate.Name == "" || info.Crate.Name == c.crateName {
		return ""
	}

	return fmt.Sprintf("\ncrates.io knows this crate as %q; cargo names are case- and hyphen-sensitive", info.Crate.Name)
}

// Fetch runs `cargo install <crate>@<version> --root <tmpdir>` and streams
// back the binary it produced. The user's own CARGO_HOME is not touched;
// the temp dir is removed once the file has been fully read.
func (c *cargoInstall) Fetch(opts *FetchOpts) (result *File, err error) {
	// spec is what cargo gets: a pinned version when one is known (ensure
	// reinstalling the recorded version, or latest resolved up front), the
	// ref itself when it is a req and cargo resolves it. The version that
	// actually ends up installed is read back from cargo's manifest.
	// pinned is set whenever spec names one exact version, which is also the
	// only case where the request alone tells us the installed version.
	spec := c.ref
	var pinned string
	switch {
	case opts.Version != "":
		// comes from the config through ensure, so it is validated like a URL ref
		v, err := exactVersion(opts.Version)
		if err != nil {
			return nil, err
		}
		pinned, spec = v, "="+v
	case c.ref == "latest":
		// cargo resolves latest itself, from the index or cache it already
		// has. Pinning here would need the crates.io HTTP API, which a
		// mirrored or air-gapped setup has no reason to expose; the version
		// that actually landed is read back from cargo's manifest anyway.
		spec = ""
	case !hasRangeOperator(c.ref):
		// already validated when the URL was parsed; the explicit "=" is
		// just spelling, so record the version without it
		pinned, _ = exactVersion(c.ref)
	}

	tmpDir, err := os.MkdirTemp("", "bin-cargo-")
	if err != nil {
		return nil, err
	}

	// on success the temp dir is handed to the caller inside
	// tempDirReader, which removes it on EOF; this defer covers every
	// error return until then
	defer func() {
		if result == nil {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	// cargo fetches sources into the shared CARGO_HOME cache but places
	// the binary under <root>/bin, leaving the user's own cargo binaries
	// untouched
	arg := c.crateName
	if spec != "" {
		arg += "@" + spec
	}
	log.Infof("Running cargo install %s", arg)

	cmd := cargoCommand("install", arg, "--root", tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return nil, fmt.Errorf("failed to install %s:\n%s%s", arg, detail, c.aliasHint(detail))
	}

	binDir := filepath.Join(tmpDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil, err
	}

	// only regular files are binaries: a stray directory in bin/ must not be
	// chosen and then fail as unreadable
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type().IsRegular() {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("cargo install produced no binaries in %s", binDir)
	}

	name, err := pickBinName(names, opts)
	if err != nil {
		return nil, fmt.Errorf("crate %s: %w", c.crateName, err)
	}

	if name == "" {
		if len(names) > 1 {
			return nil, fmt.Errorf("crate %s builds multiple binaries (%s); pick one with bin install --name <bin>",
				c.crateName, strings.Join(names, ", "))
		}

		name = names[0]
	}

	version, err := installedVersion(tmpDir, c.crateName)
	if err != nil {
		// the manifest is the source of truth, but an exact request already
		// knows its version
		if pinned == "" {
			return nil, fmt.Errorf("could not determine the installed version of %s: %w", c.crateName, err)
		}

		version = pinned
		log.Debugf("cargo manifest unreadable, assuming pinned %s: %v", version, err)
	}

	f, err := os.Open(filepath.Join(binDir, name))
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &File{
		Data:    &tempDirReader{f: f, dir: tmpDir},
		Name:    name,
		Version: version,
		Length:  fi.Size(),
	}, nil
}

// GetLatestVersion resolves the version the URL's ref currently points at:
// the newest non-yanked version matching a req, the newest stable version for
// latest, and the named version for a pin. Only the first two need the
// crates.io API. The returned URL is the original cargoinstall:// URL.
func (c *cargoInstall) GetLatestVersion() (string, string, error) {
	version, err := c.resolveRef()
	if err != nil {
		return "", "", err
	}

	log.Debugf("Resolved %s@%s to %s", c.crateName, c.ref, version)
	return version, c.url, nil
}

// resolveRef maps the URL's ref to the concrete version it points at today:
// latest is the newest stable release, a range the highest non-yanked match,
// and a pin is the version it names - the registry is only consulted for the
// forms that need it.
func (c *cargoInstall) resolveRef() (string, error) {
	if c.ref == "latest" {
		info, err := c.fetchCrate()
		if err != nil {
			return "", err
		}

		if v := info.Crate.MaxStableVersion; v != "" {
			return v, nil
		}

		return "", fmt.Errorf("crates.io returned no stable version for %q", c.crateName)
	}

	req, err := parseVersionReq(c.ref)
	if err != nil {
		return "", err
	}

	// a pin names its version outright, so neither the registry nor the
	// yanked check applies: the user asked for exactly this version, and a
	// yank after the fact must not break `bin update`
	if req.pin != "" {
		return req.pin, nil
	}

	info, err := c.fetchCrate()
	if err != nil {
		return "", err
	}

	version := maxMatching(info, req)
	if version == "" {
		return "", fmt.Errorf("no non-yanked version of %s matches %q", c.crateName, c.ref)
	}

	return version, nil
}

func (c *cargoInstall) GetID() string {
	return "cargoinstall"
}
