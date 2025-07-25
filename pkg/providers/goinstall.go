package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/caarlos0/log"
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
	cmd := exec.Command("go", "env", "GOPATH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %v failed: %w, output: %s", cmd, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
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
		return nil, fmt.Errorf("failed to open path '%s': %w", goBinPath, err)
	}
	// don't close and keep it for Data, bin is short lived CLI tool
	// defer file.Close()

	return &File{
		Data:    file,
		Name:    g.name,
		Version: g.tag,
	}, nil
}

func (g *goinstall) GetLatestVersion() (string, string, error) {
	resp, err := http.Get(g.latestURL)
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

	return version, g.repo, nil
}

func (g *goinstall) GetID() string {
	return "goinstall"
}
