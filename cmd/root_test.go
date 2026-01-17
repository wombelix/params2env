// SPDX-FileCopyrightText: 2026 Dominik Wombacher <dominik@wombelix.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// --- Test scaffolding ---
func setupExecuteTest(t *testing.T) func() {
	origOsExit := osExit
	origRegion := os.Getenv("AWS_REGION")
	origHome := os.Getenv("HOME")
	tmpDir, err := os.MkdirTemp("", "params2env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	_ = os.Setenv("AWS_REGION", "eu-central-1")
	_ = os.Setenv("HOME", tmpDir)

	osExit = func(code int) {
		panic(code)
	}

	// Replace AWS client globally
	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			value := "test-value"
			return &ssm.GetParameterOutput{
				Parameter: &types.Parameter{Value: &value},
			}, nil
		},
	}
	origNewClient := aws.NewClient
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}

	return func() {
		osExit = origOsExit
		_ = os.Setenv("AWS_REGION", origRegion)
		_ = os.Setenv("HOME", origHome)
		_ = os.RemoveAll(tmpDir)
		aws.NewClient = origNewClient
	}
}

func setupRootCmd() {
	rootCmd.ResetFlags()
	rootCmd.ResetCommands()
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Log level")
	rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version")
	// reset globals
	readPath, readRegion, readRole, readFile, readEnvName, readPrefix = "", "", "", "", "", ""
	readUpper, readFormat, readFormatExplicitSet = true, "env", false
	modifyPath, modifyValue, modifyDesc, modifyRegion, modifyRole, modifyReplica = "", "", "", "", "", ""

	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(deleteCmd)
}

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outC <- buf.String()
	}()

	f()

	_ = w.Close()
	os.Stdout = stdout
	return <-outC
}

func TestReadSingleLineGithubEnv(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()
	setupRootCmd()

	tmpEnvFile := t.TempDir() + "/GITHUB_ENV"
	_ = os.Setenv("GITHUB_ENV", tmpEnvFile)
	readFormat = "github-env"

	rootCmd.SetArgs([]string{"read", "--path", "/test/singleline"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	data, err := os.ReadFile(tmpEnvFile)
	if err != nil {
		t.Fatalf("failed to read github env file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "test-value") {
		t.Errorf("Expected value 'test-value' in env file, got:\n%s", content)
	}
	if strings.Contains(content, "<<EOF") {
		t.Errorf("Single-line value should not use <<EOF, got:\n%s", content)
	}
}

func TestReadMultiLineGithubEnv(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()
	setupRootCmd()

	multiLineValue := `-----BEGIN OPENSSH PRIVATE KEY-----
line1
line2
line3
-----END OPENSSH PRIVATE KEY-----`

	mockClient := &aws.MockSSMClient{
		GetParamFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &types.Parameter{Value: &multiLineValue},
			}, nil
		},
	}
	origNewClient := aws.NewClient
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}
	defer func() { aws.NewClient = origNewClient }()

	tmpEnvFile := t.TempDir() + "/GITHUB_ENV"
	_ = os.Setenv("GITHUB_ENV", tmpEnvFile)
	readFormat = "github-env"

	out := captureOutput(func() {
		rootCmd.SetArgs([]string{"read", "--path", "/test/multiline"})
		if err := Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})

	// Env file should contain <<EOF block
	data, err := os.ReadFile(tmpEnvFile)
	if err != nil {
		t.Fatalf("failed to read github env file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "<<EOF") {
		t.Errorf("Expected <<EOF syntax for multi-line value, got:\n%s", content)
	}

	// All lines preserved
	for _, line := range []string{"line1", "line2", "line3"} {
		if !strings.Contains(content, line) {
			t.Errorf("Expected line '%s' in env file, got:\n%s", line, content)
		}
	}

	// add-mask should appear in stdout
	if !strings.Contains(out, "::add-mask::") {
		t.Errorf("Expected add-mask output, got:\n%s", out)
	}
}

func TestReadEnvShellFormat(t *testing.T) {
	cleanup := setupExecuteTest(t)
	defer cleanup()
	setupRootCmd()

	readFormat = "env"
	tmpFile := t.TempDir() + "/.env"
	readFile = tmpFile

	rootCmd.SetArgs([]string{"read", "--path", "/test/env"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read env file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `export`) {
		t.Errorf("Expected export line, got:\n%s", content)
	}
	if !strings.Contains(content, "test-value") {
		t.Errorf("Expected value 'test-value', got:\n%s", content)
	}
}
