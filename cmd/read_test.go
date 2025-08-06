// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"
)

type readTestSetup struct {
	tmpDir        string
	origHome      string
	origRegion    string
	origNewClient aws.NewClientFunc
}

func setupReadTest(t *testing.T) *readTestSetup {
	tmpDir, err := os.MkdirTemp("", "params2env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	os.Setenv("HOME", tmpDir)
	os.Setenv("AWS_REGION", "eu-central-1")

	origNewClient := aws.NewClient
	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			value := "test-value"
			return &ssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Value: &value,
				},
			}, nil
		},
	}
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	return &readTestSetup{
		tmpDir:        tmpDir,
		origHome:      origHome,
		origRegion:    origRegion,
		origNewClient: origNewClient,
	}
}

func (rts *readTestSetup) cleanup() {
	_ = os.RemoveAll(rts.tmpDir)
	_ = os.Setenv("HOME", rts.origHome)
	_ = os.Setenv("AWS_REGION", rts.origRegion)
	aws.NewClient = rts.origNewClient
}

func setupReadFlags(t *testing.T, testRoot *cobra.Command) {
	readCmd.ResetFlags()
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
	if err := readCmd.MarkFlagRequired("path"); err != nil {
		t.Fatalf("Failed to mark path flag as required: %v", err)
	}
	testRoot.AddCommand(readCmd)
}

func TestRunRead(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantErr    bool
		mockError  error
		setupFunc  func()
	}{
		{
			name:    "missing_path",
			args:    []string{},
			wantErr: true,
		},
		{
			name:       "basic_read",
			args:       []string{"--path", "/test/param", "--region", "us-west-2"},
			wantOutput: "export PARAM=\"test-value\"\n",
		},
		{
			name:       "read_with_prefix",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--env-prefix", "APP"},
			wantOutput: "export APP_PARAM=\"test-value\"\n",
		},
		{
			name:       "read_with_env_name",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--env", "CUSTOM_NAME"},
			wantOutput: "export CUSTOM_NAME=\"test-value\"\n",
		},
		{
			name:       "read_with_file",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--file", "test.txt"},
			wantOutput: "",
		},
		{
			name:       "read_with_no_upper",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--upper=false"},
			wantOutput: "export param=\"test-value\"\n",
		},
		{
			name:    "aws_client_error",
			args:    []string{"--path", "/test/param", "--region", "invalid-region"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return nil, fmt.Errorf("invalid region")
				}
			},
		},
		{
			name:      "parameter_not_found",
			args:      []string{"--path", "/test/param"},
			wantErr:   true,
			mockError: fmt.Errorf("ParameterNotFound"),
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: &aws.MockSSMClient{
						GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
							return nil, fmt.Errorf("ParameterNotFound")
						},
					}}, nil
				}
			},
		},
		{
			name:    "file_write_error",
			args:    []string{"--path", "/test/param", "--file", "/invalid/path/test.env"},
			wantErr: true,
			setupFunc: func() {
				mockClient := &aws.MockSSMClient{
					GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
						value := "test-value"
						return &ssm.GetParameterOutput{
							Parameter: &types.Parameter{
								Value: &value,
							},
						}, nil
					},
				}
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: mockClient}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
				defer func() {
					aws.NewClient = rts.origNewClient
				}()
			}
			testRoot := &cobra.Command{Use: "params2env"}
			setupReadFlags(t, testRoot)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			args := append([]string{"read"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				t.Fatalf("Failed to read captured output: %v", err)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("runRead() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantOutput != "" {
				if got := buf.String(); got != tt.wantOutput {
					t.Errorf("runRead() output = %q, want %q", got, tt.wantOutput)
				}

				if readFile != "" {
					content, err := os.ReadFile(readFile)
					if err != nil {
						t.Errorf("Failed to read output file: %v", err)
					} else if string(content) != tt.wantOutput {
						t.Errorf("File content = %q, want %q", string(content), tt.wantOutput)
					}
				}
			}
		})
	}
}

func TestRunReadWithConfig(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	// Override mock client for config test
	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			value := "test-value-" + *input.Name
			return &ssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Value: &value,
				},
			}, nil
		},
	}
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	// Create config file
	configContent := []byte(`
region: eu-central-1
role: arn:aws:iam::123:role/test
env_prefix: APP
upper: true
params:
  - name: /app/db/url
    env: DB_URL
    region: us-east-1
  - name: /app/db/user
    env: DB_USER
  - name: /app/db/password
    env: DB_PASSWORD
`)
	if err := os.WriteFile(filepath.Join(rts.tmpDir, ".params2env.yaml"), configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "read_from_config",
			args:       []string{},
			wantOutput: "export APP_DB_URL=\"test-value-/app/db/url\"\nexport APP_DB_USER=\"test-value-/app/db/user\"\nexport APP_DB_PASSWORD=\"test-value-/app/db/password\"\n",
			wantErr:    false,
		},
		{
			name:       "override_config_with_path",
			args:       []string{"--path", "/custom/param"},
			wantOutput: "export APP_PARAM=\"test-value-/custom/param\"\n",
			wantErr:    false,
		},
		{
			name:       "write_to_file",
			args:       []string{"--file", "test.env"},
			wantOutput: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRoot := &cobra.Command{Use: "params2env"}
			readCmd.ResetFlags()
			readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
			readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
			readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
			readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
			readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
			readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
			readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
			testRoot.AddCommand(readCmd)

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			args := append([]string{"read"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				t.Fatalf("Failed to read captured output: %v", err)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("runRead() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantOutput != "" {
				if got := buf.String(); got != tt.wantOutput {
					t.Errorf("runRead() output = %q, want %q", got, tt.wantOutput)
				}
			}

			if readFile != "" {
				content, err := os.ReadFile(readFile)
				if err != nil {
					t.Errorf("Failed to read output file: %v", err)
				} else {
					expectedOutput := "export APP_DB_URL=\"test-value-/app/db/url\"\nexport APP_DB_USER=\"test-value-/app/db/user\"\nexport APP_DB_PASSWORD=\"test-value-/app/db/password\"\n"
					if string(content) != expectedOutput {
						t.Errorf("File content = %q, want %q", string(content), expectedOutput)
					}
				}
			}
		})
	}
}

func TestRunReadWithInvalidConfig(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "params2env-test-invalid-config")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Save and restore environment
	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("AWS_REGION", origRegion)
	}()
	_ = os.Setenv("HOME", tmpDir)
	_ = os.Setenv("AWS_REGION", "eu-central-1")

	// Create invalid config file
	invalidConfigContent := []byte(`
region: [invalid yaml
params:
  - name: /test
`)
	if err := os.WriteFile(filepath.Join(tmpDir, ".params2env.yaml"), invalidConfigContent, 0644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Create a test root command
	testRoot := &cobra.Command{Use: "params2env"}

	// Reset flags
	readCmd.ResetFlags()
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")

	// Add read command to test root
	testRoot.AddCommand(readCmd)

	// Execute command - should fail due to invalid config
	testRoot.SetArgs([]string{"read", "--path", "/test/param"})
	err = testRoot.Execute()

	// Should fail with config error
	if err == nil {
		t.Error("Expected error due to invalid YAML config, but got none")
	}
}

// TestSecureFilePermissions verifies that files and directories created by writeOutput
// have secure permissions to prevent unauthorized access to sensitive SSM parameter values.
// Directories are created with 0700 (owner access only) and files with 0600 (owner read/write only).
func TestSecureFilePermissions(t *testing.T) {
	tmpDir := t.TempDir() // Automatically cleaned up

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "file_in_existing_dir",
			filePath: filepath.Join(tmpDir, "test.env"),
		},
		{
			name:     "file_in_nested_dir",
			filePath: filepath.Join(tmpDir, "nested", "dir", "test.env"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test data
			output := "export TEST_PARAM=\"secret-value\"\n"
			params := []config.ParamConfig{{Name: "/test/param"}}

			// Set readFile to test path
			origReadFile := readFile
			readFile = tt.filePath
			defer func() { readFile = origReadFile }()

			// Call writeOutput to create file with secure permissions
			err := writeOutput(output, params, nil)
			if err != nil {
				t.Fatalf("writeOutput failed: %v", err)
			}

			// Verify file was created and has correct content
			content, err := os.ReadFile(tt.filePath)
			if err != nil {
				t.Fatalf("Failed to read created file: %v", err)
			}
			if string(content) != output {
				t.Errorf("File content = %q, want %q", string(content), output)
			}

			// Verify file permissions are secure (0600 - owner read/write only)
			fileInfo, err := os.Stat(tt.filePath)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}
			fileMode := fileInfo.Mode().Perm()
			expectedFileMode := os.FileMode(0600)
			if fileMode != expectedFileMode {
				t.Errorf("File permissions = %o, want %o (owner read/write only)", fileMode, expectedFileMode)
			}

			// Verify directory permissions are secure (0700 - owner access only)
			dir := filepath.Dir(tt.filePath)
			if dir != tmpDir { // Only check if we created nested directories
				dirInfo, err := os.Stat(dir)
				if err != nil {
					t.Fatalf("Failed to stat directory: %v", err)
				}
				dirMode := dirInfo.Mode().Perm()
				expectedDirMode := os.FileMode(0700)
				if dirMode != expectedDirMode {
					t.Errorf("Directory permissions = %o, want %o (owner access only)", dirMode, expectedDirMode)
				}
			}
		})
	}
}
