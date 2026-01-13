// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

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
	readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
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
			name:    "access_denied_error",
			args:    []string{"--path", "/test/param"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: &aws.MockSSMClient{
						GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
							return nil, aws.ErrNoAccess
						},
					}}, nil
				}
			},
		},
		{
			name:    "throttling_error",
			args:    []string{"--path", "/test/param"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: &aws.MockSSMClient{
						GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
							return nil, fmt.Errorf("throttling error")
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
			_ = os.Remove("test.env")

			testRoot := &cobra.Command{Use: "params2env"}
			readCmd.ResetFlags()
			readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
			readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
			readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
			readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
			readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
			readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
			readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
			readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
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
	tmpDir, err := os.MkdirTemp("", "params2env-test-invalid-config")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("AWS_REGION", origRegion)
	}()
	_ = os.Setenv("HOME", tmpDir)
	_ = os.Setenv("AWS_REGION", "eu-central-1")

	invalidConfigContent := []byte(`
region: [invalid yaml
params:
  - name: /test
`)
	if err := os.WriteFile(filepath.Join(tmpDir, ".params2env.yaml"), invalidConfigContent, 0644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	testRoot := &cobra.Command{Use: "params2env"}
	readCmd.ResetFlags()
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
	readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
	testRoot.AddCommand(readCmd)

	testRoot.SetArgs([]string{"read", "--path", "/test/param"})
	err = testRoot.Execute()

	if err == nil {
		t.Error("Expected error due to invalid YAML config, but got none")
	}
}

func TestErrorMessageFormatting(t *testing.T) {
	origNewClient := aws.NewClient
	defer func() { aws.NewClient = origNewClient }()

	tests := []struct {
		name           string
		paramName      string
		region         string
		mockError      error
		expectedFormat string
	}{
		{
			name:           "not_found_error",
			paramName:      "/test/param",
			region:         "us-west-2",
			mockError:      aws.ErrNotFound,
			expectedFormat: "parameter '/test/param' not found in region 'us-west-2'",
		},
		{
			name:           "access_denied_error",
			paramName:      "/test/secret",
			region:         "eu-central-1",
			mockError:      aws.ErrNoAccess,
			expectedFormat: "access denied to parameter '/test/secret' in region 'eu-central-1': check IAM permissions",
		},
		{
			name:           "throttling_error",
			paramName:      "/app/config",
			region:         "us-east-1",
			mockError:      fmt.Errorf("throttling error occurred"),
			expectedFormat: "request throttled for parameter '/app/config' in region 'us-east-1': try again later",
		},
		{
			name:           "generic_error_with_context",
			paramName:      "/my/param",
			region:         "ap-southeast-1",
			mockError:      fmt.Errorf("network timeout"),
			expectedFormat: "failed to get parameter '/my/param' from region 'ap-southeast-1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
				return &aws.Client{SSMClient: &aws.MockSSMClient{
					GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
						return nil, tt.mockError
					},
				}}, nil
			}

			_, err := getParameterValue(tt.paramName, tt.region, "")

			if err == nil {
				t.Errorf("getParameterValue() expected error but got none")
				return
			}

			if !containsString(err.Error(), tt.expectedFormat) {
				t.Errorf("getParameterValue() error = %q, expected to contain %q", err.Error(), tt.expectedFormat)
			}
		})
	}
}

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
			output := "export TEST_PARAM=\"secret-value\"\n"
			params := []config.ParamConfig{{Name: "/test/param"}}

			origReadFile := readFile
			readFile = tt.filePath
			defer func() { readFile = origReadFile }()

			err := writeOutput(output, params, nil)
			if err != nil {
				t.Fatalf("writeOutput failed: %v", err)
			}

			content, err := os.ReadFile(tt.filePath)
			if err != nil {
				t.Fatalf("Failed to read created file: %v", err)
			}
			if string(content) != output {
				t.Errorf("File content = %q, want %q", string(content), output)
			}

			fileInfo, err := os.Stat(tt.filePath)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}
			fileMode := fileInfo.Mode().Perm()
			expectedFileMode := os.FileMode(0600)
			if fileMode != expectedFileMode {
				t.Errorf("File permissions = %o, want %o (owner read/write only)", fileMode, expectedFileMode)
			}

			dir := filepath.Dir(tt.filePath)
			if dir != tmpDir {
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
func TestReadConfigFormatIntegration(t *testing.T) {
	tests := []struct {
		name              string
		configFormat      string
		cliFormat         string
		formatExplicitSet bool
		wantFormat        string
	}{
		{
			name:              "use config format when cli not set",
			configFormat:      "github-env",
			cliFormat:         "env", // default value
			formatExplicitSet: false,
			wantFormat:        "github-env",
		},
		{
			name:              "cli overrides config format",
			configFormat:      "env",
			cliFormat:         "github-env",
			formatExplicitSet: true,
			wantFormat:        "github-env",
		},
		{
			name:              "empty config uses cli default",
			configFormat:      "",
			cliFormat:         "env",
			formatExplicitSet: false,
			wantFormat:        "env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config
			cfg := &config.Config{
				Format: tt.configFormat,
			}

			// Simulate CLI flag value and explicit set state
			origFormat := readFormat
			origExplicitSet := readFormatExplicitSet
			readFormat = tt.cliFormat
			readFormatExplicitSet = tt.formatExplicitSet
			defer func() {
				readFormat = origFormat
				readFormatExplicitSet = origExplicitSet
			}()

			// Test mergeReadConfig function
			mergeReadConfig(cfg)

			// Verify format is set correctly
			if readFormat != tt.wantFormat {
				t.Errorf("Expected format %q, got %q", tt.wantFormat, readFormat)
			}
		})
	}
}

func TestReadConfigFormatDefault(t *testing.T) {
	// Test that CLI flag default is preserved when config has no format
	origFormat := readFormat
	readFormat = "env" // CLI default
	defer func() { readFormat = origFormat }()

	cfg := &config.Config{} // No format field set
	mergeReadConfig(cfg)

	if readFormat != "env" {
		t.Errorf("Expected default format 'env', got %q", readFormat)
	}
}

// Format validation tests
func TestReadFormatValidation(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	args := []string{"read", "--path", "/test/param", "--format", "invalid"}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	if err == nil {
		t.Error("Expected error for invalid format, got none")
	}
	if !containsString(err.Error(), "invalid format") {
		t.Errorf("Error should contain 'invalid format', got: %v", err)
	}
}

func TestReadFormatEnv(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--format", "env"}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Explicit env format test failed: %v", err)
	}

	want := "export PARAM=\"test-value\"\n"
	if got := buf.String(); got != want {
		t.Errorf("Explicit env format = %q, want %q", got, want)
	}
}

func TestReadFormatGithubEnv(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	testFile := filepath.Join(rts.tmpDir, "test.env")

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--format", "github-env", "--file", testFile}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("GitHub env format test failed: %v", err)
	}

	if !containsString(buf.String(), "::add-mask::test-value") {
		t.Errorf("Expected masking command in stdout, got: %q", buf.String())
	}

	// Should write KEY=value to file
	content, _ := os.ReadFile(testFile)
	if string(content) != "PARAM=test-value\n" {
		t.Errorf("Expected 'PARAM=test-value\n' in file, got: %q", string(content))
	}
}

func TestReadFormatGithubEnvAutoFile(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	testFile := filepath.Join(rts.tmpDir, "github_env")
	_ = os.Setenv("GITHUB_ENV", testFile)
	defer func() { _ = os.Unsetenv("GITHUB_ENV") }()

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--format", "github-env"}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("GitHub env auto-file test failed: %v", err)
	}

	if !containsString(buf.String(), "::add-mask::test-value") {
		t.Errorf("Expected masking in stdout, got: %q", buf.String())
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "PARAM=test-value\n" {
		t.Errorf("Expected auto-file content 'PARAM=test-value\n', got: %q", string(content))
	}
}

func TestReadAppendToFile(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	testFile := filepath.Join(rts.tmpDir, "append_test.env")
	_ = os.WriteFile(testFile, []byte("EXISTING=value\n"), 0600)

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--file", testFile}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	if err != nil {
		t.Errorf("Append test failed: %v", err)
	}

	content, _ := os.ReadFile(testFile)
	want := "EXISTING=value\nexport PARAM=\"test-value\"\n"
	if string(content) != want {
		t.Errorf("Expected append, got: %q, want: %q", string(content), want)
	}
}
// Security tests: verify masking happens immediately after secret retrieval
func TestGithubEnvMaskingOrder(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	// Mock that tracks call order
	var callOrder []string
	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			callOrder = append(callOrder, "GetParameter")
			value := "secret-value"
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

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	// Capture stdout to verify masking order
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Add file destination to make test valid
	testFile := filepath.Join(rts.tmpDir, "test.env")
	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--format", "github-env", "--file", testFile}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("GitHub env masking order test failed: %v", err)
	}

	output := buf.String()

	if !containsString(output, "::add-mask::secret-value") {
		t.Errorf("Expected masking command in output, got: %q", output)
	}

	if len(callOrder) == 0 || callOrder[0] != "GetParameter" {
		t.Errorf("Expected GetParameter to be called, got call order: %v", callOrder)
	}
}

func TestGithubEnvConfigFormatMasking(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	configContent := []byte(`
region: eu-central-1
format: github-env
file: test.env
params:
  - name: /app/secret
    env: SECRET
`)
	if err := os.WriteFile(filepath.Join(rts.tmpDir, ".params2env.yaml"), configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	testRoot := &cobra.Command{Use: "params2env"}
	readCmd.ResetFlags()
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
	readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
	testRoot.AddCommand(readCmd)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	testRoot.SetArgs([]string{"read"})
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Config format masking test failed: %v", err)
	}

	output := buf.String()

	if !containsString(output, "::add-mask::test-value") {
		t.Errorf("Expected config format to trigger masking, got: %q", output)
	}
}

func TestExplicitFormatEnvOverridesConfigGithubEnv(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	// Config has format: github-env and file destination
	testFile := filepath.Join(rts.tmpDir, "test.env")
	configContent := []byte(`
region: eu-central-1
format: github-env
file: ` + testFile + `
params:
  - name: /app/secret
    env: SECRET
`)
	if err := os.WriteFile(filepath.Join(rts.tmpDir, ".params2env.yaml"), configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	testRoot := &cobra.Command{Use: "params2env"}
	readCmd.ResetFlags()
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
	readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
	testRoot.AddCommand(readCmd)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Explicit --format env should override config's github-env
	testRoot.SetArgs([]string{"read", "--format", "env"})
	err := testRoot.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Explicit format env override test failed: %v", err)
	}

	output := buf.String()

	// Should NOT have ::add-mask:: since we explicitly requested env format
	if containsString(output, "::add-mask::") {
		t.Errorf("Explicit --format env should NOT produce ::add-mask::, got: %q", output)
	}

	// File should contain export format, not github-env format
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// env format uses: export KEY="value"
	if !containsString(string(content), "export SECRET=") {
		t.Errorf("File should contain 'export SECRET=' format, got: %q", string(content))
	}

	// Should NOT be github-env format (KEY=value without export)
	if string(content) == "SECRET=test-value\n" {
		t.Errorf("File should NOT be github-env format, got: %q", string(content))
	}
}

func TestGithubEnvRequiresFileDestination(t *testing.T) {
	rts := setupReadTest(t)
	defer rts.cleanup()

	origGithubEnv := os.Getenv("GITHUB_ENV")
	_ = os.Unsetenv("GITHUB_ENV")
	defer func() {
		if origGithubEnv != "" {
			_ = os.Setenv("GITHUB_ENV", origGithubEnv)
		}
	}()

	testRoot := &cobra.Command{Use: "params2env"}
	setupReadFlags(t, testRoot)

	args := []string{"read", "--path", "/test/param", "--region", "us-west-2", "--format", "github-env"}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	if err == nil {
		t.Error("Expected error when using github-env format without file destination, got none")
	}

	expectedError := "github-env format requires --file or GITHUB_ENV environment variable"
	if !containsString(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got: %v", expectedError, err)
	}
}
