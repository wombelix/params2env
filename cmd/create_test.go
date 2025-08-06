// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"git.sr.ht/~wombelix/params2env/internal/aws"
	"git.sr.ht/~wombelix/params2env/internal/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type createFlags struct {
	path        string
	value       string
	region      string
	role        string
	replica     string
	description string
	kms         string
	paramType   string
}

func TestRunCreate(t *testing.T) {
	ts := setupTest(t)
	defer ts.cleanup()

	tests := []struct {
		name    string
		flags   createFlags
		wantErr bool
	}{
		{
			name:    "missing_path",
			flags:   createFlags{value: "test"},
			wantErr: true,
		},
		{
			name:    "missing_value",
			flags:   createFlags{path: "/test/param"},
			wantErr: true,
		},
		{
			name:    "basic_create",
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "create_with_description",
			flags:   createFlags{path: "/test/param", value: "test", description: "Test parameter", region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "create_with_replica",
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2", replica: "eu-west-1"},
			wantErr: false,
		},
		{
			name:    "create_with_kms",
			flags:   createFlags{path: "/test/param", value: "test", paramType: "SecureString", kms: "alias/aws/ssm", region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "invalid_type",
			flags:   createFlags{path: "/test/param", value: "test", paramType: "Invalid", region: "us-west-2"},
			wantErr: true,
		},
		{
			name:    "aws_client_error",
			flags:   createFlags{path: "/test/param", value: "test", region: "invalid-region"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Create mock AWS client
			mockClient := &aws.MockSSMClient{
				PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
					return &ssm.PutParameterOutput{}, nil
				},
			}
			ts.setupMockClient(mockClient)

			// Reset flags before each test
			createCmd.ResetFlags()
			createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
			createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
			createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type")
			createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
			createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID")
			createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region")
			createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN")
			createCmd.Flags().StringVar(&createReplica, "replica", "", "Replica region")
			createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing")

			// Add create command to test root
			testRoot.AddCommand(createCmd)

			// Build args
			args := buildArgs("create", map[string]string{
				"path":        tt.flags.path,
				"value":       tt.flags.value,
				"region":      tt.flags.region,
				"role":        tt.flags.role,
				"replica":     tt.flags.replica,
				"description": tt.flags.description,
				"kms":         tt.flags.kms,
				"type":        tt.flags.paramType,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunCreateWithConfig(t *testing.T) {
	ts := setupTest(t)
	defer ts.cleanup()

	configContent := []byte(`
region: us-east-1
role: arn:aws:iam::123456789012:role/test
`)
	ts.setupConfigFile(t, configContent)

	// Create mock AWS client
	mockClient := &aws.MockSSMClient{
		PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
	}
	ts.setupMockClient(mockClient)

	tests := []struct {
		name    string
		cfg     *config.Config
		flags   createFlags
		wantErr bool
	}{
		{
			name:    "use config defaults",
			cfg:     &config.Config{},
			flags:   createFlags{path: "/test/param", value: "test"},
			wantErr: false,
		},
		{
			name:    "override config region",
			cfg:     &config.Config{Region: "us-east-1"},
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2"},
			wantErr: false,
		},
		{
			name: "override config role",
			cfg: &config.Config{
				Role: "arn:aws:iam::123456789012:role/other",
			},
			flags: createFlags{
				path:  "/test/param",
				value: "test-value",
				role:  "arn:aws:iam::123456789012:role/test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			// Reset flags before each test
			createCmd.ResetFlags()
			createCmd.Flags().StringVar(&createPath, "path", "", "Parameter path (required)")
			createCmd.Flags().StringVar(&createValue, "value", "", "Parameter value (required)")
			createCmd.Flags().StringVar(&createType, "type", "String", "Parameter type")
			createCmd.Flags().StringVar(&createDesc, "description", "", "Parameter description")
			createCmd.Flags().StringVar(&createKMS, "kms", "", "KMS key ID")
			createCmd.Flags().StringVar(&createRegion, "region", "", "AWS region")
			createCmd.Flags().StringVar(&createRole, "role", "", "AWS role ARN")
			createCmd.Flags().StringVar(&createReplica, "replica", "", "Replica region")
			createCmd.Flags().BoolVar(&createOverwrite, "overwrite", false, "Overwrite existing")

			// Add create command to test root
			testRoot.AddCommand(createCmd)

			// Build args
			args := buildArgs("create", map[string]string{
				"path":    tt.flags.path,
				"value":   tt.flags.value,
				"region":  tt.flags.region,
				"role":    tt.flags.role,
				"replica": tt.flags.replica,
			})

			testRoot.SetArgs(args)
			err := testRoot.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetReplicaKMSKeyID tests the KMS ARN parsing and validation logic.
// This ensures proper handling of various KMS key formats and prevents data loss
// from malformed ARN parsing that could result in wrong KMS key usage.
func TestGetReplicaKMSKeyID(t *testing.T) {
	tests := []struct {
		name        string
		kmsKeyID    string
		region      string
		expected    string
		expectError bool
	}{
		{
			name:        "valid_arn",
			kmsKeyID:    "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
			region:      "us-west-2",
			expected:    "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
			expectError: false,
		},
		{
			name:        "alias",
			kmsKeyID:    "alias/my-key",
			region:      "us-west-2",
			expected:    "alias/my-key",
			expectError: false,
		},
		{
			name:        "key_id",
			kmsKeyID:    "12345678-1234-1234-1234-123456789012",
			region:      "us-west-2",
			expected:    "12345678-1234-1234-1234-123456789012",
			expectError: false,
		},
		{
			name:        "invalid_arn_too_few_parts",
			kmsKeyID:    "arn:aws:kms:us-east-1",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid_arn_too_many_parts",
			kmsKeyID:    "arn:aws:kms:us-east-1:123456789012:key:extra:part",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty_account",
			kmsKeyID:    "arn:aws:kms:us-east-1::key/123",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid_service",
			kmsKeyID:    "arn:aws:s3:us-east-1:123456789012:key/123",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing_key_prefix",
			kmsKeyID:    "arn:aws:kms:us-east-1:123456789012:123",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty_key_id",
			kmsKeyID:    "arn:aws:kms:us-east-1:123456789012:key/",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid_arn_prefix",
			kmsKeyID:    "arn:invalid:kms:us-east-1:123456789012:key/123",
			region:      "us-west-2",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getReplicaKMSKeyID(tt.kmsKeyID, tt.region)

			if (err != nil) != tt.expectError {
				t.Errorf("getReplicaKMSKeyID() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if result == nil {
					t.Error("getReplicaKMSKeyID() returned nil result for valid input")
					return
				}
				if *result != tt.expected {
					t.Errorf("getReplicaKMSKeyID() = %q, want %q", *result, tt.expected)
				}
			}
		})
	}
}
