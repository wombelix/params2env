// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"git.sr.ht/~wombelix/params2env/cmd"
)

func TestMain(t *testing.T) {
	// Save original args and restore after test
	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	}()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "version_flag",
			args:    []string{"params2env", "--version"},
			wantErr: false,
		},
		{
			name:    "help_flag",
			args:    []string{"params2env", "--help"},
			wantErr: false,
		},
		{
			name:    "invalid_flag",
			args:    []string{"params2env", "--invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test args
			os.Args = tt.args

			// Capture stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			os.Stdout = w

			// Run main and capture error
			var execErr error
			done := make(chan bool)
			go func() {
				execErr = cmd.Execute()
				w.Close()
				done <- true
			}()

			// Wait for execution to complete
			<-done
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				t.Fatalf("Failed to read captured output: %v", err)
			}

			// Check error condition
			if (execErr != nil) != tt.wantErr {
				t.Errorf("main() error = %v, wantErr %v", execErr, tt.wantErr)
			}

			// For version and help flags, verify we got some output
			if !tt.wantErr && buf.Len() == 0 {
				t.Error("Expected output for successful command, got none")
			}
		})
	}
}
