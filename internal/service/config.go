package service

import (
	"errors"
	"io"
	"os"

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

type Preferences struct {
	Locale string `toml:"locale"`
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

func SaveConfig(config *Config) error {
	data, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	path, err := getAbsoluteConfigPath()
	if err != nil {
		return err
	}
	file, err := util.OpenOrCreateFileAndTruncate(path)
	if err != nil {
		return err
	}
	return writeConfig(file, data)
}

// writeConfig owns and closes the writer, preserving write and close causes.
// Config saving is not a vault transaction and does not promise file rollback.
func writeConfig(file io.WriteCloser, data []byte) error {
	n, err := file.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return errors.Join(err, file.Close())
}
