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
	"strconv"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"git.sr.ht/~wombelix/params2env/internal/validation"
	"github.com/spf13/cobra"
)

var (
	readPath              string
	readRegion            string
	readRole              string
	readFile              string
	readUpper             bool
	readPrefix            string
	readEnvName           string
	readFormat            string
	readFormatExplicitSet bool
)

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read a parameter from SSM Parameter Store",
	Long: `Read a parameter from SSM Parameter Store.

The parameter value will be printed to stdout in the format:
export PARAM="value"

Output formats:
  env (default): export KEY="value" - for shell sourcing
  github-env:    KEY=value - for GitHub Actions with automatic masking

Examples:
  # Read a single parameter
  params2env read --path /myapp/config/url

  # Read a parameter and write to a file
  params2env read --path /myapp/config/url --file /etc/env.d/myapp

  # GitHub Actions format (auto-detects $GITHUB_ENV)
  params2env read --path /myapp/secret --format github-env

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

	if readFormat != "env" && readFormat != "github-env" {
		return fmt.Errorf("invalid format %q (must be 'env' or 'github-env')", readFormat)
	}

	readFormatExplicitSet = cmd.Flags().Changed("format")

	effectiveFormat := readFormat
	if !readFormatExplicitSet && cfg != nil && cfg.Format != "" {
		effectiveFormat = cfg.Format
	}

	effectiveFile := readFile
	if readFile == "" && cfg != nil && cfg.File != "" {
		effectiveFile = cfg.File
	}

	if effectiveFormat == "github-env" && effectiveFile == "" && os.Getenv("GITHUB_ENV") == "" {
		return fmt.Errorf("github-env format requires --file or GITHUB_ENV environment variable")
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
	if !readFormatExplicitSet && cfg.Format != "" {
		readFormat = cfg.Format
	}

	var outputs []string
	for _, param := range cfg.Params {
		value, err := getParameterValue(param.Name, param.Region, cfg.Region)
		if err != nil {
			return err
		}

		// https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions#masking-a-value-in-a-log
		if readFormat == "github-env" {
			fmt.Printf("::add-mask::%s\n", value)
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

	// https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions#masking-a-value-in-a-log
	if readFormat == "github-env" {
		fmt.Printf("::add-mask::%s\n", value)
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
	if !readFormatExplicitSet && cfg.Format != "" {
		readFormat = cfg.Format
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

func writeOutput(output string, params []config.ParamConfig, cfg *config.Config) error {
	if readFormat == "github-env" {
		return writeGithubEnvOutput(output, params, cfg)
	}
	return writeEnvOutput(output, params, cfg)
}

func writeGithubEnvOutput(output string, params []config.ParamConfig, cfg *config.Config) error {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var fileContent []string

	for _, line := range lines {
		if strings.HasPrefix(line, "export ") {
			envLine := strings.TrimPrefix(line, "export ")
			if parts := strings.SplitN(envLine, "=", 2); len(parts) == 2 {
				key := parts[0]
				// The value is Go-quoted (from %q), so unquote it to get the original value
				// with actual newlines instead of escaped \n sequences
				value, err := strconv.Unquote(parts[1])
				if err != nil {
					// Fallback: just trim quotes if unquoting fails
					value = strings.Trim(parts[1], "\"'")
				}
				if strings.Contains(value, "\n") {
					// Multi-line value: use official GitHub EOF syntax
					fileContent = append(fileContent,
						fmt.Sprintf("%s<<EOF\n%s\nEOF", key, value),
					)
				} else {
					// Single-line value: normal KEY=value
					fileContent = append(fileContent,
						fmt.Sprintf("%s=%s", key, value),
					)
				}
			}
		}
	}

	outputFile := readFile
	if outputFile == "" {
		outputFile = os.Getenv("GITHUB_ENV")
	}

	if outputFile != "" {
		return appendToFile(outputFile, strings.Join(fileContent, "\n")+"\n")
	}
	return nil
}

func writeEnvOutput(output string, params []config.ParamConfig, cfg *config.Config) error {
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

		return appendToFile(readFile, output)
	}

	fmt.Print(output)
	return nil
}

func appendToFile(filename, content string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
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
	readCmd.Flags().StringVar(&readFormat, "format", "env", "Output format: 'env' or 'github-env'")
}
