package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"kzree.com/keepy/internal/util"
)

const (
	configPathFromHome = ".config/keepy/"
	configFileName     = "config.toml"
)

type Credentials struct {
	DBPath      string `toml:"db_path"`
	KeyFilePath string `toml:"key_file_path"`
}

// DefaultClipboardTimeout applies when clipboard_timeout is unset or invalid.
const DefaultClipboardTimeout = 2 * time.Minute

type Preferences struct {
	Locale string `toml:"locale"`
	// ClipboardTimeout is a Go duration such as "2m", or "off"/"0" to keep
	// copied passwords until overwritten. Empty uses DefaultClipboardTimeout.
	ClipboardTimeout string `toml:"clipboard_timeout,omitempty"`
}

// ClipboardClearDelay returns how long a copied password stays in the
// clipboard; zero disables clearing. An invalid value returns the default, so a
// typo never leaves passwords in the clipboard indefinitely.
func (p Preferences) ClipboardClearDelay() (time.Duration, error) {
	value := strings.TrimSpace(p.ClipboardTimeout)
	switch strings.ToLower(value) {
	case "":
		return DefaultClipboardTimeout, nil
	case "off":
		return 0, nil
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return DefaultClipboardTimeout, fmt.Errorf("invalid clipboard_timeout %q: use a duration such as \"2m\", or \"off\"", value)
	}
	return delay, nil
}

type Config struct {
	Preferences Preferences `toml:"preferences"`
	Credentials Credentials `toml:"credentials"`
	// Keybindings overrides default keys by action name. Saving preserves the
	// configured values even when the UI rejected them and used defaults.
	Keybindings map[string][]string `toml:"keybindings,omitempty"`
}

func createDefaultConfig() (*Config, error) {
	cfg := Config{
		Preferences: Preferences{
			Locale: "en",
		},
		Credentials: Credentials{
			DBPath:      "",
			KeyFilePath: "",
		},
	}

	return &cfg, SaveConfig(&cfg)
}

func getAbsoluteConfigPath() (string, error) {
	return util.PathFromHome(configPathFromHome + configFileName)
}

func LoadConfig() (*Config, error) {
	path, err := getAbsoluteConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		cfg, err := createDefaultConfig()
		if err != nil {
			return nil, err
		}

		return cfg, nil
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ConfigDurabilityError reports that the new config replaced the old one, but a
// later durability check failed. Saving again is safe; the config is not lost.
type ConfigDurabilityError struct{ Err error }

func (e *ConfigDurabilityError) Error() string {
	return "config saved, but durability could not be confirmed: " + e.Err.Error()
}

func (e *ConfigDurabilityError) Unwrap() error { return e.Err }

// SaveConfig serializes before touching the file, then replaces it atomically
// through a same-directory temporary file. Any error before replacement leaves
// the previous config intact; a *ConfigDurabilityError means it was replaced.
func SaveConfig(config *Config) error {
	data, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	path, err := getAbsoluteConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return replaceConfig(newConfigPersistence(), path, data)
}

func newConfigPersistence() *vaultPersistence {
	p := newVaultPersistence()
	p.subject = "config"
	return p
}

// replaceConfig writes through a symlinked config to its target, so links
// managed by dotfile tools survive, and keeps an existing file's permissions.
func replaceConfig(p *vaultPersistence, path string, data []byte) error {
	mode := os.FileMode(0o644)
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	prepared, err := p.prepare(path, mode, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	committed, err := prepared.commit(path)
	if committed && err != nil {
		return &ConfigDurabilityError{Err: err}
	}
	return err
}
