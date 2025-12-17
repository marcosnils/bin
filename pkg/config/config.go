package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/caarlos0/log"
)

var cfg config

type config struct {
	// DefaultPath might not be expanded so it's important that
	// the caller expands this variable with os.ExpandEnv(string)
	// if necessary
	DefaultPath string             `json:"default_path"`
	Bins        map[string]*Binary `json:"bins"`
}

type Binary struct {
	Path       string `json:"path"`
	RemoteName string `json:"remote_name"`
	Version    string `json:"version"`
	Hash       string `json:"hash"`
	URL        string `json:"url"`
	Provider   string `json:"provider"`
	// if file is installed from a package format (zip, tar, etc) store
	// the package path in config so we don't ask the user to select
	// the path again when upgrading
	PackagePath string `json:"package_path"`
	Pinned      bool   `json:"pinned"`
	// StateURL holds a release- or version-specific URL, persisted only in state
	StateURL string `json:"-"`
}

// stateEntry contains per-machine mutable data
// persisted separately from the manifest
type stateEntry struct {
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	PackagePath string `json:"package_path"`
	Pinned      bool   `json:"pinned"`
	URL         string `json:"url"`
}

type state struct {
	Bins map[string]*stateEntry `json:"bins"`
}

func CheckAndLoad() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	confDir := filepath.Dir(configPath)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("Error creating config directory [%v]", err)
	}
	log.Debugf("Config directory is: %s", confDir)

	// Load manifest (may not exist yet)
	mf, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer mf.Close()
	if err := json.NewDecoder(mf).Decode(&cfg); err != nil {
		if err == io.EOF {
			cfg.Bins = map[string]*Binary{}
		} else {
			return err
		}
	}
	if cfg.Bins == nil {
		cfg.Bins = map[string]*Binary{}
	}

	// Load state and overlay
	sp, err := getStatePath(configPath)
	if err != nil {
		return err
	}
	st := state{Bins: map[string]*stateEntry{}}
	if sf, err := os.Open(sp); err == nil {
		defer sf.Close()
		_ = json.NewDecoder(sf).Decode(&st)
	}
	for k, sb := range st.Bins {
		if b, ok := cfg.Bins[k]; ok && sb != nil {
			b.Version = sb.Version
			b.Hash = sb.Hash
			b.PackagePath = sb.PackagePath
			b.Pinned = sb.Pinned
		}
	}

	// If DefaultPath not set, prompt user and write both files
	if len(cfg.DefaultPath) == 0 {
		cfg.DefaultPath, err = getDefaultPath()
		if err != nil {
			for {
				log.Info("Could not find a PATH directory automatically, falling back to manual selection")
				reader := bufio.NewReader(os.Stdin)
				var response string
				fmt.Printf("\nPlease specify a download directory: ")
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("Invalid input")
				}
				response = strings.TrimSpace(response)

				if err = checkDirExistsAndWritable(response); err != nil {
					log.Debugf("Could not set download directory [%s]: [%v]", response, err)
					continue
				}

				cfg.DefaultPath = response
				break
			}
		}

		if err := writeAll(); err != nil {
			return err
		}
	}

	// Migration: if manifest contains state but state file is empty, split
	needsMigration := false
	if len(st.Bins) == 0 {
		for _, b := range cfg.Bins {
			if b == nil {
				continue
			}
			if b.Version != "" || b.Hash != "" || b.PackagePath != "" || b.Pinned {
				needsMigration = true
				break
			}
		}
	}
	if needsMigration {
		log.Infof("Splitting config manifest and state into %s and %s", configPath, sp)
		if err := writeAll(); err != nil {
			return err
		}
	}

	// Normalize URLs in manifest to base repository links when possible
	if normalizeManifestURLs() {
		if err := writeAll(); err != nil {
			return err
		}
	}

	log.Debugf("Download path set to %s", cfg.DefaultPath)
	return nil
}

// normalizeManifestURLs rewrites manifest URLs to stable base links
// (e.g. https://github.com/owner/repo) when they currently point
// at release/tag or download URLs. Returns true if it modified cfg.
func normalizeManifestURLs() bool {
	changed := false
	for _, b := range cfg.Bins {
		if b == nil || b.URL == "" {
			continue
		}
		base := normalizeBaseURL(b.URL, b.Provider)
		if base != "" && base != b.URL {
			// Preserve the original, potentially version-specific URL in state
			if b.StateURL == "" {
				b.StateURL = b.URL
			}
			log.Debugf("Normalizing manifest URL from %s to %s", b.URL, base)
			b.URL = base
			changed = true
		}
	}
	return changed
}

// normalizeBaseURL attempts to derive a stable repository/home URL from
// a potentially versioned or release-specific URL.
func normalizeBaseURL(raw, provider string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	// If provider isn't set in the manifest (older entries), infer it from host
	inferredProvider := provider
	if inferredProvider == "" {
		host := u.Host
		if strings.Contains(host, "github") {
			inferredProvider = "github"
		} else if strings.Contains(host, "codeberg") {
			inferredProvider = "codeberg"
		} else if strings.Contains(host, "gitlab") {
			inferredProvider = "gitlab"
		}
	}

	switch inferredProvider {
	case "github", "codeberg", "gitlab":
		parts := strings.Split(u.Path, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("%s://%s/%s/%s", u.Scheme, u.Host, parts[1], parts[2])
		}
	}
	return ""
}

func Get() *config {
	return &cfg
}

// UpsertBinary adds or updats an existing
// binary resource in the config
func UpsertBinary(c *Binary) error {
	if c != nil {
		// Preserve existing state URL unless the caller overrides it
		if existing, ok := cfg.Bins[c.Path]; ok && c.StateURL == "" {
			c.StateURL = existing.StateURL
		}
		cfg.Bins[c.Path] = c
		if err := writeAll(); err != nil {
			return err
		}
	}
	return nil
}

// RemoveBinaries removes the specified paths
// from bin configuration. It doesn't care about the order
func RemoveBinaries(paths []string) error {
	for _, p := range paths {
		delete(cfg.Bins, p)
	}
	return writeAll()
}

// writeAll writes manifest and state to their respective locations
func writeAll() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	statePath, err := getStatePath(configPath)
	if err != nil {
		return err
	}
	if err := writeManifest(configPath); err != nil {
		return err
	}
	if err := writeState(statePath); err != nil {
		return err
	}
	return nil
}

type manifestConfig struct {
	DefaultPath string                        `json:"default_path"`
	Bins        map[string]*manifestBinary    `json:"bins"`
}

type manifestBinary struct {
	Path       string `json:"path"`
	RemoteName string `json:"remote_name"`
	URL        string `json:"url"`
	Provider   string `json:"provider"`
}

func writeManifest(manifestPath string) error {
	dir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(manifestPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	// sanitize state fields out of manifest
	out := manifestConfig{DefaultPath: cfg.DefaultPath, Bins: map[string]*manifestBinary{}}
	for k, b := range cfg.Bins {
		if b == nil {
			continue
		}
		out.Bins[k] = &manifestBinary{
			Path:       b.Path,
			RemoteName: b.RemoteName,
			URL:        b.URL,
			Provider:   b.Provider,
		}
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(out)
}

func writeState(statePath string) error {
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(statePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	st := state{Bins: map[string]*stateEntry{}}
	for k, b := range cfg.Bins {
		if b == nil {
			continue
		}
		st.Bins[k] = &stateEntry{Version: b.Version, Hash: b.Hash, PackagePath: b.PackagePath, Pinned: b.Pinned, URL: b.StateURL}
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(st)
}

// GetArch is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.
func GetArch() []string {
	res := []string{runtime.GOARCH}
	if runtime.GOARCH == "amd64" {
		res = append(res, "x86_64")
		res = append(res, "x64")
	}
	return res
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() []string {
	res := []string{runtime.GOOS}
	if runtime.GOOS == "windows" {
		res = append(res, "win")
	}
	return res
}

// getConfigPath returns the path to the configuration directory respecting
// the `XDG Base Directory specification` using the following strategy:
//   - honor BIN_CONFIG is set (even if not existing yet)
//   - to prevent breaking of existing configurations, check if "$HOME/.bin/config.json"
//     exists and return "$HOME/.bin"
//   - if "XDG_CONFIG_HOME" is set, return "$XDG_CONFIG_HOME/bin"
//   - if "$HOME/.config" exists, return "$home/.config/bin"
//   - default to "$HOME/.bin/"
func getConfigPath() (string, error) {
	c := os.Getenv("BIN_CONFIG")
	if len(c) > 0 {
		return c, nil
	}

	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		if _, err := os.Stat(filepath.Join(home, ".bin", "config.json")); !os.IsNotExist(err) {
			return filepath.Join(path.Join(home, ".bin", "config.json")), nil
		}
	}

	c = os.Getenv("XDG_CONFIG_HOME")
	if c != "" {
		return filepath.Join(c, "bin", "config.json"), nil
	}
	if homeErr != nil {
		return "", homeErr
	}
	c = filepath.Join(home, ".config")
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		return filepath.Join(c, "bin", "config.json"), nil
	}
	return filepath.Join(home, ".bin", "config.json"), nil
}

// getStatePath computes the per-machine state file path derived from manifest path
func getStatePath(manifestPath string) (string, error) {
	base := filepath.Base(manifestPath)
	name := strings.TrimSuffix(base, filepath.Ext(base)) + ".state.json"
	// Prefer XDG_DATA_HOME
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "bin", name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "bin", name), nil
	case "windows":
		if ld := os.Getenv("LOCALAPPDATA"); ld != "" {
			return filepath.Join(ld, "bin", name), nil
		}
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "bin", name), nil
		}
		return filepath.Join(home, ".local", "share", "bin", name), nil
	default:
		return filepath.Join(home, ".local", "share", "bin", name), nil
	}
}

func GetOSSpecificExtensions() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{"AppImage"}
	case "windows":
		return []string{"exe"}
	default:
		return nil
	}
}
