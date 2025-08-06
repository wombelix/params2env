// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/config"
)

// containsString checks if a string contains a substring (case-insensitive)
func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

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

func TestModifyInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   string
		region  string
		replica string
		role    string
		wantErr bool
		errMsg  string
	}{
		{"valid_input", "/test/param", "value", "us-west-2", "us-east-1", "", false, ""},
		{"empty_path", "", "value", "us-west-2", "", "", true, "path\" not set"},
		{"empty_value", "/test/param", "", "us-west-2", "", "", true, "value\" not set"},
		{"invalid_path", "invalid-path", "value", "us-west-2", "", "", true, "parameter path"},
		{"invalid_region", "/test/param", "value", "invalid-region", "", "", true, "invalid region"},
		{"invalid_replica", "/test/param", "value", "us-west-2", "invalid-region", "", true, "invalid replica region"},
		{"invalid_role", "/test/param", "value", "us-west-2", "", "invalid-role", true, "invalid role ARN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables for validation
			modifyPath = tt.path
			modifyValue = tt.value
			modifyRegion = tt.region
			modifyReplica = tt.replica
			modifyRole = tt.role

			// Test validation function directly (focuses on input validation only)
			err := validateModifyFlags(nil, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateModifyFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("validateModifyFlags() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestRunModifyWithConfig(t *testing.T) {
	ts := setupTest(t)
	defer ts.cleanup()

	tests := []struct {
		name    string
		cfg     *config.Config
		flags   modifyFlags
		wantErr bool
	}{
		{
			name:    "use config defaults",
			cfg:     &config.Config{},
			flags:   modifyFlags{path: "/test/param", value: "test-value"},
			wantErr: true, // Will fail due to no AWS credentials
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
