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

// Command-line flags for the modify command
var (
	// modifyPath is the full path of the parameter to modify
	modifyPath string
	// modifyValue is the new value to assign to the parameter
	modifyValue string
	// modifyDesc is the new description for the parameter
	modifyDesc string
	// modifyRegion is the AWS region where the parameter will be modified
	modifyRegion string
	// modifyRole is the AWS IAM role to assume for the operation
	modifyRole string
	// modifyReplica is the region where the parameter replica should be modified
	modifyReplica string
)

// modifyCmd represents the modify command
var modifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify an existing parameter in SSM Parameter Store",
	Long: `Modify an existing parameter in SSM Parameter Store.

The parameter will be updated with the specified value.
Optionally, you can update the description.

Examples:
  # Modify a parameter's value
  params2env modify --path /myapp/config/url --value https://newexample.com

  # Modify a parameter's value and description
  params2env modify --path /myapp/config/url --value https://newexample.com --description "Updated URL"

  # Modify a parameter and its replica
  params2env modify --path /myapp/config/url --value https://newexample.com --replica us-west-2`,
	PreRunE: validateModifyFlags,
	RunE:    runModify,
}

// validateModifyFlags checks if all required flags are set and valid
func validateModifyFlags(cmd *cobra.Command, args []string) error {
	if modifyPath == "" {
		return fmt.Errorf("required flag \"path\" not set")
	}
	if err := validation.ValidateParameterPath(modifyPath); err != nil {
		return err
	}

	if modifyValue == "" {
		return fmt.Errorf("required flag \"value\" not set")
	}

	if err := validation.ValidateRegion(modifyRegion); err != nil {
		return err
	}

	if err := validation.ValidateRegion(modifyReplica); err != nil {
		return fmt.Errorf("invalid replica region: %w", err)
	}

	if err := validation.ValidateRoleARN(modifyRole); err != nil {
		return err
	}

	return nil
}

// runModify executes the modify command
func runModify(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Merge config with flags (flags take precedence)
	mergeModifyConfig(cfg)

	// Ensure region is set
	if err := ensureModifyRegionIsSet(); err != nil {
		return err
	}

	// Validate regions are different
	if err := validation.ValidateRegions(modifyRegion, modifyReplica); err != nil {
		return err
	}

	// Modify parameter in primary region
	if err := modifyInPrimaryRegion(); err != nil {
		return err
	}

	// Handle replica if specified
	if modifyReplica != "" {
		if err := modifyInReplicaRegion(); err != nil {
			return err
		}
	}

	return nil
}

// mergeModifyConfig merges configuration from file with command line flags
func mergeModifyConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if modifyRegion == "" {
		modifyRegion = cfg.Region
	}
	if modifyReplica == "" {
		modifyReplica = cfg.Replica
	}
	if modifyRole == "" {
		modifyRole = cfg.Role
	}
}

// ensureModifyRegionIsSet ensures AWS region is set from flags, config, or environment
func ensureModifyRegionIsSet() error {
	if modifyRegion == "" {
		if modifyRegion = os.Getenv("AWS_REGION"); modifyRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
	}
	return nil
}

// modifyInPrimaryRegion modifies the parameter in the primary region
func modifyInPrimaryRegion() error {
	ctx := context.Background()
	client, err := aws.NewClient(ctx, modifyRegion, modifyRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	if err := client.ModifyParameter(ctx, modifyPath, modifyValue, modifyDesc); err != nil {
		if errors.Is(err, aws.ErrNotFound) {
			return fmt.Errorf("parameter '%s' not found in region '%s'", modifyPath, modifyRegion)
		}
		return fmt.Errorf("failed to modify parameter: %w", err)
	}

	fmt.Printf("Successfully modified parameter '%s' in region '%s'\n", modifyPath, modifyRegion)
	return nil
}

// modifyInReplicaRegion modifies the parameter in the replica region
func modifyInReplicaRegion() error {
	ctx := context.Background()
	replicaClient, err := aws.NewClient(ctx, modifyReplica, modifyRole)
	if err != nil {
		return fmt.Errorf("failed to create AWS client for replica region: %w", err)
	}

	if err := replicaClient.ModifyParameter(ctx, modifyPath, modifyValue, modifyDesc); err != nil {
		if errors.Is(err, aws.ErrNotFound) {
			return fmt.Errorf("parameter '%s' not found in replica region '%s'", modifyPath, modifyReplica)
		}
		return fmt.Errorf("failed to modify parameter in replica region: %w", err)
	}

	fmt.Printf("Successfully modified parameter '%s' in replica region '%s'\n", modifyPath, modifyReplica)
	return nil
}

func init() {
	modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
	modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value (required)")
	modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description")
	modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN to assume (optional)")
	modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Region to replicate the parameter to")
	if err := modifyCmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}
	if err := modifyCmd.MarkFlagRequired("value"); err != nil {
		panic(err)
	}
}
