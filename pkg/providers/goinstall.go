package providers

import (
	"bytes"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/caarlos0/log"
)

// goCommand builds the *exec.Cmd used to run the go toolchain. It is a
// variable so tests can swap in a helper process and exercise the provider
// without network access or a real toolchain.
var goCommand = func(args ...string) *exec.Cmd {
	return exec.Command("go", args...)
}

type goinstall struct {
	// url is the original goinstall:// URL, persisted back to the config
	// untouched so the ref the user asked for survives updates
	url string
	// importPath is the package path handed to `go install`. It may point
	// inside a module (github.com/foo/bar/cmd/baz); `go install` resolves
	// the containing module itself
	importPath string
	// ref is the version selector: latest (default), a tag, a branch or a
	// commit. It acts as the update channel for GetLatestVersion
	ref string
	// binaryPath is the managed binary on disk, set by cmd/update through
	// BinaryPathSetter so the module path can be read from its build info
	binaryPath string
}

// parseGoInstallURL splits goinstall://<importPath>[@<ref>] into its parts.
func parseGoInstallURL(u string) (importPath, ref string) {
	path := strings.TrimPrefix(u, "goinstall://")
	ref = "latest"
	if i := strings.LastIndex(path, "@"); i > -1 {
		ref = path[i+1:]
		path = path[:i]
	}
	if ref == "" {
		ref = "latest"
	}
	return strings.Trim(path, "/"), ref
}

func newGoInstall(u string) (Provider, error) {
	importPath, ref := parseGoInstallURL(u)
	if importPath == "" {
		return nil, fmt.Errorf("goinstall: missing package path in %q", u)
	}
	return &goinstall{url: u, importPath: importPath, ref: ref}, nil
}

// SetBinaryPath implements BinaryPathSetter.
func (g *goinstall) SetBinaryPath(path string) {
	g.binaryPath = path
}

// runGo runs the go toolchain and returns its stdout, folding stderr into
// the returned error so callers get the toolchain's own explanation.
func runGo(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := goCommand(args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("go %s: %s", strings.Join(args, " "), detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// tempDirReader streams a file from a temporary directory and removes the
// whole directory once the file has been fully read. Providers hand back an
// io.Reader with no lifecycle hook, so cleanup rides on EOF.
type tempDirReader struct {
	f   *os.File
	dir string
}

func (r *tempDirReader) Read(p []byte) (int, error) {
	n, err := r.f.Read(p)
	if err == io.EOF {
		r.f.Close()
		_ = os.RemoveAll(r.dir)
	}
	return n, err
}

func (g *goinstall) Fetch(opts *FetchOpts) (*File, error) {
	ref := g.ref
	if opts.Version != "" {
		// used by `ensure` to reinstall the exact recorded version
		ref = opts.Version
	}

	tmpDir, err := os.MkdirTemp("", "bin-goinstall-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	// Install into a private GOBIN: go resolves the containing module and
	// the binary name for us, and the user's own GOBIN is left untouched.
	log.Infof("Running go install %s@%s", g.importPath, ref)
	cmd := goCommand("install", fmt.Sprintf("%s@%s", g.importPath, ref))
	cmd.Env = append(cmd.Environ(), "GOBIN="+tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("failed to install %s@%s:\n%s", g.importPath, ref, detail)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		cleanup()
		return nil, err
	}
	if len(entries) != 1 {
		cleanup()
		return nil, fmt.Errorf("expected go install to produce one binary, found %d", len(entries))
	}
	binPath := filepath.Join(tmpDir, entries[0].Name())

	info, err := buildinfo.ReadFile(binPath)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to read build info from %s: %w", binPath, err)
	}
	log.Debugf("Installed %s from module %s@%s", info.Path, info.Main.Path, info.Main.Version)

	f, err := os.Open(binPath)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open %s: %w", binPath, err)
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		cleanup()
		return nil, err
	}

	return &File{
		Data:    &tempDirReader{f: f, dir: tmpDir},
		Name:    entries[0].Name(),
		Version: info.Main.Version,
		Length:  fi.Size(),
	}, nil
}

// modulePath returns the module containing importPath. It prefers the build
// info recorded in the managed binary and falls back to asking the toolchain,
// walking up the import path until a module answers.
func (g *goinstall) modulePath() (string, error) {
	if g.binaryPath != "" {
		info, err := buildinfo.ReadFile(g.binaryPath)
		switch {
		case err != nil:
			log.Debugf("Could not read build info from %s: %v", g.binaryPath, err)
		case info.Main.Path == "":
			log.Debugf("%s has no module in its build info, resolving via go list", g.binaryPath)
		default:
			return info.Main.Path, nil
		}
	}

	var errs []error
	parts := strings.Split(g.importPath, "/")
	for len(parts) > 0 {
		candidate := strings.Join(parts, "/")
		out, err := runGo("list", "-m", "-f", "{{.Path}}", candidate+"@"+g.ref)
		if err == nil {
			return out, nil
		}
		errs = append(errs, err)
		parts = parts[:len(parts)-1]
	}
	return "", fmt.Errorf("could not find a module for %s: %w", g.importPath, errors.Join(errs...))
}

// GetLatestVersion resolves the version the URL's ref currently points at
// (@latest by default) through the go toolchain, honouring GOPROXY and
// friends. The returned URL is the original goinstall:// URL.
func (g *goinstall) GetLatestVersion() (string, string, error) {
	mod, err := g.modulePath()
	if err != nil {
		return "", "", err
	}
	log.Debugf("Resolving %s@%s", mod, g.ref)
	version, err := runGo("list", "-m", "-f", "{{.Version}}", mod+"@"+g.ref)
	if err != nil {
		return "", "", err
	}
	if version == "" {
		return "", "", fmt.Errorf("go list returned no version for %s@%s", mod, g.ref)
	}
	return version, g.url, nil
}

func (g *goinstall) GetID() string {
	return "goinstall"
}
