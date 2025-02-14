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
	"strings"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
)

func TestRunModify(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "params2env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore environment
	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("AWS_REGION", origRegion)
	}()
	os.Setenv("HOME", tmpDir)
	os.Setenv("AWS_REGION", "us-west-2")

	// Create mock AWS client
	mockClient := &aws.MockSSMClient{
		PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
	}

	// Save original NewClient and restore after tests
	origNewClient := aws.NewClient
	defer func() { aws.NewClient = origNewClient }()

	// Override NewClient for testing
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	// Save original osExit and restore after tests
	origOsExit := osExit
	defer func() { osExit = origOsExit }()

	// Override osExit for testing
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic(exitCode) // Use panic to stop execution but allow deferred functions
	}

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantExit   bool
		wantOutput string
		mockError  error
		setupFunc  func()
	}{
		{
			name:     "missing path",
			args:     []string{},
			wantErr:  true,
			wantExit: true,
		},
		{
			name:     "missing value",
			args:     []string{"--path", "/test/param"},
			wantErr:  true,
			wantExit: true,
		},
		{
			name:       "basic modify",
			args:       []string{"--path", "/test/param", "--value", "new-value"},
			wantErr:    false,
			wantOutput: "Successfully modified parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "modify with description",
			args:       []string{"--path", "/test/param", "--value", "new-value", "--description", "Updated parameter"},
			wantErr:    false,
			wantOutput: "Successfully modified parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "modify with replica",
			args:       []string{"--path", "/test/param", "--value", "new-value", "--replica", "eu-west-1"},
			wantErr:    false,
			wantOutput: "Successfully modified parameter '/test/param' in region 'us-west-2'\nSuccessfully modified parameter '/test/param' in replica region 'eu-west-1'\n",
		},
		{
			name:    "aws_client_error",
			args:    []string{"--path", "/test/param", "--value", "test", "--region", "invalid-region"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return nil, fmt.Errorf("invalid region")
				}
			},
		},
		{
			name:    "parameter_modification_error",
			args:    []string{"--path", "/test/param", "--value", "test"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: &aws.MockSSMClient{
						PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
							return nil, fmt.Errorf("modification error")
						},
					}}, nil
				}
			},
		},
		{
			name:    "replica_client_error",
			args:    []string{"--path", "/test/param", "--value", "test", "--replica", "invalid-region"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					if region == "invalid-region" {
						return nil, fmt.Errorf("invalid replica region")
					}
					return &aws.Client{SSMClient: mockClient}, nil
				}
			},
		},
		{
			name:    "replica_modification_error",
			args:    []string{"--path", "/test/param", "--value", "test", "--replica", "us-east-1"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					if region == "us-east-1" {
						return &aws.Client{SSMClient: &aws.MockSSMClient{
							PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
								return nil, fmt.Errorf("replica modification error")
							},
						}}, nil
					}
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
					aws.NewClient = origNewClient
				}()
			}
			// Create a test root command
			testRoot := &cobra.Command{Use: "params2env"}

			// Reset flags before each test
			modifyCmd.ResetFlags()
			modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
			modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value (required)")
			modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description (optional)")
			modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
			modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN to assume (optional)")
			modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Region to replicate the parameter to (optional)")

			// Add modify command to test root
			testRoot.AddCommand(modifyCmd)

			// Reset exit code
			exitCode = 0

			// Capture stdout if we expect output
			var buf bytes.Buffer
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						if tt.wantExit {
							err = fmt.Errorf("command exited with code %d", exitCode)
						} else if strings.Contains(fmt.Sprint(r), "invalid usage") {
							err = fmt.Errorf("invalid usage")
						} else {
							t.Errorf("unexpected panic: %v", r)
						}
					}
				}()
				// Execute command with "modify" prefix
				args := append([]string{"modify"}, tt.args...)
				testRoot.SetArgs(args)
				err = testRoot.Execute()
			}()

			if tt.wantOutput != "" {
				w.Close()
				os.Stdout = oldStdout
				if _, err := io.Copy(&buf, r); err != nil {
					t.Fatalf("Failed to read captured output: %v", err)
				}
				if got := buf.String(); got != tt.wantOutput {
					t.Errorf("runModify() output = %q, want %q", got, tt.wantOutput)
				}
			} else {
				w.Close()
				os.Stdout = oldStdout
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("runModify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunModifyWithConfig(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "params2env-test-config")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore environment
	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("AWS_REGION", origRegion)
	}()
	os.Setenv("HOME", tmpDir)
	os.Setenv("AWS_REGION", "us-west-2")

	// Create config file
	configContent := []byte(`
region: eu-central-1
replica: eu-west-1
role: arn:aws:iam::123:role/test
`)
	if err := os.WriteFile(filepath.Join(tmpDir, ".params2env.yaml"), configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create mock AWS client
	mockClient := &aws.MockSSMClient{
		PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
	}

	// Save original NewClient and restore after tests
	origNewClient := aws.NewClient
	defer func() { aws.NewClient = origNewClient }()

	// Override NewClient for testing
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	// Save original osExit and restore after tests
	origOsExit := osExit
	defer func() { osExit = origOsExit }()

	// Override osExit for testing
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic(exitCode) // Use panic to stop execution but allow deferred functions
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "use config defaults",
			args:    []string{"--path", "/test/param", "--value", "test"},
			wantErr: false,
		},
		{
			name:    "override config region",
			args:    []string{"--path", "/test/param", "--value", "test", "--region", "us-east-1"},
			wantErr: false,
		},
		{
			name:    "override config role",
			args:    []string{"--path", "/test/param", "--value", "test", "--role", "arn:aws:iam::123:role/other"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test root command
			testRoot := &cobra.Command{Use: "params2env"}

			// Reset flags before each test
			modifyCmd.ResetFlags()
			modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
			modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value (required)")
			modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description (optional)")
			modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
			modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN to assume (optional)")
			modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Region to replicate the parameter to (optional)")

			// Add modify command to test root
			testRoot.AddCommand(modifyCmd)

			// Reset exit code
			exitCode = 0

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						if strings.Contains(fmt.Sprint(r), "invalid usage") {
							err = fmt.Errorf("invalid usage")
						} else {
							err = fmt.Errorf("command exited with code %d", exitCode)
						}
					}
				}()
				// Execute command with "modify" prefix
				args := append([]string{"modify"}, tt.args...)
				testRoot.SetArgs(args)
				err = testRoot.Execute()
			}()

			if (err != nil) != tt.wantErr {
				t.Errorf("runModify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
