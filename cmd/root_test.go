// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestExecute(t *testing.T) {
	// Save original osExit and restore after tests
	origOsExit := osExit
	defer func() { osExit = origOsExit }()

	// Save and restore environment
	origRegion := os.Getenv("AWS_REGION")
	defer func() { os.Setenv("AWS_REGION", origRegion) }()
	os.Setenv("AWS_REGION", "eu-central-1")

	// Override osExit for testing
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic(exitCode) // Use panic to stop execution but allow deferred functions
	}

	// Create mock AWS client
	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			value := "test-value"
			return &ssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Value: &value,
				},
			}, nil
		},
		PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
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
			name:    "show_version",
			args:    []string{"--version"},
			wantErr: false,
		},
		{
			name:    "show_help",
			args:    []string{"--help"},
			wantErr: false,
		},
		{
			name:    "no_subcommand",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "unknown_subcommand",
			args:    []string{"unknown"},
			wantErr: true,
		},
		{
			name:    "valid_subcommand_read",
			args:    []string{"read", "--path", "/test/param"},
			wantErr: false,
		},
		{
			name:    "valid_subcommand_create",
			args:    []string{"create", "--path", "/test/param", "--value", "test"},
			wantErr: false,
		},
		{
			name:    "valid_subcommand_modify",
			args:    []string{"modify", "--path", "/test/param", "--value", "test"},
			wantErr: false,
		},
		{
			name:    "valid_subcommand_delete",
			args:    []string{"delete", "--path", "/test/param"},
			wantErr: false,
		},
		{
			name:    "invalid_loglevel",
			args:    []string{"--loglevel", "invalid"},
			wantErr: false,
		},
		{
			name:    "invalid_flag",
			args:    []string{"--invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags and commands
			rootCmd.ResetFlags()
			rootCmd.ResetCommands()
			rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Log level (debug, info, warn, error)")
			rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version information")
			rootCmd.AddCommand(readCmd)
			rootCmd.AddCommand(createCmd)
			rootCmd.AddCommand(modifyCmd)
			rootCmd.AddCommand(deleteCmd)

			// Set args
			rootCmd.SetArgs(tt.args)

			// Execute command
			err := Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrintUsage(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var output strings.Builder
	if _, err := io.Copy(&output, r); err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	// Check if output contains essential parts
	essentialParts := []string{
		"Usage:",
		"params2env",
		"Global options:",
		"--loglevel",
		"--version",
		"--help",
		"Subcommands:",
		"read",
		"create",
		"modify",
	}

	for _, part := range essentialParts {
		if !strings.Contains(output.String(), part) {
			t.Errorf("printUsage() output missing %q", part)
		}
	}
}
