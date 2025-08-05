// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"git.sr.ht/~wombelix/params2env/internal/validation"
	"github.com/spf13/cobra"
)

// Command-line flags for the read command
var (
	// readPath is the full path of the parameter to read
	readPath string
	// readRegion is the AWS region where the parameter will be read from
	readRegion string
	// readRole is the AWS IAM role to assume for the operation
	readRole string
	// readFile is the path to write the parameter value to
	readFile string
	// readUpper determines if the environment variable name should be uppercase
	readUpper bool
	// readPrefix is prepended to the environment variable name
	readPrefix string
	// readEnvName overrides the default environment variable name
	readEnvName string
)

// readCmd represents the read command
var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read a parameter from SSM Parameter Store",
	Long: `Read a parameter from SSM Parameter Store.

The parameter value will be printed to stdout in the format:
export PARAM="value"

Examples:
  # Read a single parameter
  params2env read --path /myapp/config/url

  # Read a parameter and write to a file
  params2env read --path /myapp/config/url --file /etc/env.d/myapp

  # Read a parameter with custom environment variable name
  params2env read --path /myapp/config/url --env MY_URL

  # Read a parameter with prefix and uppercase name
  params2env read --path /myapp/config/url --env-prefix MYAPP --upper`,
	PreRunE: validateReadFlags,
	RunE:    runRead,
}

// validateReadFlags checks if all required flags are set and valid
func validateReadFlags(cmd *cobra.Command, args []string) error {
	// Load config to check if parameters are defined
	cfg, _ := config.LoadConfig()

	// Path is required only if no parameters are defined in config
	if readPath == "" && (cfg == nil || len(cfg.Params) == 0) {
		return fmt.Errorf("required flag \"path\" not set")
	}

	if readPath != "" {
		if err := validation.ValidateParameterPath(readPath); err != nil {
			return err
		}
	}

	if readRegion != "" {
		if err := validation.ValidateRegion(readRegion); err != nil {
			return err
		}
	}

	if readRole != "" {
		if err := validation.ValidateRoleARN(readRole); err != nil {
			return err
		}
	}

	return nil
}

// runRead executes the read command
func runRead(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
	}

	// If path is not set but we have params in config, use those
	if readPath == "" && cfg != nil && len(cfg.Params) > 0 {
		return handleConfigParameters(cfg)
	}

	// Handle single parameter case
	return handleSingleParameter(cfg)
}

// handleConfigParameters processes parameters defined in the configuration
func handleConfigParameters(cfg *config.Config) error {
	var outputs []string
	for _, param := range cfg.Params {
		// Get parameter value
		value, err := getParameterValue(param.Name, param.Region, cfg.Region)
		if err != nil {
			return err
		}

		// Format the output
		name := formatEnvName(param.Name, param.Env, cfg)
		outputs = append(outputs, fmt.Sprintf("export %s=%q", name, value))
	}

	output := strings.Join(outputs, "\n") + "\n"
	return writeOutput(output, cfg.Params, cfg)
}

// handleSingleParameter processes a single parameter specified via command line
func handleSingleParameter(cfg *config.Config) error {
	// Merge config with flags (flags take precedence)
	mergeReadConfig(cfg)

	// Ensure region is set
	if err := ensureReadRegionIsSet(); err != nil {
		return err
	}

	// Get parameter value
	value, err := getParameterValue(readPath, readRegion, "")
	if err != nil {
		return err
	}

	// Format the output
	name := formatEnvName(readPath, readEnvName, cfg)
	output := fmt.Sprintf("export %s=%q\n", name, value)

	return writeOutput(output, []config.ParamConfig{{Name: readPath}}, cfg)
}

// mergeReadConfig merges configuration from file with command line flags
func mergeReadConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if readRegion == "" {
		readRegion = cfg.Region
	}
	if readRole == "" {
		readRole = cfg.Role
	}
	if readPrefix == "" {
		readPrefix = cfg.EnvPrefix
	}
	if readFile == "" {
		readFile = cfg.File
	}
	if cfg.Upper != nil && !readUpper {
		readUpper = *cfg.Upper
	}
}

// ensureReadRegionIsSet ensures AWS region is set from flags, config, or environment
func ensureReadRegionIsSet() error {
	if readRegion == "" {
		readRegion = os.Getenv("AWS_REGION")
		if readRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
	}
	return nil
}

// getParameterValue retrieves a parameter value from SSM Parameter Store
func getParameterValue(paramName, paramRegion, defaultRegion string) (string, error) {
	region := paramRegion
	if region == "" {
		region = defaultRegion
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		return "", fmt.Errorf("AWS region must be specified via config, --region, or AWS_REGION environment variable")
	}

	ctx := context.Background()
	client, err := aws.NewClient(ctx, region, readRole)
	if err != nil {
		return "", fmt.Errorf("failed to create AWS client: %w", err)
	}

	value, err := client.GetParameter(ctx, paramName)
	if err != nil {
		if errors.Is(err, aws.ErrNotFound) {
			return "", fmt.Errorf("parameter '%s' not found in region '%s'", paramName, region)
		}
		return "", fmt.Errorf("failed to get parameter %s: %w", paramName, err)
	}

	return value, nil
}

// formatEnvName formats the environment variable name according to configuration
func formatEnvName(paramPath, envName string, cfg *config.Config) string {
	name := envName
	if name == "" {
		name = filepath.Base(paramPath)
	}

	prefix := readPrefix
	if prefix == "" && cfg != nil {
		prefix = cfg.EnvPrefix
	}
	if prefix != "" {
		name = prefix + "_" + name
	}

	if readUpper {
		name = strings.ToUpper(name)
	}

	return name
}

// writeOutput writes the parameter value(s) to a file or stdout
func writeOutput(output string, params []config.ParamConfig, cfg *config.Config) error {
	if readFile == "" && cfg != nil {
		readFile = cfg.File
	}

	if readFile != "" {
		// Ensure directory exists
		dir := filepath.Dir(readFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Print reading messages for each parameter
		for _, param := range params {
			fmt.Printf("Reading parameter '%s' from region '%s'\n", param.Name, readRegion)
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

func init() {
	readCmd.Flags().StringVar(&readPath, "path", "", "Parameter path (required if no parameters defined in config)")
	readCmd.Flags().StringVar(&readRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	readCmd.Flags().StringVar(&readRole, "role", "", "AWS role ARN to assume (optional)")
	readCmd.Flags().StringVar(&readFile, "file", "", "File to write to (optional)")
	readCmd.Flags().BoolVar(&readUpper, "upper", true, "Convert env var name to uppercase")
	readCmd.Flags().StringVar(&readPrefix, "env-prefix", "", "Prefix for env var name")
	readCmd.Flags().StringVar(&readEnvName, "env", "", "Environment variable name")
}
