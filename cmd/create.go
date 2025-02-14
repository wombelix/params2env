// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/spf13/cobra"
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
Optionally, you can provide a description and KMS key for SecureString parameters.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Check required flags
		if createPath == "" {
			return fmt.Errorf("required flag \"path\" not set")
		}
		if createValue == "" {
			return fmt.Errorf("required flag \"value\" not set")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}

		// Merge config with flags (flags take precedence)
		if cfg != nil {
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

		// Validate parameter type
		paramTypeStr := strings.TrimSpace(createType)
		if paramTypeStr != "String" && paramTypeStr != "SecureString" {
			return fmt.Errorf("invalid parameter type: %s (must be 'String' or 'SecureString')", paramTypeStr)
		}
		createType = paramTypeStr

		// If region is still empty, try AWS_REGION env var
		if createRegion == "" {
			createRegion = os.Getenv("AWS_REGION")
			if createRegion == "" {
				return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
			}
		}

		// Create AWS client
		ctx := context.Background()
		client, err := aws.NewClient(ctx, createRegion, createRole)
		if err != nil {
			return fmt.Errorf("failed to create AWS client: %w", err)
		}

		// Create parameter
		var kmsKeyID *string
		if createKMS != "" {
			kmsKeyID = &createKMS
		}

		if err := client.CreateParameter(ctx, createPath, createValue, createDesc, createType, kmsKeyID, createOverwrite); err != nil {
			return fmt.Errorf("failed to create parameter: %w", err)
		}

		fmt.Printf("Successfully created parameter '%s' in region '%s'\n", createPath, createRegion)

		// Handle replica if specified
		if createReplica != "" {
			replicaClient, err := aws.NewClient(ctx, createReplica, createRole)
			if err != nil {
				return fmt.Errorf("failed to create AWS client for replica region: %w", err)
			}

			// If using KMS key ARN, update it for replica region
			var replicaKMSKeyID *string
			if kmsKeyID != nil {
				// Extract account ID and key ID from the ARN
				arnParts := strings.Split(*kmsKeyID, ":")
				if len(arnParts) >= 6 && strings.HasPrefix(*kmsKeyID, "arn:aws:kms:") {
					accountID := arnParts[4]
					keyID := strings.TrimPrefix(arnParts[5], "key/")
					// Create new ARN for replica region
					replicaARN := fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", createReplica, accountID, keyID)
					replicaKMSKeyID = &replicaARN
				} else {
					// If not an ARN, use the key ID as is
					replicaKMSKeyID = kmsKeyID
				}
			}

			if err := replicaClient.CreateParameter(ctx, createPath, createValue, createDesc, createType, replicaKMSKeyID, createOverwrite); err != nil {
				return fmt.Errorf("failed to create parameter in replica region: %w", err)
			}

			fmt.Printf("Successfully created parameter '%s' in replica region '%s'\n", createPath, createReplica)
		}

		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
	createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
	createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type (String or SecureString)")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
	createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID for SecureString parameters")
	createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN to assume (optional)")
	createCmd.Flags().StringVar(&createReplica, "replica", "", "Region to replicate the parameter to")
	createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing parameter")
}
