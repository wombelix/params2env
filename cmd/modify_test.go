// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"
)

type modifyFlags struct {
	path        string
	value       string
	region      string
	role        string
	replica     string
	description string
}



func runModifyTest(t *testing.T, ts *testSetup, flags modifyFlags, wantErr bool) {
	ts.output.Reset()
	setupModifyFlags()

	args := buildArgs("modify", map[string]string{
		"path":        flags.path,
		"value":       flags.value,
		"region":      flags.region,
		"role":        flags.role,
		"replica":     flags.replica,
		"description": flags.description,
	})

	testRoot.SetArgs(args)
	err := testRoot.Execute()

	if (err != nil) != wantErr {
		t.Errorf("runModify() error = %v, wantErr %v", err, wantErr)
	}
}

func TestRunModify(t *testing.T) {
	ts := setupTest(t)
	t.Cleanup(ts.cleanup)

	tests := []struct {
		name    string
		flags   modifyFlags
		wantErr bool
	}{
		{
			name:    "missing path",
			flags:   modifyFlags{},
			wantErr: true,
		},
		{
			name:    "missing value",
			flags:   modifyFlags{path: "/test/param"},
			wantErr: true,
		},
		{
			name:    "basic modify",
			flags:   modifyFlags{path: "/test/param", value: "new-value"},
			wantErr: true, // Will fail due to no AWS credentials in test
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Setup flags using helper
			setupModifyFlags()
			testRoot.AddCommand(modifyCmd)

			// Build args
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
