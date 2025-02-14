// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
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

func TestRunCreate(t *testing.T) {
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
			args:    []string{"--value", "test"},
			wantErr: true,
		},
		{
			name:    "missing_value",
			args:    []string{"--path", "/test/param"},
			wantErr: true,
		},
		{
			name:       "basic_create",
			args:       []string{"--path", "/test/param", "--value", "test", "--region", "us-west-2"},
			wantOutput: "Successfully created parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "create_with_description",
			args:       []string{"--path", "/test/param", "--value", "test", "--description", "Test parameter", "--region", "us-west-2"},
			wantOutput: "Successfully created parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "create_with_replica",
			args:       []string{"--path", "/test/param", "--value", "test", "--region", "us-west-2", "--replica", "eu-west-1"},
			wantOutput: "Successfully created parameter '/test/param' in region 'us-west-2'\nSuccessfully created parameter '/test/param' in replica region 'eu-west-1'\n",
		},
		{
			name:       "create_with_kms",
			args:       []string{"--path", "/test/param", "--value", "test", "--type", "SecureString", "--kms", "alias/aws/ssm", "--region", "us-west-2"},
			wantOutput: "Successfully created parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "create_with_explicit_String_type",
			args:       []string{"--path", "/test/param", "--value", "test", "--type", "String", "--region", "us-west-2"},
			wantOutput: "Successfully created parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:    "invalid_type",
			args:    []string{"--path", "/test/param", "--value", "test", "--type", "Invalid", "--region", "us-west-2"},
			wantErr: true,
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
			name:    "parameter_creation_error",
			args:    []string{"--path", "/test/param", "--value", "test"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					return &aws.Client{SSMClient: &aws.MockSSMClient{
						PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
							return nil, fmt.Errorf("creation error")
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
			name:    "replica_creation_error",
			args:    []string{"--path", "/test/param", "--value", "test", "--replica", "us-east-1"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					if region == "us-east-1" {
						return &aws.Client{SSMClient: &aws.MockSSMClient{
							PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
								return nil, fmt.Errorf("replica creation error")
							},
						}}, nil
					}
					return &aws.Client{SSMClient: mockClient}, nil
				}
			},
		},
		{
			name:    "missing_kms_key_for_secure_string",
			args:    []string{"--path", "/test/param", "--value", "test", "--type", "SecureString"},
			wantErr: true,
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
			createCmd.ResetFlags()
			createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
			createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
			createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type (String or SecureString)")
			createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
			createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID for SecureString parameters")
			createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
			createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN to assume (optional)")
			createCmd.Flags().StringVar(&createReplica, "replica", "", "Region to replicate the parameter to")
			createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing parameter")

			// Add create command to test root
			testRoot.AddCommand(createCmd)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Execute command with "create" prefix
			args := append([]string{"create"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var output strings.Builder
			if _, err := io.Copy(&output, r); err != nil {
				t.Fatalf("Failed to read captured output: %v", err)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantOutput != "" && output.String() != tt.wantOutput {
				t.Errorf("runCreate() output = %q, want %q", output.String(), tt.wantOutput)
			}
		})
	}
}

func TestRunCreateWithConfig(t *testing.T) {
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

	// Create config file
	configContent := []byte(`
region: eu-central-1
replica: us-east-1
role: arn:aws:iam::123:role/test
`)
	configFile := filepath.Join(tmpDir, ".params2env.yaml")
	if err := os.WriteFile(configFile, configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
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
			createCmd.ResetFlags()
			createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
			createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
			createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type (String or SecureString)")
			createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
			createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID for SecureString parameters")
			createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
			createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN to assume (optional)")
			createCmd.Flags().StringVar(&createReplica, "replica", "", "Region to replicate the parameter to")
			createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing parameter")

			// Add create command to test root
			testRoot.AddCommand(createCmd)

			// Execute command with "create" prefix
			args := append([]string{"create"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
