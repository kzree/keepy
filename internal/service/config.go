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

type Config struct {
	Credentials Credentials `toml:"credentials"`
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
		return nil, err
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
