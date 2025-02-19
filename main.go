// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package main provides the entry point for the params2env CLI tool.
//
// params2env is a command-line tool for managing AWS SSM Parameter Store entries.
// It allows users to read, create, modify, and delete parameters, with support for
// replication across regions and secure string parameters using KMS keys.
//
// The tool supports both command line flags and YAML configuration files with the
// following precedence:
//  1. CLI arguments (highest priority)
//  2. Configuration file in the current directory (.params2env.yaml)
//  3. Configuration file in the home directory (~/.params2env.yaml)
package main

import (
	"log/slog"
	"os"

	"git.sr.ht/~wombelix/params2env/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
