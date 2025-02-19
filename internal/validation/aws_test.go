// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package validation

import (
	"strings"
	"testing"
)

func TestValidateParameterPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple path",
			path:    "/test/param",
			wantErr: false,
		},
		{
			name:    "valid complex path",
			path:    "/myapp/prod/db/password.123-TEST",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "parameter path cannot be empty",
		},
		{
			name:    "no leading slash",
			path:    "test/param",
			wantErr: true,
			errMsg:  "parameter path must start with '/'",
		},
		{
			name:    "trailing slash",
			path:    "/test/param/",
			wantErr: true,
			errMsg:  "parameter path must not end with '/'",
		},
		{
			name:    "consecutive slashes",
			path:    "/test//param",
			wantErr: true,
			errMsg:  "parameter path must not contain consecutive '/'",
		},
		{
			name:    "invalid characters",
			path:    "/test/param$#@",
			wantErr: true,
			errMsg:  "invalid parameter path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameterPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParameterPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateParameterPath() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid us-east-1",
			region:  "us-east-1",
			wantErr: false,
		},
		{
			name:    "valid eu-central-1",
			region:  "eu-central-1",
			wantErr: false,
		},
		{
			name:    "valid ap-southeast-2",
			region:  "ap-southeast-2",
			wantErr: false,
		},
		{
			name:    "empty region",
			region:  "",
			wantErr: true,
			errMsg:  "region cannot be empty",
		},
		{
			name:    "invalid format",
			region:  "useast1",
			wantErr: true,
			errMsg:  "invalid region format",
		},
		{
			name:    "invalid characters",
			region:  "us-EAST-1",
			wantErr: true,
			errMsg:  "invalid region format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegion(tt.region)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateRegion() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateKMSKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid key ID",
			key:     "1234abcd-12ab-34cd-56ef-1234567890ab",
			wantErr: false,
		},
		{
			name:    "valid alias",
			key:     "alias/my-key",
			wantErr: false,
		},
		{
			name:    "valid alias with path",
			key:     "alias/my/nested/key",
			wantErr: false,
		},
		{
			name:    "valid ARN",
			key:     "arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab",
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
			errMsg:  "KMS key identifier cannot be empty",
		},
		{
			name:    "invalid key ID format",
			key:     "not-a-uuid",
			wantErr: true,
			errMsg:  "invalid KMS key format",
		},
		{
			name:    "invalid alias format",
			key:     "alias",
			wantErr: true,
			errMsg:  "invalid KMS key format",
		},
		{
			name:    "invalid ARN format",
			key:     "arn:aws:kms:invalid:123456789012:key/not-a-uuid",
			wantErr: true,
			errMsg:  "invalid KMS key format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKMSKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKMSKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateKMSKey() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateRoleARN(t *testing.T) {
	tests := []struct {
		name    string
		arn     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple role ARN",
			arn:     "arn:aws:iam::123456789012:role/test-role",
			wantErr: false,
		},
		{
			name:    "valid role ARN with path",
			arn:     "arn:aws:iam::123456789012:role/service/test-role",
			wantErr: false,
		},
		{
			name:    "valid role ARN with special chars",
			arn:     "arn:aws:iam::123456789012:role/test.role@123",
			wantErr: false,
		},
		{
			name:    "empty ARN",
			arn:     "",
			wantErr: true,
			errMsg:  "role ARN cannot be empty",
		},
		{
			name:    "invalid account ID",
			arn:     "arn:aws:iam::1234:role/test-role",
			wantErr: true,
			errMsg:  "invalid role ARN format",
		},
		{
			name:    "invalid role name",
			arn:     "arn:aws:iam::123456789012:role/test#role",
			wantErr: true,
			errMsg:  "invalid role ARN format",
		},
		{
			name:    "invalid ARN format",
			arn:     "not:an:arn",
			wantErr: true,
			errMsg:  "invalid role ARN format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleARN(tt.arn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleARN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateRoleARN() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}
