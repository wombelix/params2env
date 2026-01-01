// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type modifyFlags struct {
	path        string
	value       string
	region      string
	role        string
	replica     string
	description string
}

func TestRunModify(t *testing.T) {
	ts := setupTest(t)
	t.Cleanup(ts.cleanup)

	tests := []struct {
		name       string
		flags      modifyFlags
		stdinValue string
		wantErr    bool
	}{
		{
			name:    "missing path",
			flags:   modifyFlags{},
			wantErr: true,
		},
		{
			name:    "basic_modify_with_value_flag",
			flags:   modifyFlags{path: "/test/param", value: "test"},
			wantErr: false,
		},
		{
			name:       "modify_via_stdin",
			flags:      modifyFlags{path: "/test/param"},
			stdinValue: "value-from-stdin",
			wantErr:    false,
		},
		{
			name:       "no_value_no_stdin",
			flags:      modifyFlags{path: "/test/param"},
			stdinValue: "",
			wantErr:    true,
		},
		{
			name:    "aws_client_error",
			flags:   modifyFlags{path: "/test/param", value: "test", region: "invalid-region"},
			wantErr: true,
		},
		{
			name:    "same_region_validation",
			flags:   modifyFlags{path: "/test/param", value: "test", region: "us-west-2", replica: "us-west-2"},
			wantErr: true,
		},
		{
			name:    "invalid_path_format",
			flags:   modifyFlags{path: "invalid-path", value: "test"},
			wantErr: true,
		},
		{
			name:    "invalid_role_arn",
			flags:   modifyFlags{path: "/test/param", value: "test", role: "invalid-role"},
			wantErr: true,
		},
		{
			name:       "value_with_spaces_preserved",
			flags:      modifyFlags{path: "/test/param"},
			stdinValue: "  value with spaces  ",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			existingValue := "existing-value"
			mockClient := &aws.MockSSMClient{
				GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &ssmtypes.Parameter{
							Value: &existingValue,
						},
					}, nil
				},
				PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
					return &ssm.PutParameterOutput{}, nil
				},
			}
			ts.setupMockClient(mockClient)

			oldStdin := os.Stdin
			if tt.flags.value == "" {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatalf("Failed to create pipe: %v", err)
				}
				if tt.stdinValue != "" {
					if _, err := w.WriteString(tt.stdinValue + "\n"); err != nil {
						t.Fatalf("Failed to write to pipe: %v", err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatalf("Failed to close pipe: %v", err)
				}
				os.Stdin = r
			}
			defer func() { os.Stdin = oldStdin }()

			setupModifyFlags()
			testRoot.AddCommand(modifyCmd)

			args := buildArgs("modify", map[string]string{
				"path":        tt.flags.path,
				"value":       tt.flags.value,
				"region":      tt.flags.region,
				"role":        tt.flags.role,
				"replica":     tt.flags.replica,
				"description": tt.flags.description,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("runModify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
