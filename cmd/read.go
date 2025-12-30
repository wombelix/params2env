// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

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

func validateReadFlags(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if readPath == "" && (cfg == nil || len(cfg.Params) == 0) {
		return fmt.Errorf("required flag \"path\" not set")
	}

	if readPath != "" {
		if err := validation.ValidateParameterPath(readPath); err != nil {
			return err
		}
	}

	if err := validation.ValidateRegion(readRegion); err != nil {
		return err
	}

	if err := validation.ValidateRoleARN(readRole); err != nil {
		return err
	}

	return nil
}

func runRead(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if readPath == "" && cfg != nil && len(cfg.Params) > 0 {
		return handleConfigParameters(cfg)
	}

	return handleSingleParameter(cfg)
}

func handleConfigParameters(cfg *config.Config) error {
	var outputs []string
	for _, param := range cfg.Params {
		value, err := getParameterValue(param.Name, param.Region, cfg.Region)
		if err != nil {
			return err
		}

		name := formatEnvName(param.Name, param.Env, cfg)
		outputs = append(outputs, fmt.Sprintf("export %s=%q", name, value))
	}

	output := strings.Join(outputs, "\n") + "\n"
	return writeOutput(output, cfg.Params, cfg)
}

func handleSingleParameter(cfg *config.Config) error {
	mergeReadConfig(cfg)

	if err := ensureReadRegionIsSet(); err != nil {
		return err
	}

	value, err := getParameterValue(readPath, readRegion, "")
	if err != nil {
		return err
	}

	name := formatEnvName(readPath, readEnvName, cfg)
	output := fmt.Sprintf("export %s=%q\n", name, value)

	return writeOutput(output, []config.ParamConfig{{Name: readPath}}, cfg)
}

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

func ensureReadRegionIsSet() error {
	if readRegion == "" {
		if readRegion = os.Getenv("AWS_REGION"); readRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
		if err := validation.ValidateRegion(readRegion); err != nil {
			return fmt.Errorf("invalid AWS_REGION environment variable: %w", err)
		}
	}
	return nil
}

func getParameterValue(paramName, paramRegion, defaultRegion string) (string, error) {
	region := paramRegion
	if region == "" {
		region = defaultRegion
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region != "" {
			if err := validation.ValidateRegion(region); err != nil {
				return "", fmt.Errorf("invalid AWS_REGION environment variable: %w", err)
			}
		}
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
		if errors.Is(err, aws.ErrNoAccess) {
			return "", fmt.Errorf("access denied to parameter '%s' in region '%s': check IAM permissions", paramName, region)
		}
		if strings.Contains(err.Error(), "throttl") {
			return "", fmt.Errorf("request throttled for parameter '%s' in region '%s': try again later", paramName, region)
		}
		return "", fmt.Errorf("failed to get parameter '%s' from region '%s': %w", paramName, region, err)
	}

	return value, nil
}

func formatEnvName(paramPath, envName string, cfg *config.Config) string {
	name := envName
	if name == "" {
		name = filepath.Base(paramPath)
	}

	if readPrefix != "" {
		name = readPrefix + "_" + name
	} else if cfg != nil && cfg.EnvPrefix != "" {
		name = cfg.EnvPrefix + "_" + name
	}

	if readUpper {
		name = strings.ToUpper(name)
	}

	return name
}

// Writes to file or stdout. Uses secure permissions (0700 dirs, 0600 files).
func writeOutput(output string, params []config.ParamConfig, cfg *config.Config) error {
	if readFile == "" && cfg != nil {
		readFile = cfg.File
	}

	if readFile != "" {
		dir := filepath.Dir(readFile)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		for _, param := range params {
			fmt.Printf("Reading parameter '%s' from region '%s'\n", param.Name, readRegion)
		}

		if err := os.WriteFile(readFile, []byte(output), 0600); err != nil {
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
