// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type deleteFlags struct {
	path    string
	region  string
	role    string
	replica string
}

func TestRunDelete(t *testing.T) {
	ts := setupTest(t)
	defer ts.cleanup()

	tests := []struct {
		name    string
		flags   deleteFlags
		wantErr bool
	}{
		{
			name: "delete with role",
			flags: deleteFlags{
				path:   "/test/param",
				region: "us-west-2",
				role:   "arn:aws:iam::123456789012:role/test",
			},
			wantErr: false,
		},
		{
			name: "delete with replica",
			flags: deleteFlags{
				path:    "/test/param",
				region:  "us-west-2",
				replica: "eu-west-1",
			},
			wantErr: false,
		},
		{
			name:    "missing_path",
			flags:   deleteFlags{},
			wantErr: true,
		},
		{
			name: "basic_delete",
			flags: deleteFlags{
				path:   "/test/param",
				region: "us-west-2",
			},
			wantErr: false,
		},
		{
			name: "aws_client_error",
			flags: deleteFlags{
				path:   "/test/param",
				region: "invalid-region",
			},
			wantErr: true,
		},
		{
			name: "parameter_not_found",
			flags: deleteFlags{
				path: "/test/param",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Create mock AWS client with test-specific behavior
			mockClient := &aws.MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					switch tt.name {
					case "parameter_not_found":
						return nil, &types.ParameterNotFound{}
					default:
						return &ssm.DeleteParameterOutput{}, nil
					}
				},
			}

			ts.setupMockClient(mockClient)

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

			// Build args
			args := buildArgs("delete", map[string]string{
				"path":    tt.flags.path,
				"region":  tt.flags.region,
				"role":    tt.flags.role,
				"replica": tt.flags.replica,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("RunDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunDeleteWithConfig(t *testing.T) {
	ts := setupTest(t)
	defer ts.cleanup()

	configContent := []byte(`
region: eu-central-1
role: arn:aws:iam::123456789012:role/test
replica: eu-west-1
`)
	ts.setupConfigFile(t, configContent)

	tests := []struct {
		name    string
		cfg     *config.Config
		flags   deleteFlags
		wantErr bool
	}{
		{
			name:    "use_config_defaults",
			flags:   deleteFlags{path: "/test/param"},
			wantErr: false,
		},
		{
			name:    "override_config_region",
			flags:   deleteFlags{path: "/test/param", region: "us-east-1"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Create mock AWS client with test-specific behavior
			mockClient := &aws.MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					return &ssm.DeleteParameterOutput{}, nil
				},
			}

			ts.setupMockClient(mockClient)

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

			// Build args
			args := buildArgs("delete", map[string]string{
				"path":    tt.flags.path,
				"region":  tt.flags.region,
				"role":    tt.flags.role,
				"replica": tt.flags.replica,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("RunDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
