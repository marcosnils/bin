package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// builtinDefaultCooldown is the cooldown used when neither a per-binary override
// nor a global default is configured. It is 0 (disabled) on purpose: cooldowns
// are opt-in so that upgrading bin never silently starts blocking updates.
// Users enable protection with `bin cooldown set 24h`.
const builtinDefaultCooldown time.Duration = 0

// ParseCooldown parses a human-friendly cooldown string into a time.Duration.
//
// Accepted forms:
//   - ""  or "0"           -> 0 (disabled)
//   - "<n>d" / "<n>w"      -> n days / n weeks
//   - anything else        -> time.ParseDuration (e.g. "24h", "90m", "1h30m")
//
// time.ParseDuration does not understand day ("d") or week ("w") units, which
// is why they are handled explicitly here. Negative durations are rejected.
func ParseCooldown(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	var d time.Duration
	var err error
	switch {
	case strings.HasSuffix(s, "d"):
		d, err = parseUnit(s, "d", 24*time.Hour)
	case strings.HasSuffix(s, "w"):
		d, err = parseUnit(s, "w", 7*24*time.Hour)
	default:
		d, err = time.ParseDuration(s)
	}
	if err != nil {
		return 0, fmt.Errorf("invalid cooldown %q: %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid cooldown %q: must not be negative", s)
	}
	return d, nil
}

// parseUnit parses "<n><suffix>" as n*unit.
func parseUnit(s, suffix string, unit time.Duration) (time.Duration, error) {
	n, err := strconv.Atoi(strings.TrimSuffix(s, suffix))
	if err != nil {
		return 0, err
	}
	return time.Duration(n) * unit, nil
}

// FormatCooldown renders a duration back to the shortest sensible unit for
// display. Whole weeks/days are shown as "Nw"/"Nd"; 0 is "disabled"; everything
// else falls back to time.Duration.String() (e.g. "1h30m0s").
func FormatCooldown(d time.Duration) string {
	switch {
	case d <= 0:
		return "disabled"
	case d%(7*24*time.Hour) == 0:
		return fmt.Sprintf("%dw", d/(7*24*time.Hour))
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	default:
		return d.String()
	}
}

// EffectiveCooldown resolves the cooldown that applies to a binary using the
// order: per-binary override -> global default -> built-in default.
func EffectiveCooldown(b *Binary) (time.Duration, error) {
	if b != nil && b.Cooldown != "" {
		return ParseCooldown(b.Cooldown)
	}
	if cfg.DefaultCooldown != "" {
		return ParseCooldown(cfg.DefaultCooldown)
	}
	return builtinDefaultCooldown, nil
}

// SetDefaultCooldown validates and persists the global default cooldown.
// An empty string clears it (reverting to the built-in default).
func SetDefaultCooldown(s string) error {
	s = strings.TrimSpace(s)
	if s != "" {
		if _, err := ParseCooldown(s); err != nil {
			return err
		}
	}
	cfg.DefaultCooldown = s
	return write()
}
