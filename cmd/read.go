// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/spf13/cobra"
)

var (
	readPath    string
	readRegion  string
	readRole    string
	readFile    string
	readUpper   bool
	readPrefix  string
	readEnvName string
)

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read a parameter from SSM Parameter Store",
	Long: `Read a parameter from SSM Parameter Store.

The parameter value will be printed to stdout in the format:
export PARAM="value"`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration to check for params
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}

		// Only require path if no params are defined in config
		if readPath == "" && (cfg == nil || len(cfg.Params) == 0) {
			return fmt.Errorf("required flag \"path\" not set and no parameters defined in config")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}

		// If path is not set but we have params in config, use those
		if readPath == "" && cfg != nil && len(cfg.Params) > 0 {
			var outputs []string
			for _, param := range cfg.Params {
				// Use param-specific settings or global settings
				paramRegion := param.Region
				if paramRegion == "" {
					paramRegion = cfg.Region
				}
				if paramRegion == "" {
					paramRegion = os.Getenv("AWS_REGION")
				}
				if paramRegion == "" {
					return fmt.Errorf("AWS region must be specified via config, --region, or AWS_REGION environment variable")
				}

				// Create AWS client for this parameter
				ctx := context.Background()
				client, err := aws.NewClient(ctx, paramRegion, readRole)
				if err != nil {
					return fmt.Errorf("failed to create AWS client: %w", err)
				}

				// Get parameter value
				value, err := client.GetParameter(ctx, param.Name)
				if err != nil {
					return fmt.Errorf("failed to get parameter %s: %w", param.Name, err)
				}

				// Format the output
				name := filepath.Base(param.Name)
				if param.Env != "" {
					name = param.Env
				}
				if cfg.EnvPrefix != "" && readPrefix == "" {
					readPrefix = cfg.EnvPrefix
				}
				if readPrefix != "" {
					name = readPrefix + "_" + name
				}
				if cfg.Upper != nil {
					readUpper = *cfg.Upper
				}
				if readUpper {
					name = strings.ToUpper(name)
				}
				outputs = append(outputs, fmt.Sprintf("export %s=%q", name, value))
			}

			output := strings.Join(outputs, "\n") + "\n"

			// Write output
			if readFile == "" && cfg.File != "" {
				readFile = cfg.File
			}
			if readFile != "" {
				// Ensure directory exists
				dir := filepath.Dir(readFile)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}

				// Print reading messages for each parameter
				for _, param := range cfg.Params {
					paramRegion := param.Region
					if paramRegion == "" {
						paramRegion = cfg.Region
					}
					if paramRegion == "" {
						paramRegion = os.Getenv("AWS_REGION")
					}
					fmt.Printf("Reading parameter '%s' from region '%s'\n", param.Name, paramRegion)
				}

				// Write to file
				if err := os.WriteFile(readFile, []byte(output), 0644); err != nil {
					return fmt.Errorf("failed to write to file: %w", err)
				}
				fmt.Printf("Parameter value written to %s\n", readFile)
				return nil
			}

			fmt.Print(output)
			return nil
		}

		// If path is set, handle single parameter case
		if readPath == "" {
			return fmt.Errorf("required flag \"path\" not set and no parameters defined in config")
		}

		// Merge config with flags (flags take precedence)
		if cfg != nil {
			if readRegion == "" {
				readRegion = cfg.Region
			}
			if readRole == "" {
				readRole = cfg.Role
			}
			if readPrefix == "" {
				readPrefix = cfg.EnvPrefix
			}
		}

		// If region is still empty, try AWS_REGION env var
		if readRegion == "" {
			readRegion = os.Getenv("AWS_REGION")
			if readRegion == "" {
				return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
			}
		}

		// Create AWS client
		ctx := context.Background()
		client, err := aws.NewClient(ctx, readRegion, readRole)
		if err != nil {
			return fmt.Errorf("failed to create AWS client: %w", err)
		}

		// Get parameter value
		value, err := client.GetParameter(ctx, readPath)
		if err != nil {
			return fmt.Errorf("failed to get parameter: %w", err)
		}

		// Format the output
		name := filepath.Base(readPath)
		if readEnvName != "" {
			name = readEnvName
		}
		if readPrefix != "" {
			name = readPrefix + "_" + name
		}
		if readUpper {
			name = strings.ToUpper(name)
		}
		output := fmt.Sprintf("export %s=%q\n", name, value)

		// Write output
		if readFile != "" {
			// Ensure directory exists
			dir := filepath.Dir(readFile)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}

			fmt.Printf("Reading parameter '%s' from region '%s'\n", readPath, readRegion)

			// Write to file
			if err := os.WriteFile(readFile, []byte(output), 0644); err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}
			fmt.Printf("Parameter value written to %s\n", readFile)
			return nil
		}

		fmt.Print(output)
		return nil
	},
}

func init() {
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
}
