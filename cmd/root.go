// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~wombelix/params2env/internal/logger"
	"github.com/spf13/cobra"
)

var (
	version     = "1.0.0"
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
				fmt.Printf("params2env version %s\n", version)
				return nil
			}
			return cmd.Help()
		},
	}

	// Global variables for testing
	osExit = os.Exit
)

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
func Execute() error {
	return rootCmd.Execute()
}

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
