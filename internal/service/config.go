package service

import (
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
}

func createDefaultConfig() error {
	cfg := Config{
		Preferences: Preferences{
			Locale: "En",
		},
		Credentials: Credentials{
			DBPath:      "",
			KeyFilePath: "",
		},
	}

	return SaveConfig(&cfg)
}

func getAbsoluteConfigPath() string {
	abs, err := util.PathFromHome(configPathFromHome + configFileName)
	if err != nil {
		panic("Failed to get absolute config path: " + err.Error())
	}

	return abs
}

func LoadConfig() (*Config, error) {
	path := getAbsoluteConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		cfg := Config{
			Preferences: Preferences{Locale: "En"},
			Credentials: Credentials{},
		}

		if err := SaveConfig(&cfg); err != nil {
			return nil, err
		}

		return &cfg, nil
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveConfig(config *Config) error {
	path := getAbsoluteConfigPath()
	file, err := util.OpenOrCreateFileAndTruncate(path)
	if err != nil {
		return err
	}
	defer file.Close()

	toml, err := toml.Marshal(config)
	if err != nil {
		return err
	}

	file.Write(toml)

	return nil
}
