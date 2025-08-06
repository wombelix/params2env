// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"
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

type deleteTestCase struct {
	name    string
	flags   deleteFlags
	wantErr bool
}

func runDeleteTest(t *testing.T, ts *testSetup, tt deleteTestCase, mockFunc func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)) {
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

// TestDeleteReplicaNotFound tests that replica deletion behavior is consistent with primary region.
// This test expects replica deletion to fail when parameter is not found, matching primary region behavior.
func TestDeleteReplicaNotFound(t *testing.T) {
	ts := setupDeleteTest(t)
	defer ts.cleanup()

	tests := []struct {
		name           string
		flags          deleteFlags
		primaryError   error
		replicaError   error
		wantErr        bool
		errorContains  string
	}{
		{
			name: "replica_not_found_should_fail",
			flags: deleteFlags{
				path:    "/test/param",
				region:  "us-west-2",
				replica: "eu-west-1",
			},
			primaryError:  nil, // Primary deletion succeeds
			replicaError:  &types.ParameterNotFound{}, // Replica not found
			wantErr:       true,
			errorContains: "not found in replica region",
		},
		{
			name: "both_regions_succeed",
			flags: deleteFlags{
				path:    "/test/param",
				region:  "us-west-2",
				replica: "eu-west-1",
			},
			primaryError: nil,
			replicaError: nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Track which region is being called to return appropriate error
			callCount := 0
			mockClient := &aws.MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					callCount++
					if callCount == 1 {
						// First call is primary region
						if tt.primaryError != nil {
							return nil, tt.primaryError
						}
						return &ssm.DeleteParameterOutput{}, nil
					} else {
						// Second call is replica region
						if tt.replicaError != nil {
							return nil, tt.replicaError
						}
						return &ssm.DeleteParameterOutput{}, nil
					}
				},
			}
			ts.setupMockClient(mockClient)

			args := buildArgs("delete", map[string]string{
				"path":    tt.flags.path,
				"region":  tt.flags.region,
				"replica": tt.flags.replica,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("TestDeleteReplicaNotFound() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("TestDeleteReplicaNotFound() error = %v, should contain %q", err, tt.errorContains)
				}
			}
		})
	}
}
