// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package cmd implements the command-line interface for params2env.
//
// It uses the cobra library to provide a rich CLI experience with subcommands
// for reading, creating, modifying, and deleting AWS SSM parameters. The package
// handles command-line argument parsing, configuration loading, and dispatching
// to the appropriate functionality.
//
// Global flags supported by all commands include:
//   - --loglevel: Set logging verbosity (debug, info, warn, error)
//   - --version: Display version information
//   - --help: Show help and usage information
package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~wombelix/params2env/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Build information, set via ldflags during build
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Command-line flags
	logLevel    string
	showVersion bool

	// rootCmd represents the base command when called without any subcommands.
	// It provides global flags and displays help information by default.
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

	// osExit allows tests to override os.Exit
	osExit = os.Exit
)

// init initializes the root command by setting up global flags and registering
// all subcommands. It also configures the persistent pre-run hook for logging
// initialization.
func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version information")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Initialize logger with the specified log level
		logger.InitLogger(logLevel)
		return nil
	}

	// Add subcommands
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(deleteCmd)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
// If there is an error, it will be returned to the caller.
func Execute() error {
	return rootCmd.Execute()
}

// printUsage displays detailed usage information for all commands and their options.
// This includes global flags and all subcommands with their respective options.
func printUsage() {
	fmt.Printf(`Usage: params2env [global options] <subcommand> [subcommand options]

A tool to manage AWS SSM Parameter Store entries.

Global options:
  --loglevel string   Log level (debug, info, warn, error) (default "info")
  --version           Show version information
  --help             Show this help message

Subcommands:
  read    Read a parameter from SSM Parameter Store
    Options:
      --path string     Parameter path (required)
      --region string   AWS region (optional, default: from AWS config or environment)
      --role string     AWS role ARN to assume (optional)

  create  Create a new parameter in SSM Parameter Store
    Options:
      --path string        Parameter path (required)
      --value string       Parameter value (required)
      --type string        Parameter type (String or SecureString) (default: String)
      --description string Parameter description (optional)
      --kms string         KMS key ID for SecureString parameters (optional)
      --region string      AWS region (optional, default: from AWS config or environment)
      --role string        AWS role ARN to assume (optional)
      --replica string     Region to replicate the parameter to (optional)
      --overwrite bool     Overwrite existing parameter (optional, default: false)

  modify  Modify an existing parameter in SSM Parameter Store
    Options:
      --path string        Parameter path (required)
      --value string       New parameter value (required)
      --description string New parameter description (optional)
      --region string      AWS region (optional, default: from AWS config or environment)
      --role string        AWS role ARN to assume (optional)
      --replica string     Region to replicate the parameter to (optional)

For more information, visit: https://git.sr.ht/~wombelix/params2env
`)
}
