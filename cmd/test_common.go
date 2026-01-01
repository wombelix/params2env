// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/spf13/cobra"
)

func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

var testRoot = &cobra.Command{Use: "params2env"}

type testSetup struct {
	output        *bytes.Buffer
	tmpDir        string
	origHome      string
	origRegion    string
	origNewClient aws.NewClientFunc
	origStdout    *os.File
	cleanup       func()
}

func setupTest(t *testing.T) *testSetup {
	var output bytes.Buffer
	testRoot.SetOut(&output)

	tmpDir, err := os.MkdirTemp("", "params2env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	origRegion := os.Getenv("AWS_REGION")
	origNewClient := aws.NewClient
	origStdout := os.Stdout

	os.Setenv("HOME", tmpDir)
	os.Setenv("AWS_REGION", "us-west-2")

	cleanup := func() {
		os.RemoveAll(tmpDir)
		os.Setenv("HOME", origHome)
		os.Setenv("AWS_REGION", origRegion)
		aws.NewClient = origNewClient
		os.Stdout = origStdout
	}

	return &testSetup{
		output:        &output,
		tmpDir:        tmpDir,
		origHome:      origHome,
		origRegion:    origRegion,
		origNewClient: origNewClient,
		origStdout:    origStdout,
		cleanup:       cleanup,
	}
}

func (ts *testSetup) setupMockClient(mockClient *aws.MockSSMClient) {
	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: mockClient}, nil
	}
}

func (ts *testSetup) setupConfigFile(t *testing.T, content []byte) {
	if err := os.WriteFile(filepath.Join(ts.tmpDir, ".params2env.yaml"), content, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
}

func buildArgs(command string, flags map[string]string) []string {
	args := []string{command}
	for flag, value := range flags {
		if value != "" {
			args = append(args, "--"+flag, value)
		}
	}
	return args
}

func setupCreateFlags() {
	createCmd.ResetFlags()
	createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
	createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value")
	createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
	createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID")
	createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region")
	createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN")
	createCmd.Flags().StringVar(&createReplica, "replica", "", "Replica region")
	createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing")
}

func setupModifyFlags() {
	modifyCmd.ResetFlags()
	modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
	modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value")
	modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description")
	modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region")
	modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN")
	modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Replica region")
}
