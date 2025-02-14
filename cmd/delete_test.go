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
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
)

func TestRunDelete(t *testing.T) {
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
		DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			return &ssm.DeleteParameterOutput{}, nil
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
			args:    []string{},
			wantErr: true,
		},
		{
			name:       "basic_delete",
			args:       []string{"--path", "/test/param", "--region", "us-west-2"},
			wantOutput: "Deleting parameter '/test/param' in region 'us-west-2'...\nSuccessfully deleted parameter '/test/param' in region 'us-west-2'\n",
		},
		{
			name:       "delete_with_replica",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--replica", "us-east-1"},
			wantOutput: "Deleting parameter '/test/param' in region 'us-west-2'...\nSuccessfully deleted parameter '/test/param' in region 'us-west-2'\nDeleting parameter '/test/param' in replica region 'us-east-1'...\nSuccessfully deleted parameter '/test/param' in replica region 'us-east-1'\n",
		},
		{
			name:       "delete_with_role",
			args:       []string{"--path", "/test/param", "--region", "us-west-2", "--role", "arn:aws:iam::123:role/test"},
			wantOutput: "Deleting parameter '/test/param' in region 'us-west-2'...\nSuccessfully deleted parameter '/test/param' in region 'us-west-2'\n",
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
						DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
							return nil, fmt.Errorf("ParameterNotFound")
						},
					}}, nil
				}
			},
		},
		{
			name:    "replica_client_error",
			args:    []string{"--path", "/test/param", "--replica", "invalid-region"},
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
			name:    "replica_delete_error",
			args:    []string{"--path", "/test/param", "--replica", "us-east-1"},
			wantErr: true,
			setupFunc: func() {
				aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
					if region == "us-east-1" {
						return &aws.Client{SSMClient: &aws.MockSSMClient{
							DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
								return nil, fmt.Errorf("delete error")
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
			deleteCmd.ResetFlags()
			deleteCmd.Flags().StringVar(&deletePath, "path", "", "Parameter path (required)")
			deleteCmd.Flags().StringVar(&deleteRegion, "region", "", "AWS region (optional)")
			deleteCmd.Flags().StringVar(&deleteRole, "role", "", "AWS role ARN to assume (optional)")
			deleteCmd.Flags().StringVar(&deleteReplica, "replica", "", "Region to delete the replica from")
			if err := deleteCmd.MarkFlagRequired("path"); err != nil {
				t.Fatalf("Failed to mark path flag as required: %v", err)
			}

			// Add delete command to test root
			testRoot.AddCommand(deleteCmd)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Execute command with "delete" prefix
			args := append([]string{"delete"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				t.Fatalf("Failed to read captured output: %v", err)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("runDelete() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantOutput != "" {
				if got := buf.String(); got != tt.wantOutput {
					t.Errorf("runDelete() output = %q, want %q", got, tt.wantOutput)
				}
			}
		})
	}
}

func TestRunDeleteWithConfig(t *testing.T) {
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
role: arn:aws:iam::123:role/test
replica: eu-west-1
`)
	if err := os.WriteFile(filepath.Join(tmpDir, ".params2env.yaml"), configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create mock AWS client
	mockClient := &aws.MockSSMClient{
		DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			return &ssm.DeleteParameterOutput{}, nil
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
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "use config defaults",
			args:    []string{"--path", "/test/param"},
			wantErr: false,
		},
		{
			name:    "override config region",
			args:    []string{"--path", "/test/param", "--region", "us-east-1"},
			wantErr: false,
		},
		{
			name:    "override config role",
			args:    []string{"--path", "/test/param", "--role", "arn:aws:iam::123:role/other"},
			wantErr: false,
		},
		{
			name:    "override config replica",
			args:    []string{"--path", "/test/param", "--replica", "us-west-1"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test root command
			testRoot := &cobra.Command{Use: "params2env"}

			// Reset flags before each test
			deleteCmd.ResetFlags()
			deleteCmd.Flags().StringVar(&deletePath, "path", "", "Parameter path (required)")
			deleteCmd.Flags().StringVar(&deleteRegion, "region", "", "AWS region (optional)")
			deleteCmd.Flags().StringVar(&deleteRole, "role", "", "AWS role ARN to assume (optional)")
			deleteCmd.Flags().StringVar(&deleteReplica, "replica", "", "Region to delete the replica from")
			if err := deleteCmd.MarkFlagRequired("path"); err != nil {
				t.Fatalf("Failed to mark path flag as required: %v", err)
			}

			// Add delete command to test root
			testRoot.AddCommand(deleteCmd)

			// Execute command with "delete" prefix
			args := append([]string{"delete"}, tt.args...)
			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("runDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
