package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/caarlos0/log"
	"github.com/hashicorp/go-version"
	"github.com/marcosnils/bin/pkg/config"
)

type awsCLI struct {
	version string // parsed from URL, empty means "latest"
	tagsURL string // GitHub tags API URL (overridable for testing)
}

func newAWSCLI(u string) (Provider, error) {
	raw := strings.TrimPrefix(u, "awscli://")
	v := strings.TrimSpace(raw)
	if v == "" || v == "latest" {
		v = ""
	}
	return &awsCLI{
		version: v,
		tagsURL: "https://api.github.com/repos/aws/aws-cli/tags",
	}, nil
}

func (a *awsCLI) GetID() string {
	return "awscli"
}

func (a *awsCLI) GetLatestVersion() (string, string, error) {
	req, err := http.NewRequest("GET", a.tagsURL+"?per_page=100", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use GitHub token if available to avoid rate limits
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else if token := os.Getenv("GITHUB_AUTH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch AWS CLI tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return "", "", fmt.Errorf("GitHub API rate limit exceeded; set GITHUB_TOKEN to increase limits")
		}
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", "", fmt.Errorf("failed to parse tags response: %w", err)
	}

	var versions []*version.Version
	for _, t := range tags {
		if !strings.HasPrefix(t.Name, "2.") {
			continue
		}
		v, err := version.NewVersion(t.Name)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return "", "", fmt.Errorf("no AWS CLI v2 versions found")
	}

	sort.Sort(version.Collection(versions))
	latest := versions[len(versions)-1].Original()

	return latest, fmt.Sprintf("awscli://%s", latest), nil
}

func (a *awsCLI) Fetch(opts *FetchOpts) (*File, error) {
	// Resolve version
	ver := a.version
	if len(opts.Version) > 0 {
		ver = opts.Version
	}
	if ver == "" {
		log.Infof("Getting latest release for AWS CLI")
		latest, _, err := a.GetLatestVersion()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
		ver = latest
	} else {
		log.Infof("Getting AWS CLI release %s", ver)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" && goos != "darwin" {
		return nil, fmt.Errorf("AWS CLI v2 provider does not support %s", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return nil, fmt.Errorf("AWS CLI v2 provider does not support architecture %s", goarch)
	}

	// Compute install directory from config default path
	defaultPath := expandPath(config.Get().DefaultPath)
	installDir := filepath.Join(filepath.Dir(defaultPath), "lib", "awscli")

	// Build download URL
	downloadURL, err := buildDownloadURL(ver, goos, goarch)
	if err != nil {
		return nil, err
	}

	// Download to temp file
	log.Infof("Downloading AWS CLI v2 from %s", downloadURL)
	tmpFile, err := downloadToTemp(downloadURL)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	// Platform-specific install — returns the directory containing the aws/aws_completer binaries
	binBase, err := installAWSCLI(tmpFile, installDir, goos, ver)
	if err != nil {
		return nil, fmt.Errorf("failed to install AWS CLI: %w", err)
	}

	// Determine real binary paths
	awsBin := filepath.Join(binBase, "aws")
	awsCompleter := filepath.Join(binBase, "aws_completer")

	// Verify the install worked
	if _, err := os.Stat(awsBin); err != nil {
		return nil, fmt.Errorf("AWS CLI binary not found at %s after installation: %w", awsBin, err)
	}

	// Create aws_completer wrapper as a side-effect
	if _, err := os.Stat(awsCompleter); err == nil {
		completerWrapper := fmt.Sprintf("#!/bin/sh\n# awscli version: %s\nexec \"%s\" \"$@\"\n", ver, awsCompleter)
		completerPath := filepath.Join(defaultPath, "aws_completer")
		if err := os.WriteFile(completerPath, []byte(completerWrapper), 0o766); err != nil {
			log.Warnf("Failed to create aws_completer wrapper at %s: %v", completerPath, err)
		} else {
			log.Infof("Created aws_completer wrapper at %s", completerPath)
		}
	}

	// Build the aws wrapper script
	wrapperScript := fmt.Sprintf("#!/bin/sh\n# awscli version: %s\nexec \"%s\" \"$@\"\n", ver, awsBin)

	return &File{
		Data:    strings.NewReader(wrapperScript),
		Name:    "aws",
		Version: ver,
	}, nil
}

// Cleanup removes the AWS CLI support directory and aws_completer wrapper.
func (a *awsCLI) Cleanup() error {
	defaultPath := expandPath(config.Get().DefaultPath)
	installDir := filepath.Join(filepath.Dir(defaultPath), "lib", "awscli")
	completerPath := filepath.Join(defaultPath, "aws_completer")

	if err := os.RemoveAll(installDir); err != nil {
		log.Warnf("Failed to remove AWS CLI support directory %s: %v", installDir, err)
	} else {
		log.Infof("Removed AWS CLI support directory %s", installDir)
	}

	if err := os.Remove(completerPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to remove aws_completer at %s: %v", completerPath, err)
	} else if err == nil {
		log.Infof("Removed aws_completer at %s", completerPath)
	}

	return nil
}

// buildDownloadURL constructs the AWS CLI download URL for the given version, OS, and architecture.
func buildDownloadURL(ver, goos, goarch string) (string, error) {
	const base = "https://awscli.amazonaws.com"

	switch goos {
	case "linux":
		arch := "x86_64"
		if goarch == "arm64" {
			arch = "aarch64"
		}
		if ver != "" {
			return fmt.Sprintf("%s/awscli-exe-linux-%s-%s.zip", base, arch, ver), nil
		}
		return fmt.Sprintf("%s/awscli-exe-linux-%s.zip", base, arch), nil

	case "darwin":
		if ver != "" {
			return fmt.Sprintf("%s/AWSCLIV2-%s.pkg", base, ver), nil
		}
		return fmt.Sprintf("%s/AWSCLIV2.pkg", base), nil

	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
}

// downloadToTemp downloads a URL to a temporary file and returns the path.
func downloadToTemp(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download %s: HTTP %d", url, resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "awscli-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write download: %w", err)
	}

	return tmpFile.Name(), nil
}

// expandPath expands both environment variables ($HOME, etc.) and the ~ prefix
// in a path. os.ExpandEnv only handles $VAR, not ~.
func expandPath(path string) string {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}

// getAWSCLIVersion runs the installed aws binary and extracts the version string.
// This is used as a fallback to verify the installed version.
func getAWSCLIVersion(awsBin string) (string, error) {
	cmd := exec.Command(awsBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Output format: "aws-cli/2.15.0 Python/3.11.6 Linux/6.1.0 ..."
	parts := strings.Fields(string(out))
	if len(parts) > 0 {
		return strings.TrimPrefix(parts[0], "aws-cli/"), nil
	}
	return "", fmt.Errorf("unexpected aws --version output: %s", string(out))
}
