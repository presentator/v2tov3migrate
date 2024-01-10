package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
)

// NewConfigFromJson reads the specified json file and returns it as new Config.
func NewConfigFromJson(jsonPath string) (*Config, error) {
	c := &Config{}

	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read json file: %w", err)
	}

	if err := json.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("unable to parse json file: %w", err)
	}

	return c, nil
}

// Config defines the migrator configurations.
type Config struct {
	// The "pb_data" directory of the Presentator v3 installation.
	//
	// It is recommended to setup your Presentator v3 upfront from the Admin UI.
	V3DataDir string `json:"v3DataDir"`

	// The v2 DB database driver to use - "mysql" or "pgx".
	V2DBDriver string `json:"v2DBDriver"`

	// The v2 DB connection string.
	V2DBConnection string `json:"v2DBConnection"`

	// V2 storage configuration.
	//
	// Either V2LocalStorage or V2S3Storage must be set!
	V2LocalStorage string `json:"v2LocalStorage,omitempty"`
	V2S3Storage    struct {
		Bucket         string `json:"bucket,omitempty"`
		Region         string `json:"region,omitempty"`
		Endpoint       string `json:"endpoint,omitempty"`
		AccessKey      string `json:"accessKey,omitempty"`
		Secret         string `json:"secret,omitempty"`
		ForcePathStyle bool   `json:"forcePathStyle,omitempty"`
	} `json:"v2S3Storage"`
}

// Validate performs very basic validity checks for the current Config fields.
func (c *Config) Validate() error {
	if c.V3DataDir == "" {
		return errors.New("v3DataDir is not set")
	}

	if _, err := os.Stat(path.Join(c.V3DataDir, "data.db")); err != nil {
		return err
	}

	if c.V2DBDriver == "" {
		return errors.New("v2DBDriver name is not set")
	}

	if c.V2DBConnection == "" {
		return errors.New("v2DBConnection is not set")
	}

	if c.V2LocalStorage == "" && c.V2S3Storage.Bucket == "" {
		return errors.New("either v2LocalStorage or v2S3Storage must be set")
	}

	if c.V2LocalStorage != "" && c.V2S3Storage.Bucket != "" {
		return errors.New("only one of v2LocalStorage or v2S3Storage must be set")
	}

	return nil
}
