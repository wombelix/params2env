// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

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

// SSMAPI defines the interface for AWS SSM operations
type SSMAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
}

// Client represents an AWS SSM client
type Client struct {
	SSMClient SSMAPI
}

// NewClientFunc is the type for the client creation function
type NewClientFunc func(context.Context, string, string) (*Client, error)

// DefaultNewClient is the default implementation of NewClientFunc
var DefaultNewClient NewClientFunc = func(ctx context.Context, region, role string) (*Client, error) {
	if region == "" {
		return nil, fmt.Errorf("region is required")
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

// NewClient is the function used to create new AWS SSM clients
var NewClient = DefaultNewClient

// GetParameter retrieves a parameter from SSM Parameter Store
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

// CreateParameter creates a new parameter in SSM Parameter Store
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

// ModifyParameter updates an existing parameter in SSM Parameter Store
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

// DeleteParameter deletes a parameter from SSM Parameter Store
func (c *Client) DeleteParameter(ctx context.Context, name string) error {
	input := &ssm.DeleteParameterInput{
		Name: &name,
	}

	_, err := c.SSMClient.DeleteParameter(ctx, input)
	if err != nil {
		var pnf *ssmtypes.ParameterNotFound
		if errors.As(err, &pnf) {
			return fmt.Errorf("ParameterNotFound: parameter %s does not exist", name)
		}
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "AccessDeniedException" {
				return fmt.Errorf("AccessDenied: insufficient permissions to delete parameter %s", name)
			}
		}
		return fmt.Errorf("failed to delete parameter %s: %w", name, err)
	}

	return nil
}
