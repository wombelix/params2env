// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type deleteFlags struct {
	path    string
	region  string
	role    string
	replica string
}

func setupDeleteFlags(t *testing.T) {
	// Reset global variables
	deletePath = ""
	deleteRegion = ""
	deleteRole = ""
	deleteReplica = ""

	deleteCmd.ResetFlags()
	deleteCmd.Flags().StringVar(&deletePath, "path", "", "Parameter path (required)")
	deleteCmd.Flags().StringVar(&deleteRegion, "region", "", "AWS region (optional)")
	deleteCmd.Flags().StringVar(&deleteRole, "role", "", "AWS role ARN to assume (optional)")
	deleteCmd.Flags().StringVar(&deleteReplica, "replica", "", "Region to delete the replica from")
	if err := deleteCmd.MarkFlagRequired("path"); err != nil {
		t.Fatalf("Failed to mark path flag as required: %v", err)
	}
	testRoot.AddCommand(deleteCmd)
}

func setupDeleteTest(t *testing.T) *testSetup {
	ts := setupTest(t)
	setupDeleteFlags(t)
	return ts
}

func TestRunDelete(t *testing.T) {
	ts := setupDeleteTest(t)
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

			runDeleteTest(t, ts, tt, func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
				switch tt.name {
				case "parameter_not_found":
					return nil, &types.ParameterNotFound{}
				default:
					return &ssm.DeleteParameterOutput{}, nil
				}
			})
		})
	}
}

func runDeleteTest(t *testing.T, ts *testSetup, tt struct {
	name    string
	flags   deleteFlags
	wantErr bool
}, mockFunc func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)) {
	ts.output.Reset()

	// Only setup mock client if we expect the command to reach AWS operations
	if tt.name != "missing_path" {
		mockClient := &aws.MockSSMClient{DeleteParamFunc: mockFunc}
		ts.setupMockClient(mockClient)
	}

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
}

func TestRunDeleteMissingPath(t *testing.T) {
	ts := setupDeleteTest(t)
	defer ts.cleanup()

	ts.output.Reset()
	args := []string{"delete"}
	testRoot.SetArgs(args)
	err := testRoot.Execute()

	if err == nil {
		t.Errorf("Expected error for missing path, but got nil")
	}
}

func TestRunDeleteWithConfig(t *testing.T) {
	ts := setupDeleteTest(t)
	defer ts.cleanup()

	configContent := []byte(`
region: eu-central-1
role: arn:aws:iam::123456789012:role/test
replica: eu-west-1
`)
	ts.setupConfigFile(t, configContent)

	tests := []struct {
		name    string
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
			runDeleteTest(t, ts, tt, func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
				return &ssm.DeleteParameterOutput{}, nil
			})
		})
	}
}
