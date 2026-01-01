// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"git.sr.ht/~wombelix/params2env/internal/validation"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	createPath      string
	createValue     string
	createType      string
	createDesc      string
	createKMS       string
	createRegion    string
	createRole      string
	createReplica   string
	createOverwrite bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new parameter in SSM Parameter Store",
	Long: `Create a new parameter in SSM Parameter Store.

The parameter will be created with the specified value and type.
Value can be provided via --value flag, piped stdin, or interactive prompt.
SecureString prompts use hidden input; String prompts are visible.

Examples:
  # Create a String parameter
  params2env create --path /myapp/config/url --value https://example.com

  # Create a SecureString parameter (prompts for value)
  params2env create --path /myapp/secrets/api-key --type SecureString --kms alias/mykey

  # Pipe value from another command
  echo "mysecret" | params2env create --path /myapp/secrets/token --type SecureString --kms alias/mykey

  # Create and replicate to another region
  params2env create --path /myapp/config/shared --value myvalue --replica us-west-2`,
	PreRunE: validateCreateFlags,
	RunE:    runCreate,
}

func validateCreateFlags(cmd *cobra.Command, args []string) error {
	if createPath == "" {
		return fmt.Errorf("required flag \"path\" not set")
	}
	if err := validation.ValidateParameterPath(createPath); err != nil {
		return err
	}

	if err := validation.ValidateRegion(createRegion); err != nil {
		return err
	}

	if err := validation.ValidateRegion(createReplica); err != nil {
		return fmt.Errorf("invalid replica region: %w", err)
	}

	if err := validation.ValidateRoleARN(createRole); err != nil {
		return err
	}

	if err := validation.ValidateKMSKey(createKMS); err != nil {
		return err
	}

	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	mergeCreateConfig(cfg)

	if err := validateParameterType(); err != nil {
		return err
	}

	if err := ensureRegionIsSet(); err != nil {
		return err
	}

	if err := validation.ValidateRegions(createRegion, createReplica); err != nil {
		return err
	}

	if err := validation.ValidateSecureStringRequirements(createType, createKMS); err != nil {
		return err
	}

	valueProvided := cmd.Flags().Changed("value")
	if valueProvided && createValue == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if !valueProvided {
		value, err := readValueInteractive(createType)
		if err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
		createValue = value
		if createValue == "" {
			return fmt.Errorf("value cannot be empty")
		}
	}

	if err := createInPrimaryRegion(); err != nil {
		return err
	}

	if createReplica != "" {
		if err := createInReplicaRegion(); err != nil {
			return err
		}
	}

	return nil
}

func readValueInteractive(paramType string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return readFromStdin()
	}

	if paramType == aws.ParameterTypeSecureString {
		fmt.Fprint(os.Stderr, "Enter parameter value: ")
		value, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("failed to read value: %w", err)
		}
		return string(value), nil
	}

	fmt.Fprint(os.Stderr, "Enter parameter value: ")
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read value: %w", err)
	}
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	return value, nil
}

func readFromStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read from stdin: %w", err)
	}
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	return value, nil
}

func mergeCreateConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if createRegion == "" {
		createRegion = cfg.Region
	}
	if createReplica == "" {
		createReplica = cfg.Replica
	}
	if createRole == "" {
		createRole = cfg.Role
	}
	if createKMS == "" && cfg.KMS != "" {
		createKMS = cfg.KMS
	}
}

func validateParameterType() error {
	paramTypeStr := strings.TrimSpace(createType)
	if paramTypeStr != aws.ParameterTypeString && paramTypeStr != aws.ParameterTypeSecureString {
		return fmt.Errorf("invalid parameter type: %s (must be '%s' or '%s')",
			paramTypeStr, aws.ParameterTypeString, aws.ParameterTypeSecureString)
	}
	createType = paramTypeStr
	return nil
}

func ensureRegionIsSet() error {
	if createRegion == "" {
		if createRegion = os.Getenv("AWS_REGION"); createRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
		if err := validation.ValidateRegion(createRegion); err != nil {
			return fmt.Errorf("invalid AWS_REGION environment variable: %w", err)
		}
	}
	return nil
}

func createInPrimaryRegion() error {
	ctx := context.Background()
	client, err := aws.NewClient(ctx, createRegion, createRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	var kmsKeyID *string
	if createKMS != "" {
		kmsKeyID = &createKMS
	}

	if err := client.CreateParameter(ctx, createPath, createValue, createDesc, createType, kmsKeyID, createOverwrite); err != nil {
		return fmt.Errorf("failed to create parameter: %w", err)
	}

	fmt.Printf("Successfully created parameter '%s' in region '%s'\n", createPath, createRegion)
	return nil
}

func createInReplicaRegion() error {
	ctx := context.Background()
	replicaClient, err := aws.NewClient(ctx, createReplica, createRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client for replica region: %w", err)
	}

	var replicaKMSKeyID *string
	if createKMS != "" {
		var err error
		replicaKMSKeyID, err = getReplicaKMSKeyID(createKMS, createReplica)
		if err != nil {
			return fmt.Errorf("failed to process KMS key for replica region: %w", err)
		}
	}

	if err := replicaClient.CreateParameter(ctx, createPath, createValue, createDesc, createType, replicaKMSKeyID, createOverwrite); err != nil {
		if replicaKMSKeyID != nil && strings.Contains(err.Error(), "KMS") {
			return fmt.Errorf("failed to create parameter in replica region: %w\nHint: if using a single-region KMS key, create a key in the replica region and run separate create commands for each region", err)
		}
		return fmt.Errorf("failed to create parameter in replica region: %w", err)
	}

	fmt.Printf("Successfully created parameter '%s' in replica region '%s'\n", createPath, createReplica)
	return nil
}

// For aliases or key IDs, returns input unchanged. For ARNs, builds a new ARN for the replica region.
func getReplicaKMSKeyID(kmsKeyID, replicaRegion string) (*string, error) {
	if !strings.HasPrefix(kmsKeyID, "arn:") {
		return &kmsKeyID, nil
	}

	arnParts := strings.Split(kmsKeyID, ":")
	if len(arnParts) != 6 {
		return nil, fmt.Errorf("invalid KMS ARN format: %s", kmsKeyID)
	}

	if arnParts[0] != "arn" || arnParts[1] != "aws" || arnParts[2] != "kms" {
		return nil, fmt.Errorf("invalid KMS ARN format: %s", kmsKeyID)
	}

	accountID := arnParts[4]
	if accountID == "" {
		return nil, fmt.Errorf("missing account ID in KMS ARN: %s", kmsKeyID)
	}

	keyPart := arnParts[5]
	if !strings.HasPrefix(keyPart, "key/") {
		return nil, fmt.Errorf("invalid key format in KMS ARN: %s", kmsKeyID)
	}

	keyID := strings.TrimPrefix(keyPart, "key/")
	if keyID == "" {
		return nil, fmt.Errorf("missing key ID in KMS ARN: %s", kmsKeyID)
	}

	replicaARN := fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", replicaRegion, accountID, keyID)
	return &replicaARN, nil
}

func init() {
	createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
	createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (optional, can be provided via stdin or interactive prompt)")
	createCmd.Flags().StringVar(&createType, "type", aws.ParameterTypeString, "Parameter type (String or SecureString)")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
	createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID for SecureString parameters")
	createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN to assume (optional)")
	createCmd.Flags().StringVar(&createReplica, "replica", "", "Region to replicate the parameter to")
	createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing parameter")
}
