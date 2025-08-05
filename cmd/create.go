// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package cmd implements the command-line interface for params2env.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"git.sr.ht/~wombelix/params2env/internal/validation"
	"github.com/spf13/cobra"
)

// Command-line flags for the create command
var (
	// createPath is the full path of the parameter to create
	createPath string
	// createValue is the value to assign to the parameter
	createValue string
	// createType is the parameter type (String or SecureString)
	createType string
	// createDesc is an optional description for the parameter
	createDesc string
	// createKMS is the KMS key ID for SecureString parameters
	createKMS string
	// createRegion is the AWS region where the parameter will be created
	createRegion string
	// createRole is the AWS IAM role to assume for the operation
	createRole string
	// createReplica is the region where the parameter should be replicated
	createReplica string
	// createOverwrite determines if an existing parameter should be overwritten
	createOverwrite bool
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new parameter in SSM Parameter Store",
	Long: `Create a new parameter in SSM Parameter Store.

The parameter will be created with the specified value and type.
Optionally, you can provide a description and KMS key for SecureString parameters.

Examples:
  # Create a String parameter
  params2env create --path /myapp/config/url --value https://example.com --type String

  # Create a SecureString parameter with KMS key
  params2env create --path /myapp/secrets/api-key --value mysecret --type SecureString --kms alias/mykey

  # Create a parameter and replicate it to another region
  params2env create --path /myapp/config/shared --value myvalue --replica us-west-2`,
	PreRunE: validateCreateFlags,
	RunE:    runCreate,
}

// validateCreateFlags checks if all required flags are set and valid
func validateCreateFlags(cmd *cobra.Command, args []string) error {
	if createPath == "" {
		return fmt.Errorf("required flag \"path\" not set")
	}
	if err := validation.ValidateParameterPath(createPath); err != nil {
		return err
	}

	if createValue == "" {
		return fmt.Errorf("required flag \"value\" not set")
	}

	if createRegion != "" {
		if err := validation.ValidateRegion(createRegion); err != nil {
			return err
		}
	}

	if createReplica != "" {
		if err := validation.ValidateRegion(createReplica); err != nil {
			return fmt.Errorf("invalid replica region: %w", err)
		}
	}

	if createRole != "" {
		if err := validation.ValidateRoleARN(createRole); err != nil {
			return err
		}
	}

	if createKMS != "" {
		if err := validation.ValidateKMSKey(createKMS); err != nil {
			return err
		}
	}

	return nil
}

// runCreate executes the create command
func runCreate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Merge config with flags (flags take precedence)
	mergeCreateConfig(cfg)

	// Validate parameter type
	if err := validateParameterType(); err != nil {
		return err
	}

	// Ensure region is set
	if err := ensureRegionIsSet(); err != nil {
		return err
	}

	// Create parameter in primary region
	if err := createInPrimaryRegion(); err != nil {
		return err
	}

	// Handle replication if specified
	if createReplica != "" {
		if err := createInReplicaRegion(); err != nil {
			return err
		}
	}

	return nil
}

// mergeCreateConfig merges configuration from file with command line flags
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

// validateParameterType ensures the parameter type is valid
func validateParameterType() error {
	paramTypeStr := strings.TrimSpace(createType)
	if paramTypeStr != aws.ParameterTypeString && paramTypeStr != aws.ParameterTypeSecureString {
		return fmt.Errorf("invalid parameter type: %s (must be '%s' or '%s')",
			paramTypeStr, aws.ParameterTypeString, aws.ParameterTypeSecureString)
	}
	createType = paramTypeStr
	return nil
}

// ensureRegionIsSet ensures AWS region is set from flags, config, or environment
func ensureRegionIsSet() error {
	if createRegion == "" {
		if createRegion = os.Getenv("AWS_REGION"); createRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
	}
	return nil
}

// createInPrimaryRegion creates the parameter in the primary region
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

// createInReplicaRegion creates the parameter in the replica region
func createInReplicaRegion() error {
	ctx := context.Background()
	replicaClient, err := aws.NewClient(ctx, createReplica, createRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client for replica region: %w", err)
	}

	var replicaKMSKeyID *string
	if createKMS != "" {
		replicaKMSKeyID = getReplicaKMSKeyID(createKMS, createReplica)
	}

	if err := replicaClient.CreateParameter(ctx, createPath, createValue, createDesc, createType, replicaKMSKeyID, createOverwrite); err != nil {
		return fmt.Errorf("failed to create parameter in replica region: %w", err)
	}

	fmt.Printf("Successfully created parameter '%s' in replica region '%s'\n", createPath, createReplica)
	return nil
}

// getReplicaKMSKeyID returns the KMS key ID for the replica region
func getReplicaKMSKeyID(kmsKeyID, replicaRegion string) *string {
	const arnPrefix = "arn:aws:kms:"
	const minARNParts = 6

	// If not an ARN, use the key ID as is
	if !strings.HasPrefix(kmsKeyID, arnPrefix) {
		return &kmsKeyID
	}

	// Extract account ID and key ID from the ARN
	arnParts := strings.Split(kmsKeyID, ":")
	if len(arnParts) < minARNParts {
		return &kmsKeyID
	}

	accountID := arnParts[4]
	keyID := arnParts[5]
	replicaARN := fmt.Sprintf("%s%s:%s:%s", arnPrefix, replicaRegion, accountID, keyID)
	return &replicaARN
}

func init() {
	createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
	createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
	createCmd.Flags().StringVar(&createType, "type", aws.ParameterTypeString, "Parameter type (String or SecureString)")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
	createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID for SecureString parameters")
	createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN to assume (optional)")
	createCmd.Flags().StringVar(&createReplica, "replica", "", "Region to replicate the parameter to")
	createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing parameter")
}
