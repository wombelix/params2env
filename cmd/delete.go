// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"git.sr.ht/~wombelix/params2env/internal/validation"
	"github.com/spf13/cobra"
)

// Command-line flags for the delete command
var (
	// deletePath is the full path of the parameter to delete
	deletePath string
	// deleteRegion is the AWS region where the parameter will be deleted
	deleteRegion string
	// deleteRole is the AWS IAM role to assume for the operation
	deleteRole string
	// deleteReplica is the region where the parameter replica should be deleted
	deleteReplica string
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a parameter from SSM Parameter Store",
	Long: `Delete a parameter from SSM Parameter Store.

The parameter will be deleted from the specified region and optionally from a replica region.
If the parameter doesn't exist, the command will fail with an appropriate error message.

Examples:
  # Delete a parameter from the default region
  params2env delete --path /myapp/config/url

  # Delete a parameter from a specific region
  params2env delete --path /myapp/config/url --region us-west-2

  # Delete a parameter and its replica
  params2env delete --path /myapp/config/url --replica us-west-2`,
	PreRunE: validateDeleteFlags,
	RunE:    runDelete,
}

// validateDeleteFlags checks if all required flags are set and valid
func validateDeleteFlags(cmd *cobra.Command, args []string) error {
	if deletePath == "" {
		return fmt.Errorf("required flag \"path\" not set")
	}
	if err := validation.ValidateParameterPath(deletePath); err != nil {
		return err
	}

	if deleteRegion != "" {
		if err := validation.ValidateRegion(deleteRegion); err != nil {
			return err
		}
	}

	if deleteReplica != "" {
		if err := validation.ValidateRegion(deleteReplica); err != nil {
			return fmt.Errorf("invalid replica region: %w", err)
		}
	}

	if deleteRole != "" {
		if err := validation.ValidateRoleARN(deleteRole); err != nil {
			return err
		}
	}

	return nil
}

// runDelete executes the delete command
func runDelete(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
	}

	// Merge config with flags (flags take precedence)
	mergeDeleteConfig(cfg)

	// Ensure region is set
	if err := ensureDeleteRegionIsSet(); err != nil {
		return err
	}

	// Delete parameter in primary region
	if err := deleteInPrimaryRegion(); err != nil {
		return err
	}

	// Handle replica if specified
	if deleteReplica != "" {
		if err := deleteInReplicaRegion(); err != nil {
			return err
		}
	}

	return nil
}

// mergeDeleteConfig merges configuration from file with command line flags
func mergeDeleteConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
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

// ensureDeleteRegionIsSet ensures AWS region is set from flags, config, or environment
func ensureDeleteRegionIsSet() error {
	if deleteRegion == "" {
		if deleteRegion = os.Getenv("AWS_REGION"); deleteRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
	}
	return nil
}

// deleteInPrimaryRegion deletes the parameter in the primary region
func deleteInPrimaryRegion() error {
	ctx := context.Background()
	client, err := aws.NewClient(ctx, deleteRegion, deleteRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	fmt.Printf("Deleting parameter '%s' in region '%s'...\n", deletePath, deleteRegion)
	if err := client.DeleteParameter(ctx, deletePath); err != nil {
		if errors.Is(err, aws.ErrNotFound) {
			return fmt.Errorf("parameter '%s' not found in region '%s'", deletePath, deleteRegion)
		}
		return fmt.Errorf("failed to delete parameter in region '%s': %w", deleteRegion, err)
	}

	fmt.Printf("Successfully deleted parameter '%s' in region '%s'\n", deletePath, deleteRegion)
	return nil
}

// deleteInReplicaRegion deletes the parameter in the replica region
func deleteInReplicaRegion() error {
	ctx := context.Background()
	replicaClient, err := aws.NewClient(ctx, deleteReplica, deleteRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client for replica region: %w", err)
	}

	fmt.Printf("Deleting parameter '%s' in replica region '%s'...\n", deletePath, deleteReplica)
	if err := replicaClient.DeleteParameter(ctx, deletePath); err != nil {
		if errors.Is(err, aws.ErrNotFound) {
			fmt.Printf("Warning: parameter '%s' not found in replica region '%s' (already deleted or never existed)\n", deletePath, deleteReplica)
			return nil
		}
		return fmt.Errorf("failed to delete parameter in replica region '%s': %w", deleteReplica, err)
	}

	fmt.Printf("Successfully deleted parameter '%s' in replica region '%s'\n", deletePath, deleteReplica)
	return nil
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
