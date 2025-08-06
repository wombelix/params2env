// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package config provides configuration management for the params2env tool.
//
// It handles loading and merging of YAML configuration files from multiple
// locations with a defined precedence order. The package supports both global
// (user home directory) and local (current directory) configurations, with
// local settings taking precedence over global ones.
//
// Configuration files are expected to be named .params2env.yaml and can define
// default settings for AWS regions, parameter handling, and individual parameter
// configurations.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Common errors returned by the package
var (
	ErrInvalidConfig = errors.New("invalid configuration")
)

// Config represents the main configuration structure for params2env.
// It defines global settings that apply to all parameter operations
// unless overridden by specific parameter configurations.
type Config struct {
	// Region is the default AWS region for operations
	Region string `yaml:"region,omitempty"`
	// Replica is the region where parameters should be replicated
	Replica string `yaml:"replica,omitempty"`
	// Prefix is the common prefix for all parameter paths
	Prefix string `yaml:"prefix,omitempty"`
	// Output defines the default output format
	Output string `yaml:"output,omitempty"`
	// File is the path where parameter values should be written
	File string `yaml:"file,omitempty"`
	// Upper determines if environment variable names should be uppercase
	Upper *bool `yaml:"upper,omitempty"`
	// EnvPrefix is prepended to all environment variable names
	EnvPrefix string `yaml:"env_prefix,omitempty"`
	// Role is the AWS IAM role to assume for operations
	Role string `yaml:"role,omitempty"`
	// KMS is the default KMS key ID for SecureString parameters
	KMS string `yaml:"kms,omitempty"`
	// Params defines specific parameter configurations
	Params []ParamConfig `yaml:"params,omitempty"`
}

// ParamConfig represents individual parameter configurations that can
// override global settings for specific parameters.
type ParamConfig struct {
	// Name is the full path of the parameter (required)
	Name string `yaml:"name"`
	// Env is the environment variable name to use (overrides default naming)
	Env string `yaml:"env,omitempty"`
	// Region overrides the global AWS region for this parameter
	Region string `yaml:"region,omitempty"`
	// Output overrides the global output format for this parameter
	Output string `yaml:"output,omitempty"`
}

// Validate checks if the configuration is valid.
// It ensures that required fields are present and have valid values.
func (c *Config) Validate() error {
	// If parameters are specified, each must have a name
	for i, param := range c.Params {
		if param.Name == "" {
			return fmt.Errorf("%w: parameter at index %d missing name", ErrInvalidConfig, i)
		}
	}

	// Validate output format if specified
	if c.Output != "" && c.Output != "env" && c.Output != "file" {
		return fmt.Errorf("%w: invalid output format %q (must be 'env' or 'file')", ErrInvalidConfig, c.Output)
	}

	return nil
}

// LoadConfig loads configuration from files with precedence:
// 1. Current directory (.params2env.yaml)
// 2. Home directory (~/.params2env.yaml)
//
// If a configuration file exists but cannot be loaded, a warning is printed
// and the function continues with any successfully loaded configuration.
// If no configuration files are found, returns an empty configuration.
func LoadConfig() (*Config, error) {
	var cfg Config

	// Try loading from home directory first
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

	// Try loading from current directory (overrides home config)
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

// fileExists checks if a file exists and is not a directory.
// It returns false if the file doesn't exist, is a directory,
// or if there's an error checking the file status.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

// loadFile loads and unmarshals a YAML configuration file.
// It returns an error if the file cannot be read or if the YAML
// is invalid.
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

// mergeConfig merges local configuration into global configuration.
// Local settings take precedence over global settings. For slices
// (like Params), the local values completely replace global values
// rather than being merged.
func mergeConfig(global, local *Config) {
	// Merge string fields
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

	// Merge pointer fields
	if local.Upper != nil {
		global.Upper = local.Upper
	}

	// Merge slice fields
	if len(local.Params) > 0 {
		global.Params = local.Params
	}
}

// sanitizeForLog removes control characters that could be used for log injection (CWE-117 mitigation)
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.ReplaceAll(s, "\x1b", "") // Remove escape sequences
}
