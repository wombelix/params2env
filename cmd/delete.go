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
	deletePath    string
	deleteRegion  string
	deleteRole    string
	deleteReplica string
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a parameter from SSM Parameter Store",
	Long: `Delete a parameter from SSM Parameter Store.

The parameter will be deleted from the specified region and optionally from a replica region.
If the parameter doesn't exist, the command will fail with an appropriate error message.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Check required flags
		if deletePath == "" {
			return fmt.Errorf("required flag \"path\" not set")
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
			if deleteRegion == "" {
				deleteRegion = cfg.Region
			}
			if deleteReplica == "" {
				deleteReplica = cfg.Replica
			}
			if deleteRole == "" {
				deleteRole = cfg.Role
			}
		}

		// If region is still empty, try AWS_REGION env var
		if deleteRegion == "" {
			deleteRegion = os.Getenv("AWS_REGION")
			if deleteRegion == "" {
				return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
			}
		}

		// Create AWS client
		ctx := context.Background()
		client, err := aws.NewClient(ctx, deleteRegion, deleteRole)
		if err != nil {
			return fmt.Errorf("failed to create AWS client: %w", err)
		}

		// Delete parameter in primary region
		fmt.Printf("Deleting parameter '%s' in region '%s'...\n", deletePath, deleteRegion)
		if err := client.DeleteParameter(ctx, deletePath); err != nil {
			if strings.Contains(err.Error(), "ParameterNotFound") {
				return fmt.Errorf("parameter '%s' not found in region '%s'", deletePath, deleteRegion)
			}
			return fmt.Errorf("failed to delete parameter in region '%s': %w", deleteRegion, err)
		}
		fmt.Printf("Successfully deleted parameter '%s' in region '%s'\n", deletePath, deleteRegion)

		// Handle replica if specified
		if deleteReplica != "" {
			fmt.Printf("Deleting parameter '%s' in replica region '%s'...\n", deletePath, deleteReplica)
			replicaClient, err := aws.NewClient(ctx, deleteReplica, deleteRole)
			if err != nil {
				return fmt.Errorf("failed to create AWS client for replica region: %w", err)
			}

			if err := replicaClient.DeleteParameter(ctx, deletePath); err != nil {
				if strings.Contains(err.Error(), "ParameterNotFound") {
					fmt.Printf("Warning: Parameter '%s' not found in replica region '%s'\n", deletePath, deleteReplica)
					return nil
				}
				return fmt.Errorf("failed to delete parameter in replica region '%s': %w", deleteReplica, err)
			}
			fmt.Printf("Successfully deleted parameter '%s' in replica region '%s'\n", deletePath, deleteReplica)
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().StringVar(&deletePath, "path", "", "Parameter path (required)")
	deleteCmd.Flags().StringVar(&deleteRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	deleteCmd.Flags().StringVar(&deleteRole, "role", "", "AWS role ARN to assume (optional)")
	deleteCmd.Flags().StringVar(&deleteReplica, "replica", "", "Region to delete the replica from")
	if err := deleteCmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}
}
