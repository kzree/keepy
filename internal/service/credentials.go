package service

import (
	"encoding/json"

	"kzree.com/keepy/internal/util"
)

const (
	configPathFromHome = ".config/keepy/"
	configFileName     = "config.json"
)

type Credentials struct {
	DBPath      string `json:"dbPath"`
	KeyFilePath string `json:"keyFilePath"`
}

func getAbsoluteConfigPath() string {
	abs, err := util.PathFromHome(configPathFromHome + configFileName)
	if err != nil {
		panic("Failed to get absolute config path: " + err.Error())
	}

	return abs
}

func LoadSavedCredentials() (*Credentials, error) {
	path := getAbsoluteConfigPath()
	file, err := util.OpenOrCreateFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var creds Credentials
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&creds); err != nil {
		if err.Error() == "EOF" {
			return nil, nil
		}
		return nil, err
	}

	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	path := getAbsoluteConfigPath()
	file, err := util.OpenOrCreateFile(path)
	if err != nil {
		return err
	}
	defer file.Close()

	json, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		panic(err)
	}

	file.Write(json)

	return nil
}
