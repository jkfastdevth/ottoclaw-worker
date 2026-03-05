package configstore

import (
	"errors"
	"os"
	"path/filepath"

	ottoclawconfig "github.com/sipeed/ottoclaw/pkg/config"
)

const (
	configDirName  = ".ottoclaw"
	configFileName = "config.json"
)

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName), nil
}

func Load() (*ottoclawconfig.Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	return ottoclawconfig.LoadConfig(path)
}

func Save(cfg *ottoclawconfig.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	return ottoclawconfig.SaveConfig(path, cfg)
}
