// SPDX-FileCopyrightText: 2026 Dominik Wombacher <dominik@wombelix.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

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
