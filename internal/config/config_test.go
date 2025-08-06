// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type testEnv struct {
	tmpDir   string
	origHome string
	origWd   string
}

func setupTestEnv(t *testing.T, prefix string) *testEnv {
	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	return &testEnv{
		tmpDir:   tmpDir,
		origHome: origHome,
		origWd:   origWd,
	}
}

func (te *testEnv) cleanup(t *testing.T) {
	if err := os.RemoveAll(te.tmpDir); err != nil {
		t.Errorf("Failed to remove temp directory: %v", err)
	}
	if err := os.Setenv("HOME", te.origHome); err != nil {
		t.Errorf("Failed to restore HOME environment variable: %v", err)
	}
	if err := os.Chdir(te.origWd); err != nil {
		t.Errorf("Failed to change back to original directory: %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	te := setupTestEnv(t, "params2env-test")
	t.Cleanup(func() { te.cleanup(t) })

	// Create test files
	homeConfig := filepath.Join(te.tmpDir, ".params2env.yaml")
	localConfig := filepath.Join(te.tmpDir, "work", ".params2env.yaml")

	// Create home config
	homeContent := []byte(`
region: eu-central-1
replica: eu-west-1
prefix: /home/params
output: env
file: ~/.secrets
upper: true
env_prefix: HOME_
role: arn:aws:iam::123:role/home
kms: alias/myapp-key
params:
  - name: /home/secret
    env: HOME_SECRET
    region: us-east-1
`)
	if err := os.WriteFile(homeConfig, homeContent, 0644); err != nil {
		t.Fatalf("Failed to write home config: %v", err)
	}

	// Create local config directory and file
	if err := os.MkdirAll(filepath.Join(te.tmpDir, "work"), 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	localContent := []byte(`
region: us-west-2
prefix: /local/params
env_prefix: LOCAL_
role: arn:aws:iam::123:role/local
kms: alias/local-key
params:
  - name: /local/secret
    env: LOCAL_SECRET
`)
	if err := os.WriteFile(localConfig, localContent, 0644); err != nil {
		t.Fatalf("Failed to write local config: %v", err)
	}

	// Change to work directory for test
	if err := os.Chdir(filepath.Join(te.tmpDir, "work")); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Test configuration loading
	tests := []struct {
		name    string
		want    *Config
		wantErr bool
	}{
		{
			name: "load and merge configs",
			want: &Config{
				Region:    "us-west-2",                   // From local config
				Replica:   "eu-west-1",                   // From home config
				Prefix:    "/local/params",               // From local config
				Output:    "env",                         // From home config
				File:      "~/.secrets",                  // From home config
				Upper:     boolPtr(true),                 // From home config
				EnvPrefix: "LOCAL_",                      // From local config
				Role:      "arn:aws:iam::123:role/local", // From local config
				KMS:       "alias/local-key",             // From local config
				Params: []ParamConfig{
					{
						Name: "/local/secret",
						Env:  "LOCAL_SECRET",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfigNoFiles(t *testing.T) {
	te := setupTestEnv(t, "params2env-test-nofiles")
	defer te.cleanup(t)

	// Change to temp directory
	if err := os.Chdir(te.tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test loading with no config files
	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() error = %v, want no error", err)
	}
	if cfg == nil {
		t.Error("LoadConfig() returned nil, want empty config")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	te := setupTestEnv(t, "params2env-test-invalid")
	defer te.cleanup(t)

	// Create invalid config files
	homeConfig := filepath.Join(te.tmpDir, ".params2env.yaml")

	invalidContent := []byte(`
region: [invalid yaml
`)
	if err := os.WriteFile(homeConfig, invalidContent, 0644); err != nil {
		t.Fatalf("Failed to write invalid home config: %v", err)
	}

	// Create work directory and local config
	if err := os.MkdirAll(filepath.Join(te.tmpDir, "work"), 0755); err != nil {
		t.Fatalf("Failed to create work directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(te.tmpDir, "work", ".params2env.yaml"), invalidContent, 0644); err != nil {
		t.Fatalf("Failed to write invalid local config: %v", err)
	}

	// Change to work directory
	if err := os.Chdir(filepath.Join(te.tmpDir, "work")); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Test loading invalid config - should now fail fast
	cfg, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for invalid YAML")
	}
	if cfg != nil {
		t.Error("LoadConfig() returned config, want nil for invalid YAML")
	}
}

func TestLoadConfigHomeError(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Unsetenv("HOME")

	// Test loading config without HOME set
	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() error = %v, want no error when HOME is not set", err)
	}
	if cfg == nil {
		t.Error("LoadConfig() returned nil, want empty config")
	}
}

func TestLoadConfigFilePermissionError(t *testing.T) {
	te := setupTestEnv(t, "params2env-test-perms")
	defer te.cleanup(t)

	// Create config file with no read permissions
	configPath := filepath.Join(te.tmpDir, ".params2env.yaml")
	if err := os.WriteFile(configPath, []byte("region: us-west-2"), 0000); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test loading config with unreadable file - should now fail fast
	cfg, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error when config file is unreadable")
	}
	if cfg != nil {
		t.Error("LoadConfig() returned config, want nil when config file is unreadable")
	}
}

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name   string
		global *Config
		local  *Config
		want   *Config
	}{
		{
			name: "merge all fields",
			global: &Config{
				Region:    "us-west-2",
				Replica:   "us-east-1",
				Prefix:    "/global",
				Output:    "env",
				File:      "~/.env",
				Upper:     boolPtr(false),
				EnvPrefix: "GLOBAL_",
				Role:      "arn:aws:iam::123:role/global",
				KMS:       "alias/global-key",
				Params: []ParamConfig{
					{Name: "/global/param"},
				},
			},
			local: &Config{
				Region:    "eu-central-1",
				Replica:   "eu-west-1",
				Prefix:    "/local",
				Output:    "file",
				File:      "./local.env",
				Upper:     boolPtr(true),
				EnvPrefix: "LOCAL_",
				Role:      "arn:aws:iam::123:role/local",
				KMS:       "alias/local-key",
				Params: []ParamConfig{
					{Name: "/local/param"},
				},
			},
			want: &Config{
				Region:    "eu-central-1",
				Replica:   "eu-west-1",
				Prefix:    "/local",
				Output:    "file",
				File:      "./local.env",
				Upper:     boolPtr(true),
				EnvPrefix: "LOCAL_",
				Role:      "arn:aws:iam::123:role/local",
				KMS:       "alias/local-key",
				Params: []ParamConfig{
					{Name: "/local/param"},
				},
			},
		},
		{
			name: "merge empty local",
			global: &Config{
				Region:    "us-west-2",
				Replica:   "us-east-1",
				Prefix:    "/global",
				Output:    "env",
				File:      "~/.env",
				Upper:     boolPtr(false),
				EnvPrefix: "GLOBAL_",
				Role:      "arn:aws:iam::123:role/global",
				KMS:       "alias/global-key",
				Params: []ParamConfig{
					{Name: "/global/param"},
				},
			},
			local: &Config{},
			want: &Config{
				Region:    "us-west-2",
				Replica:   "us-east-1",
				Prefix:    "/global",
				Output:    "env",
				File:      "~/.env",
				Upper:     boolPtr(false),
				EnvPrefix: "GLOBAL_",
				Role:      "arn:aws:iam::123:role/global",
				KMS:       "alias/global-key",
				Params: []ParamConfig{
					{Name: "/global/param"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeConfig(tt.global, tt.local)
			if !reflect.DeepEqual(tt.global, tt.want) {
				t.Errorf("mergeConfig() = %v, want %v", tt.global, tt.want)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
