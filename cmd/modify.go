// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/spf13/cobra"
)

var (
	modifyPath    string
	modifyValue   string
	modifyDesc    string
	modifyRegion  string
	modifyRole    string
	modifyReplica string
)

var modifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify an existing parameter in SSM Parameter Store",
	Long: `Modify an existing parameter in SSM Parameter Store.

The parameter will be updated with the specified value.
Optionally, you can update the description.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Check required flags
		if modifyPath == "" {
			return fmt.Errorf("required flag \"path\" not set")
		}
		if modifyValue == "" {
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

		// If region is still empty, try AWS_REGION env var
		if modifyRegion == "" {
			modifyRegion = os.Getenv("AWS_REGION")
			if modifyRegion == "" {
				return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
			}
		}

		// Create AWS client
		ctx := context.Background()
		client, err := aws.NewClient(ctx, modifyRegion, modifyRole)
		if err != nil {
			return fmt.Errorf("failed to create AWS client: %w", err)
		}

		// Modify parameter
		if err := client.ModifyParameter(ctx, modifyPath, modifyValue, modifyDesc); err != nil {
			return fmt.Errorf("failed to modify parameter: %w", err)
		}

		fmt.Printf("Successfully modified parameter '%s' in region '%s'\n", modifyPath, modifyRegion)

		// Handle replica if specified
		if modifyReplica != "" {
			replicaClient, err := aws.NewClient(ctx, modifyReplica, modifyRole)
			if err != nil {
				return fmt.Errorf("failed to create AWS client for replica region: %w", err)
			}

			if err := replicaClient.ModifyParameter(ctx, modifyPath, modifyValue, modifyDesc); err != nil {
				return fmt.Errorf("failed to modify parameter in replica region: %w", err)
			}

			fmt.Printf("Successfully modified parameter '%s' in replica region '%s'\n", modifyPath, modifyReplica)
		}

		return nil
	},
}

func init() {
	modifyCmd.Flags().StringVar(&modifyPath, "path", "", "Parameter path (required)")
	modifyCmd.Flags().StringVar(&modifyValue, "value", "", "Parameter value (required)")
	modifyCmd.Flags().StringVar(&modifyDesc, "description", "", "Parameter description")
	modifyCmd.Flags().StringVar(&modifyRegion, "region", "", "AWS region (optional, default: from AWS config or environment)")
	modifyCmd.Flags().StringVar(&modifyRole, "role", "", "AWS role ARN to assume (optional)")
	modifyCmd.Flags().StringVar(&modifyReplica, "replica", "", "Region to replicate the parameter to")
}
