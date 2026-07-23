package cmd

import (
	"reflect"
	"testing"
	"time"

	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
)

type mockProvider struct {
	providers.Provider
	latestVersion    string
	latestVersionURL string
	publishedAt      time.Time
	err              error
}

func (m mockProvider) GetLatestVersion() (string, string, time.Time, error) {
	return m.latestVersion, m.latestVersionURL, m.publishedAt, m.err
}

func (m mockProvider) GetID() string {
	return "github"
}

func TestGetLatestVersion(t *testing.T) {
	// Pin the clock so cooldown windows are deterministic.
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = time.Now })

	type mockValues struct {
		latestVersion    string
		latestVersionURL string
		publishedAt      time.Time
		err              error
	}
	cases := []struct {
		name string
		in   *config.Binary
		m    mockValues
		out  *updateInfo
	}{
		{
			name: "newer version available",
			in: &config.Binary{
				Path:       "/home/user/bin/launchpad",
				Version:    "1.1.0",
				URL:        "https://github.com/Mirantis/launchpad/releases/download/1.1.0/launchpad-linux-x64",
				RemoteName: "launchpad-linux-x64",
				Provider:   "github",
			},
			m: mockValues{"1.1.1", "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64", time.Time{}, nil},
			out: &updateInfo{
				version: "1.1.1",
				url:     "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64",
			},
		},
		{
			name: "latest is older than current",
			in: &config.Binary{
				Path:       "/home/user/bin/launchpad",
				Version:    "1.2.0-rc.1",
				URL:        "https://github.com/Mirantis/launchpad/releases/download/1.2.0-rc.1/launchpad-linux-x64",
				RemoteName: "launchpad-linux-x64",
				Provider:   "github",
			},
			m:   mockValues{"1.1.1", "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64", time.Time{}, nil},
			out: nil,
		},
		{
			name: "held back by cooldown",
			in: &config.Binary{
				Path:       "/home/user/bin/launchpad",
				Version:    "1.1.0",
				URL:        "https://github.com/Mirantis/launchpad/releases/download/1.1.0/launchpad-linux-x64",
				RemoteName: "launchpad-linux-x64",
				Provider:   "github",
				Cooldown:   "24h",
			},
			// published 1h ago -> within the 24h cooldown
			m:   mockValues{"1.1.1", "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64", now.Add(-1 * time.Hour), nil},
			out: nil,
		},
		{
			name: "cooldown elapsed",
			in: &config.Binary{
				Path:       "/home/user/bin/launchpad",
				Version:    "1.1.0",
				URL:        "https://github.com/Mirantis/launchpad/releases/download/1.1.0/launchpad-linux-x64",
				RemoteName: "launchpad-linux-x64",
				Provider:   "github",
				Cooldown:   "24h",
			},
			// published 48h ago -> past the 24h cooldown
			m: mockValues{"1.1.1", "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64", now.Add(-48 * time.Hour), nil},
			out: &updateInfo{
				version: "1.1.1",
				url:     "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64",
			},
		},
		{
			name: "cooldown set but publish date unknown",
			in: &config.Binary{
				Path:       "/home/user/bin/launchpad",
				Version:    "1.1.0",
				URL:        "https://github.com/Mirantis/launchpad/releases/download/1.1.0/launchpad-linux-x64",
				RemoteName: "launchpad-linux-x64",
				Provider:   "github",
				Cooldown:   "24h",
			},
			// zero publish time -> cooldown is not enforced
			m: mockValues{"1.1.1", "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64", time.Time{}, nil},
			out: &updateInfo{
				version: "1.1.1",
				url:     "https://github.com/Mirantis/launchpad/releases/download/1.1.1/launchpad-linux-x64",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mockProvider{latestVersion: c.m.latestVersion, latestVersionURL: c.m.latestVersionURL, publishedAt: c.m.publishedAt, err: c.m.err}
			if v, err := getLatestVersion(c.in, p); err != nil {
				t.Fatalf("Error during getLatestVersion(%#v, %#v): %v", c.in, p, err)
			} else if !reflect.DeepEqual(v, c.out) {
				t.Fatalf("For case %q: %#v does not match %#v", c.name, v, c.out)
			}
		})
	}
}
