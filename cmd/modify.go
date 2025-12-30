// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

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

func runModify(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	mergeModifyConfig(cfg)

	if err := ensureModifyRegionIsSet(); err != nil {
		return err
	}

	if err := validation.ValidateRegions(modifyRegion, modifyReplica); err != nil {
		return err
	}

	if err := modifyInPrimaryRegion(); err != nil {
		return err
	}

	if modifyReplica != "" {
		if err := modifyInReplicaRegion(); err != nil {
			return err
		}
	}

	return nil
}

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

func ensureModifyRegionIsSet() error {
	if modifyRegion == "" {
		if modifyRegion = os.Getenv("AWS_REGION"); modifyRegion == "" {
			return fmt.Errorf("AWS region must be specified via --region, config file, or AWS_REGION environment variable")
		}
		if err := validation.ValidateRegion(modifyRegion); err != nil {
			return fmt.Errorf("invalid AWS_REGION environment variable: %w", err)
		}
	}
	return nil
}

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
