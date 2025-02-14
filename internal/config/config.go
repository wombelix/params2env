// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Region    string        `yaml:"region,omitempty"`
	Replica   string        `yaml:"replica,omitempty"`
	Prefix    string        `yaml:"prefix,omitempty"`
	Output    string        `yaml:"output,omitempty"`
	File      string        `yaml:"file,omitempty"`
	Upper     *bool         `yaml:"upper,omitempty"`
	EnvPrefix string        `yaml:"env_prefix,omitempty"`
	Role      string        `yaml:"role,omitempty"`
	KMS       string        `yaml:"kms,omitempty"`
	Params    []ParamConfig `yaml:"params,omitempty"`
}

// ParamConfig represents individual parameter configurations
type ParamConfig struct {
	Name   string `yaml:"name"`
	Env    string `yaml:"env,omitempty"`
	Region string `yaml:"region,omitempty"`
	Output string `yaml:"output,omitempty"`
}

// LoadConfig loads configuration from files with precedence:
// 1. Current directory (.params2env.yaml)
// 2. Home directory (~/.params2env.yaml)
func LoadConfig() (*Config, error) {
	var cfg Config

	// Try loading from home directory first
	home, err := os.UserHomeDir()
	if err == nil {
		homeConfig := filepath.Join(home, ".params2env.yaml")
		if fileExists(homeConfig) {
			if err := loadFile(homeConfig, &cfg); err != nil {
				// Log error but continue with empty config
				fmt.Fprintf(os.Stderr, "Warning: Failed to load global config: %v\n", err)
			}
		}
	}

	// Try loading from current directory (overrides home config)
	cwdConfig := ".params2env.yaml"
	if fileExists(cwdConfig) {
		localCfg := Config{}
		if err := loadFile(cwdConfig, &localCfg); err != nil {
			// Log error but continue with existing config
			fmt.Fprintf(os.Stderr, "Warning: Failed to load local config: %v\n", err)
		} else {
			mergeConfig(&cfg, &localCfg)
		}
	}

	return &cfg, nil
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

// loadFile loads and unmarshals a YAML configuration file
func loadFile(filename string, cfg *Config) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// mergeConfig merges local configuration into global configuration
func mergeConfig(global, local *Config) {
	if local.Region != "" {
		global.Region = local.Region
	}
	if local.Replica != "" {
		global.Replica = local.Replica
	}
	if local.Prefix != "" {
		global.Prefix = local.Prefix
	}
	if local.Output != "" {
		global.Output = local.Output
	}
	if local.File != "" {
		global.File = local.File
	}
	if local.Upper != nil {
		global.Upper = local.Upper
	}
	if local.EnvPrefix != "" {
		global.EnvPrefix = local.EnvPrefix
	}
	if local.Role != "" {
		global.Role = local.Role
	}
	if local.KMS != "" {
		global.KMS = local.KMS
	}
	if len(local.Params) > 0 {
		global.Params = local.Params
	}
}
