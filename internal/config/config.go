package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

const (
	defaultConfigFilePath = "config.yaml"
)

// FileProvider is a config provider for local files
type FileProvider struct {
	Filename string
}

// Config variables for the application
type Config struct {
	PostgresDSN      string            `yaml:"POSTGRES_DSN"`
	PrivilegedTokens map[string]string `yaml:"privileged_tokens"`
}

// Load loads the configuration for the application.
func Load() (Config, error) {
	var config Config

	file, err := os.Open(defaultConfigFilePath)
	if err != nil {
		return Config{}, errors.Wrap(err, "can't open file config file")
	}
	defer file.Close()

	d := yaml.NewDecoder(file)

	if err := d.Decode(&config); err != nil {
		return Config{}, errors.Wrap(err, "failed to decode config")
	}

	return config, nil
}
