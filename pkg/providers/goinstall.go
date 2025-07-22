package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apex/log"
)

type goinstall struct {
	name, repo, tag, latestURL string
}

func parseRepo(path string) (string, string, string, string) {
	repo := path
	tag := "latest"
	if i := strings.LastIndex(path, "@"); i > -1 {
		repo = filepath.Clean(path[:i])
		tag = path[i+1:]
	}

	name := path
	if i := strings.LastIndex(repo, "/"); i > -1 {
		name = repo[i+1:]
	}

	latestURL := fmt.Sprintf("https://proxy.golang.org/%s/@latest", repo)

	return repo, tag, name, latestURL
}

func newGoInstall(repo string) (Provider, error) {
	repoUrl := strings.TrimPrefix(repo, "goinstall://")
	repo, tag, name, latestURL := parseRepo(repoUrl)
	return &goinstall{repo: repo, tag: tag, name: name, latestURL: latestURL}, nil
}

func getGoPath() (string, error) {
	cmd := exec.Command("go", "env", "path")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %v failed: %w, output: %s", cmd, err, string(output))
	}
	return string(output), nil
}

func (g *goinstall) Fetch(opts *FetchOpts) (*File, error) {
	goPath, err := getGoPath()
	if err != nil {
		return nil, err
	}

	if (len(g.tag) > 0 && g.tag != "latest") || len(opts.Version) > 0 {
		if len(opts.Version) > 0 {
			// this is used by for the `ensure` command
			g.tag = opts.Version
		}
		log.Infof("Getting %s release for %s", g.tag, g.repo)
	} else {
		log.Infof("Getting latest release for %s", g.repo)
		if name, _, err := g.GetLatestVersion(); err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		} else {
			g.tag = name
		}
	}

	cmd := exec.Command("go", "install", fmt.Sprintf("%s@%s", g.repo, g.tag))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to install package: %w", err)
	}

	goBinPath := filepath.Join(goPath, "bin", g.name)

	file, err := os.Open(os.ExpandEnv(goBinPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open path: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return nil, err
	}

	// Clean go file added in gopath file
	if err := os.Remove(os.ExpandEnv(goBinPath)); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error removing path %s: %v", os.ExpandEnv(goBinPath), err)
	}

	return &File{
		Data:    &buf,
		Name:    g.name,
		Version: g.tag,
	}, nil
}

func (d *goinstall) GetLatestVersion() (string, string, error) {
	resp, err := http.Get(d.latestURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}

	version, ok := result["Version"].(string)
	if !ok {
		return "", "", fmt.Errorf("version not found in response")
	}

	return version, d.repo, nil
}

func (d *goinstall) GetID() string {
	return "goinstall"
}
