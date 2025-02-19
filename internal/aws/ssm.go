// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package aws provides AWS service interactions for the params2env tool.
//
// It implements a clean interface for AWS Systems Manager Parameter Store operations,
// supporting parameter creation, retrieval, modification, and deletion. The package
// handles AWS authentication, including role assumption, and provides proper error
// handling and context support.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := aws.NewClient(ctx, "us-west-2", "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	value, err := client.GetParameter(ctx, "/my/parameter")
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// Common errors returned by the package
var (
	ErrEmptyRegion = errors.New("region is required")
)

// SSMAPI defines the interface for AWS SSM operations.
// This interface allows for easy mocking in tests and flexibility
// in implementation.
type SSMAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
}

// Client represents an AWS SSM client with the necessary API operations.
// It wraps the AWS SDK's SSM client and provides a simpler interface
// for parameter store operations.
type Client struct {
	SSMClient SSMAPI
}

// NewClientFunc is the type for the client creation function.
// This allows for dependency injection and easier testing.
type NewClientFunc func(context.Context, string, string) (*Client, error)

// DefaultNewClient is the default implementation of NewClientFunc.
// It creates a new AWS SSM client with the specified region and optional role.
// If role is provided, it will use AWS STS to assume the role before creating the client.
var DefaultNewClient NewClientFunc = func(ctx context.Context, region, role string) (*Client, error) {
	if region == "" {
		return nil, ErrEmptyRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if role != "" {
		// Create an STS client to assume the role
		stsClient := sts.NewFromConfig(cfg)
		provider := stscreds.NewAssumeRoleProvider(stsClient, role)
		cfg.Credentials = aws.NewCredentialsCache(provider)
	}

	return &Client{
		SSMClient: ssm.NewFromConfig(cfg),
	}, nil
}

// NewClient is the function used to create new AWS SSM clients.
// By default, it points to DefaultNewClient but can be overridden for testing.
var NewClient = DefaultNewClient

// GetParameter retrieves a parameter from SSM Parameter Store.
// It automatically handles decryption for SecureString parameters.
//
// Parameters:
//   - ctx: Context for the AWS API call
//   - name: The full path of the parameter to retrieve
//
// Returns:
//   - The parameter value as a string
//   - An error if the parameter doesn't exist or cannot be retrieved
func (c *Client) GetParameter(ctx context.Context, name string) (string, error) {
	withDecryption := true
	input := &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: &withDecryption,
	}

	output, err := c.SSMClient.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get parameter %s: %w", name, err)
	}

	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", fmt.Errorf("parameter %s has no value", name)
	}

	return *output.Parameter.Value, nil
}

// CreateParameter creates a new parameter in SSM Parameter Store.
//
// Parameters:
//   - ctx: Context for the AWS API call
//   - name: The full path of the parameter to create
//   - value: The parameter value
//   - description: Optional description of the parameter
//   - paramType: Parameter type (String or SecureString)
//   - kmsKeyID: Optional KMS key ID for SecureString parameters
//   - overwrite: Whether to overwrite an existing parameter
//
// Returns an error if the parameter cannot be created or if validation fails.
func (c *Client) CreateParameter(ctx context.Context, name, value, description string, paramType string, kmsKeyID *string, overwrite bool) error {
	input := &ssm.PutParameterInput{
		Name:        &name,
		Value:       &value,
		Type:        ssmtypes.ParameterType(paramType),
		Overwrite:   &overwrite,
		Description: &description,
	}

	if kmsKeyID != nil {
		input.KeyId = kmsKeyID
	}

	_, err := c.SSMClient.PutParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create parameter %s: %w", name, err)
	}

	return nil
}

// ModifyParameter updates an existing parameter in SSM Parameter Store.
//
// Parameters:
//   - ctx: Context for the AWS API call
//   - name: The full path of the parameter to modify
//   - value: The new parameter value
//   - description: Optional new description (empty string to keep existing)
//
// Returns an error if the parameter cannot be modified or doesn't exist.
func (c *Client) ModifyParameter(ctx context.Context, name, value, description string) error {
	overwrite := true
	input := &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Overwrite: &overwrite,
	}

	if description != "" {
		input.Description = &description
	}

	_, err := c.SSMClient.PutParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to modify parameter %s: %w", name, err)
	}

	return nil
}

// DeleteParameter deletes a parameter from SSM Parameter Store.
//
// Parameters:
//   - ctx: Context for the AWS API call
//   - name: The full path of the parameter to delete
//
// Returns an error if the parameter cannot be deleted, doesn't exist,
// or if there are insufficient permissions.
func (c *Client) DeleteParameter(ctx context.Context, name string) error {
	input := &ssm.DeleteParameterInput{
		Name: &name,
	}

	_, err := c.SSMClient.DeleteParameter(ctx, input)
	if err != nil {
		var pnf *ssmtypes.ParameterNotFound
		if errors.As(err, &pnf) {
			return fmt.Errorf("parameter %s not found", name)
		}
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "AccessDeniedException" {
				return fmt.Errorf("insufficient permissions to delete parameter %s", name)
			}
		}
		return fmt.Errorf("failed to delete parameter %s: %w", name, err)
	}

	return nil
}
