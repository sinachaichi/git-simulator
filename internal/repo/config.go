package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

// ErrNoIdentity is returned when no author name or email is configured.
var ErrNoIdentity = errors.New("author identity unknown")

func (r *Repo) configPath() string { return filepath.Join(r.GitDir, "config") }

// ConfigGet returns a value such as "user.name". The second result is
// false when the key is not set.
func (r *Repo) ConfigGet(key string) (string, bool, error) {
	lines, err := r.readConfig()
	if err != nil {
		return "", false, err
	}
	section, name, err := splitKey(key)
	if err != nil {
		return "", false, err
	}
	current := ""
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			current = strings.ToLower(strings.TrimSpace(t[1 : len(t)-1]))
			continue
		}
		k, v, ok := strings.Cut(t, "=")
		if ok && current == section && strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.TrimSpace(v), true, nil
		}
	}
	return "", false, nil
}

// ConfigSet writes a value such as "user.name" to .hood/config.
func (r *Repo) ConfigSet(key, value string) error {
	section, name, err := splitKey(key)
	if err != nil {
		return err
	}
	lines, err := r.readConfig()
	if err != nil {
		return err
	}
	current, header, inserted := "", -1, false
	var out []string
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			if current == section && !inserted {
				out = append(out, "\t"+name+" = "+value)
				inserted = true
			}
			current = strings.ToLower(strings.TrimSpace(t[1 : len(t)-1]))
			if current == section {
				header = i
			}
		} else if k, _, ok := strings.Cut(t, "="); ok && current == section && strings.EqualFold(strings.TrimSpace(k), name) {
			if !inserted {
				out = append(out, "\t"+name+" = "+value)
				inserted = true
			}
			continue
		}
		out = append(out, line)
	}
	switch {
	case inserted:
	case header >= 0:
		out = append(out, "\t"+name+" = "+value)
	default:
		out = append(out, "["+section+"]", "\t"+name+" = "+value)
	}
	return os.WriteFile(r.configPath(), []byte(strings.Join(out, "\n")+"\n"), 0o644)
}

func (r *Repo) readConfig() ([]string, error) {
	data, err := os.ReadFile(r.configPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"), nil
}

func splitKey(key string) (section, name string, err error) {
	section, name, ok := strings.Cut(strings.ToLower(key), ".")
	if !ok || section == "" || name == "" || strings.Contains(name, ".") {
		return "", "", fmt.Errorf("invalid config key %q, expected section.name", key)
	}
	return section, name, nil
}

// Identity returns the author or committer ("author" or "committer"),
// using the same environment variables as git: GIT_AUTHOR_NAME,
// GIT_AUTHOR_EMAIL and GIT_AUTHOR_DATE. Without them it falls back to
// user.name and user.email from the config and the current time.
func (r *Repo) Identity(role string) (objects.Signature, error) {
	prefix := "GIT_" + strings.ToUpper(role) + "_"
	name, email := os.Getenv(prefix+"NAME"), os.Getenv(prefix+"EMAIL")
	var err error
	if name == "" {
		if name, _, err = r.ConfigGet("user.name"); err != nil {
			return objects.Signature{}, err
		}
	}
	if email == "" {
		if email, _, err = r.ConfigGet("user.email"); err != nil {
			return objects.Signature{}, err
		}
	}
	if name == "" || email == "" {
		return objects.Signature{}, ErrNoIdentity
	}
	when := time.Now()
	if text := os.Getenv(prefix + "DATE"); text != "" {
		if when, err = ParseDate(text); err != nil {
			return objects.Signature{}, err
		}
	}
	return objects.Signature{Name: name, Email: email, When: when}, nil
}

// ParseDate reads "1700000000 +0200" (git's raw format) or ISO 8601.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if unix, zone, ok := strings.Cut(s, " "); ok {
		if sec, err := strconv.ParseInt(unix, 10, 64); err == nil {
			if t, err := time.Parse("-0700", zone); err == nil {
				_, off := t.Zone()
				return time.Unix(sec, 0).In(time.FixedZone("", off)), nil
			}
		}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02 15:04:05 -0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q (use ISO 8601 or \"<unix> +0200\")", s)
}
