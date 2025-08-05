// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/config"
)

type modifyFlags struct {
	path        string
	value       string
	region      string
	role        string
	replica     string
	description string
}

func setupModifyFlags(t *testing.T) {
	// Reset flags before each test
	modifyCmd.ResetFlags()
	modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
	modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value (required)")
	modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description")
	modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region")
	modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN")
	modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Replica region")
	// Add modify command to test root
	testRoot.AddCommand(modifyCmd)
}

func runModifyTest(t *testing.T, ts *testSetup, flags modifyFlags, wantErr bool) {
	ts.output.Reset()
	setupModifyFlags(t)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runModifyTest(t, ts, tt.flags, tt.wantErr)
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
			runModifyTest(t, ts, tt.flags, tt.wantErr)
		})
	}
}
