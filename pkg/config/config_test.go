package config

import (
	"runtime"
	"testing"
)

func TestGetArchIncludesAliases(t *testing.T) {
	archs := GetArch()
	contains := func(v string) bool {
		for _, arch := range archs {
			if arch == v {
				return true
			}
		}
		return false
	}

	if !contains(runtime.GOARCH) {
		t.Fatalf("expected GetArch to include runtime arch %s, got %v", runtime.GOARCH, archs)
	}

	if runtime.GOARCH == "amd64" {
		if !contains("x86_64") {
			t.Fatalf("expected amd64 aliases to include x86_64, got %v", archs)
		}
		if !contains("x64") {
			t.Fatalf("expected amd64 aliases to include x64, got %v", archs)
		}
	}

	if runtime.GOARCH == "arm64" && !contains("aarch64") {
		t.Fatalf("expected arm64 aliases to include aarch64, got %v", archs)
	}
}
