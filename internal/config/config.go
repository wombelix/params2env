// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

// Package config loads and merges YAML configs from ~/.params2env.yaml and ./.params2env.yaml.
// Local config takes precedence over global.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrInvalidConfig = errors.New("invalid configuration")

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

type ParamConfig struct {
	Name   string `yaml:"name"`
	Env    string `yaml:"env,omitempty"`
	Region string `yaml:"region,omitempty"`
	Output string `yaml:"output,omitempty"`
}

func (c *Config) Validate() error {
	for i, param := range c.Params {
		if param.Name == "" {
			return fmt.Errorf("%w: parameter at index %d missing name", ErrInvalidConfig, i)
		}
	}

	if c.Output != "" && c.Output != "env" && c.Output != "file" {
		return fmt.Errorf("%w: invalid output format %q (must be 'env' or 'file')", ErrInvalidConfig, c.Output)
	}

	return nil
}

// Loads config from ~/.params2env.yaml then ./.params2env.yaml (local wins).
func LoadConfig() (*Config, error) {
	var cfg Config

	home, err := os.UserHomeDir()
	if err == nil {
		homeConfig := filepath.Join(home, ".params2env.yaml")
		if fileExists(homeConfig) {
			if err := loadFile(homeConfig, &cfg); err != nil {
				return nil, fmt.Errorf("failed to load global config %s: %w", homeConfig, err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("invalid global config %s: %w", homeConfig, err)
			}
		}
	}

	cwdConfig := ".params2env.yaml"
	if fileExists(cwdConfig) {
		localCfg := Config{}
		if err := loadFile(cwdConfig, &localCfg); err != nil {
			return nil, fmt.Errorf("failed to load local config %s: %w", cwdConfig, err)
		}
		if err := localCfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid local config %s: %w", cwdConfig, err)
		}
		mergeConfig(&cfg, &localCfg)
	}

	return &cfg, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

func loadFile(filename string, cfg *Config) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", sanitizeForLog(filename), err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML in %s: %w", sanitizeForLog(filename), err)
	}
	return nil
}

// Local wins. Params slice is replaced, not merged.
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
	if local.EnvPrefix != "" {
		global.EnvPrefix = local.EnvPrefix
	}
	if local.Role != "" {
		global.Role = local.Role
	}
	if local.KMS != "" {
		global.KMS = local.KMS
	}
	if local.Upper != nil {
		global.Upper = local.Upper
	}
	if len(local.Params) > 0 {
		global.Params = local.Params
	}
}

// Strips control chars to prevent log injection.
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.ReplaceAll(s, "\x1b", "") // Remove escape sequences
}
