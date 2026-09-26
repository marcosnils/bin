//go:build linux

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
)

// Installed binaries must be runnable by everyone (e.g. root installing into
// /usr/local/bin for all users, #304) and must never be
// writable by group/others, otherwise anyone on the box can swap a binary
// that sits in the user's PATH.
func TestSaveToDiskPermissions(t *testing.T) {
	oldMask := syscall.Umask(0o022)
	defer syscall.Umask(oldMask)

	cfgHome := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	if err := os.MkdirAll(filepath.Join(cfgHome, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"default_path": "` + binDir + `", "bins": {}}`
	if err := os.WriteFile(filepath.Join(cfgHome, "bin", "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.CheckAndLoad(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(binDir, "mybin")
	f := &providers.File{Data: strings.NewReader("#!/bin/sh\necho ok\n"), Name: "mybin", Version: "1.0.0"}
	if _, err := saveToDisk(f, dst, false); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	perm := fi.Mode().Perm()
	if perm&0o111 != 0o111 {
		t.Errorf("binary not executable by owner, group and others: %v", perm)
	}
	if perm&0o022 != 0 {
		t.Errorf("binary writable by group/others: %v", perm)
	}
}
