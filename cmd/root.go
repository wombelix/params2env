// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

// Package cmd implements the CLI commands for params2env.
package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~wombelix/params2env/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Set via ldflags during build
	version = "dev"
	commit  = "none"
	date    = "unknown"

	logLevel    string
	showVersion bool

	rootCmd = &cobra.Command{
		Use:   "params2env",
		Short: "A tool to manage AWS SSM Parameter Store entries",
		Long: `params2env is a command-line tool for managing AWS SSM Parameter Store entries.
It allows you to read, create, and modify parameters, with support for replication
across regions and secure string parameters using KMS keys.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Printf("params2env version %s (commit %s, built on %s)\n", version, commit, date)
				return nil
			}
			return cmd.Help()
		},
	}

	osExit = os.Exit // overridable for tests
)

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version information")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		logger.InitLogger(logLevel)
		return nil
	}

	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(deleteCmd)

	rootCmd.InitDefaultCompletionCmd()
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
