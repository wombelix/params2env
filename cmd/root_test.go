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

func setupExecuteTest(t *testing.T) func() {
	// Save original osExit and restore after tests
	origOsExit := osExit
	// Save and restore environment
	origRegion := os.Getenv("AWS_REGION")
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
	// Override NewClient for testing
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	return func() {
		osExit = origOsExit
		_ = os.Setenv("AWS_REGION", origRegion)
		aws.NewClient = origNewClient
	}
}

func setupRootCmd() {
	rootCmd.ResetFlags()
	rootCmd.ResetCommands()
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version information")
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(deleteCmd)
}

func TestExecuteVersion(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()
	setupRootCmd()
	rootCmd.SetArgs([]string{"--version"})
	if err := Execute(); err != nil {
		t.Errorf("Execute() error = %v, wantErr false", err)
	}
}

func TestExecuteHelp(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()
	setupRootCmd()
	rootCmd.SetArgs([]string{"--help"})
	if err := Execute(); err != nil {
		t.Errorf("Execute() error = %v, wantErr false", err)
	}
}

func TestExecuteSubcommands(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"read", []string{"read", "--path", "/test/param"}, false},
		{"create", []string{"create", "--path", "/test/param", "--value", "test"}, false},
		{"modify", []string{"modify", "--path", "/test/param", "--value", "test"}, false},
		{"delete", []string{"delete", "--path", "/test/param"}, false},
		{"unknown", []string{"unknown"}, true},
		{"invalid_flag", []string{"--invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupRootCmd()
			rootCmd.SetArgs(tt.args)
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
