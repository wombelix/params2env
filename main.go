// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

// CLI tool to manage AWS SSM Parameter Store entries.
// Supports reading, creating, modifying, and deleting parameters.
// Config precedence: CLI flags > .params2env.yaml (cwd) > ~/.params2env.yaml
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
