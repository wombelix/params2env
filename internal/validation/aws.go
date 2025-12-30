// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

// Package validation checks AWS resource names and inputs.
package validation

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	parameterPathRegex = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+(/[a-zA-Z0-9_.-]+)*$`)
	regionRegex        = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	kmsKeyIDRegex      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	kmsAliasRegex      = regexp.MustCompile(`^alias/[a-zA-Z0-9/_-]+$`)
	kmsArnRegex        = regexp.MustCompile(`^arn:aws:kms:[a-z]{2}(-[a-z]+)+-\d:\d{12}:key/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	roleArnRegex       = regexp.MustCompile(`^arn:aws:iam::\d{12}:role/[a-zA-Z0-9+=,.@_-]+(/[a-zA-Z0-9+=,.@_-]+)*$`)
)

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

// Empty is ok (optional fields).
func ValidateRegion(region string) error {
	if region == "" {
		return nil
	}
	if !regionRegex.MatchString(region) {
		return fmt.Errorf("invalid region format: %s", region)
	}
	return nil
}

// Accepts key ID, alias, or ARN. Empty is ok.
func ValidateKMSKey(key string) error {
	if key == "" {
		return nil
	}

	if kmsKeyIDRegex.MatchString(key) || kmsAliasRegex.MatchString(key) || kmsArnRegex.MatchString(key) {
		return nil
	}

	return fmt.Errorf("invalid KMS key format: %s", key)
}

// Empty is ok.
func ValidateRoleARN(arn string) error {
	if arn == "" {
		return nil
	}
	if !roleArnRegex.MatchString(arn) {
		return fmt.Errorf("invalid role ARN format: %s", arn)
	}
	return nil
}

func ValidateRegions(primary, replica string) error {
	if replica != "" && primary == replica {
		return fmt.Errorf("replica region '%s' cannot be the same as primary region '%s'", replica, primary)
	}
	return nil
}

func ValidateSecureStringRequirements(paramType, kmsKey string) error {
	if paramType == "SecureString" && kmsKey == "" {
		return fmt.Errorf("KMS key is required for SecureString parameters")
	}
	return nil
}
