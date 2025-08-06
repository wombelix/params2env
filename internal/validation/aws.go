// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package validation provides validation functions for AWS resource names and other inputs.
//
// It includes validation for:
// - SSM Parameter Store paths
// - AWS Region names
// - AWS KMS Key IDs and ARNs
// - AWS IAM Role ARNs
package validation

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Regular expressions for AWS resource validation
	parameterPathRegex = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+(/[a-zA-Z0-9_.-]+)*$`)
	regionRegex        = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	kmsKeyIDRegex      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	kmsAliasRegex      = regexp.MustCompile(`^alias/[a-zA-Z0-9/_-]+$`)
	kmsArnRegex        = regexp.MustCompile(`^arn:aws:kms:[a-z]{2}(-[a-z]+)+-\d:\d{12}:key/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	roleArnRegex       = regexp.MustCompile(`^arn:aws:iam::\d{12}:role/[a-zA-Z0-9+=,.@_-]+(/[a-zA-Z0-9+=,.@_-]+)*$`)
)

// ValidateParameterPath checks if the given SSM parameter path is valid.
// A valid path:
// - Must start with a forward slash
// - Can contain letters, numbers, dots, hyphens, and forward slashes
// - Must not be empty
// - Must not end with a forward slash
// - Must not contain consecutive forward slashes
func ValidateParameterPath(path string) error {
	if path == "" {
		return fmt.Errorf("parameter path cannot be empty")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("parameter path must start with '/'")
	}
	if strings.HasSuffix(path, "/") {
		return fmt.Errorf("parameter path must not end with '/'")
	}
	if strings.Contains(path, "//") {
		return fmt.Errorf("parameter path must not contain consecutive '/'")
	}
	if !parameterPathRegex.MatchString(path) {
		return fmt.Errorf("invalid parameter path format: %s", path)
	}
	return nil
}

// ValidateRegion checks if the given AWS region name is valid.
// A valid region name:
// - Must be in the format: [a-z]{2}-[a-z]+-\d
// - Examples: us-east-1, eu-central-1, ap-southeast-2
// - Empty string is considered valid (for optional fields)
func ValidateRegion(region string) error {
	if region == "" {
		return nil
	}
	if !regionRegex.MatchString(region) {
		return fmt.Errorf("invalid region format: %s", region)
	}
	return nil
}

// ValidateKMSKey checks if the given KMS key identifier is valid.
// It accepts:
// - Key ID (UUID format)
// - Key alias (alias/name format)
// - Key ARN (full ARN format)
// - Empty string is considered valid (for optional fields)
func ValidateKMSKey(key string) error {
	if key == "" {
		return nil
	}

	// Check if it matches any valid KMS key format
	if kmsKeyIDRegex.MatchString(key) || kmsAliasRegex.MatchString(key) || kmsArnRegex.MatchString(key) {
		return nil
	}

	return fmt.Errorf("invalid KMS key format: %s", key)
}

// ValidateRoleARN checks if the given IAM role ARN is valid.
// A valid role ARN:
// - Must be in the format: arn:aws:iam::<account-id>:role/<role-name-with-path>
// - Account ID must be 12 digits
// - Role name must follow IAM naming rules
// - Empty string is considered valid (for optional fields)
func ValidateRoleARN(arn string) error {
	if arn == "" {
		return nil
	}
	if !roleArnRegex.MatchString(arn) {
		return fmt.Errorf("invalid role ARN format: %s", arn)
	}
	return nil
}

// ValidateRegions ensures replica region differs from primary region.
// This prevents unnecessary duplicate operations and potential confusion.
func ValidateRegions(primary, replica string) error {
	if replica != "" && primary == replica {
		return fmt.Errorf("replica region '%s' cannot be the same as primary region '%s'", replica, primary)
	}
	return nil
}

// ValidateSecureStringRequirements ensures KMS key is provided for SecureString parameters.
// This prevents accidental use of AWS managed keys when custom encryption is expected.
func ValidateSecureStringRequirements(paramType, kmsKey string) error {
	if paramType == "SecureString" && kmsKey == "" {
		return fmt.Errorf("KMS key is required for SecureString parameters")
	}
	return nil
}
