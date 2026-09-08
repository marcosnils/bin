package config

import (
	"testing"
	"time"
)

func TestParseCooldown(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{" 0 ", 0, false},
		{"24h", 24 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"1h30m", 90 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"abc", 0, true},
		{"-1d", 0, true},
		{"-5h", 0, true},
		{"5x", 0, true},
	}

	for _, c := range cases {
		got, err := ParseCooldown(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseCooldown(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCooldown(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseCooldown(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFormatCooldown(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "disabled"},
		{-1, "disabled"},
		{24 * time.Hour, "1d"},
		{7 * 24 * time.Hour, "1w"},
		{14 * 24 * time.Hour, "2w"},
		{48 * time.Hour, "2d"},
		{90 * time.Minute, "1h30m0s"},
	}
	for _, c := range cases {
		if got := FormatCooldown(c.in); got != c.want {
			t.Errorf("FormatCooldown(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEffectiveCooldown(t *testing.T) {
	// Save and restore the package singleton.
	orig := cfg
	t.Cleanup(func() { cfg = orig })

	// No default, no override -> built-in default (0/disabled).
	cfg = config{}
	if d, err := EffectiveCooldown(nil); err != nil || d != builtinDefaultCooldown {
		t.Fatalf("nil binary, no default: got (%v, %v), want (%v, nil)", d, err, builtinDefaultCooldown)
	}

	// Global default applies when no per-binary override is set.
	cfg = config{DefaultCooldown: "24h"}
	if d, err := EffectiveCooldown(&Binary{}); err != nil || d != 24*time.Hour {
		t.Fatalf("default only: got (%v, %v), want (24h, nil)", d, err)
	}

	// Per-binary override beats the global default.
	if d, err := EffectiveCooldown(&Binary{Cooldown: "7d"}); err != nil || d != 7*24*time.Hour {
		t.Fatalf("override: got (%v, %v), want (168h, nil)", d, err)
	}

	// An explicit "0" override disables cooldown even when a default is set.
	if d, err := EffectiveCooldown(&Binary{Cooldown: "0"}); err != nil || d != 0 {
		t.Fatalf("override disable: got (%v, %v), want (0, nil)", d, err)
	}
}
