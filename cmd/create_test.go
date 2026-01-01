// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
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
		name       string
		flags      createFlags
		stdinValue string
		wantErr    bool
	}{
		{
			name:    "missing_path",
			flags:   createFlags{value: "test"},
			wantErr: true,
		},
		{
			name:    "basic_create_with_value_flag",
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "create_with_description_value_flag",
			flags:   createFlags{path: "/test/param", value: "test", description: "Test parameter", region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "create_with_replica_value_flag",
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2", replica: "eu-west-1"},
			wantErr: false,
		},
		{
			name:       "create_string_via_stdin",
			flags:      createFlags{path: "/test/param", region: "us-west-2"},
			stdinValue: "test-from-stdin",
			wantErr:    false,
		},
		{
			name:       "create_securestring_via_stdin",
			flags:      createFlags{path: "/test/param", paramType: "SecureString", kms: "alias/aws/ssm", region: "us-west-2"},
			stdinValue: "mysecretvalue",
			wantErr:    false,
		},
		{
			name:       "no_value_no_stdin",
			flags:      createFlags{path: "/test/param", region: "us-west-2"},
			stdinValue: "",
			wantErr:    true,
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
		{
			name:    "same_region_validation",
			flags:   createFlags{path: "/test/param", value: "test", region: "us-west-2", replica: "us-west-2"},
			wantErr: true,
		},
		{
			name:       "secure_string_without_kms",
			flags:      createFlags{path: "/test/param", paramType: "SecureString", region: "us-west-2"},
			stdinValue: "secret",
			wantErr:    true,
		},
		{
			name:       "secure_string_with_kms",
			flags:      createFlags{path: "/test/param", paramType: "SecureString", kms: "alias/test-key", region: "us-west-2"},
			stdinValue: "mysecret",
			wantErr:    false,
		},
		{
			name:       "secure_string_empty_stdin_no_value",
			flags:      createFlags{path: "/test/param", paramType: "SecureString", kms: "alias/test-key", region: "us-west-2"},
			stdinValue: "",
			wantErr:    true,
		},
		{
			name:       "value_with_spaces_preserved",
			flags:      createFlags{path: "/test/param", region: "us-west-2"},
			stdinValue: "  value with spaces  ",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.output.Reset()

			mockClient := &aws.MockSSMClient{
				PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
					return &ssm.PutParameterOutput{}, nil
				},
			}
			ts.setupMockClient(mockClient)

			oldStdin := os.Stdin
			if tt.flags.value == "" {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatalf("Failed to create pipe: %v", err)
				}
				if tt.stdinValue != "" {
					if _, err := w.WriteString(tt.stdinValue + "\n"); err != nil {
						t.Fatalf("Failed to write to pipe: %v", err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatalf("Failed to close pipe: %v", err)
				}
				os.Stdin = r
			}
			defer func() { os.Stdin = oldStdin }()

			setupCreateFlags()
			testRoot.AddCommand(createCmd)

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

			setupCreateFlags()
			testRoot.AddCommand(createCmd)

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

func TestInvalidAWSRegionEnvVar(t *testing.T) {
	origRegion := os.Getenv("AWS_REGION")
	origHome := os.Getenv("HOME")
	origNewClient := aws.NewClient
	origCreateRegion := createRegion

	tmpDir, err := os.MkdirTemp("", "params2env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer func() {
		_ = os.Setenv("AWS_REGION", origRegion)
		_ = os.Setenv("HOME", origHome)
		aws.NewClient = origNewClient
		createRegion = origCreateRegion
		_ = os.RemoveAll(tmpDir)
	}()

	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set HOME: %v", err)
	}

	if err := os.Setenv("AWS_REGION", "invalid-region-format"); err != nil {
		t.Fatalf("Failed to set AWS_REGION: %v", err)
	}

	aws.NewClient = func(ctx context.Context, region, role string) (*aws.Client, error) {
		return &aws.Client{SSMClient: &aws.MockSSMClient{}}, nil
	}

	setupCreateFlags()

	createRegion = ""
	createPath = "/test/param"
	createValue = "test-value"
	createType = "String"

	err = ensureRegionIsSet()

	if err == nil {
		t.Error("ensureRegionIsSet() should return error for invalid AWS_REGION")
		return
	}

	errMsg := err.Error()
	if !containsString(errMsg, "invalid AWS_REGION") {
		t.Errorf("ensureRegionIsSet() error = %q, want error containing 'invalid AWS_REGION'", errMsg)
	}
	if !containsString(errMsg, "invalid region format") {
		t.Errorf("ensureRegionIsSet() error = %q, want error containing 'invalid region format'", errMsg)
	}
	if !containsString(errMsg, "invalid-region-format") {
		t.Errorf("ensureRegionIsSet() error = %q, want error containing the actual invalid value 'invalid-region-format'", errMsg)
	}
}

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

func TestReadValueFromStdin(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "simple value",
			input:     "mysecret\n",
			wantValue: "mysecret",
			wantErr:   false,
		},
		{
			name:      "value with spaces preserved",
			input:     "my secret value\n",
			wantValue: "my secret value",
			wantErr:   false,
		},
		{
			name:      "empty value",
			input:     "\n",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "value without newline",
			input:     "mysecret",
			wantValue: "mysecret",
			wantErr:   false,
		},
		{
			name:      "leading spaces preserved",
			input:     "  leadingspaces\n",
			wantValue: "  leadingspaces",
			wantErr:   false,
		},
		{
			name:      "trailing spaces preserved",
			input:     "trailingspaces  \n",
			wantValue: "trailingspaces  ",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tt.input)
			value, err := readValueFromReader(reader)

			if (err != nil) != tt.wantErr {
				t.Errorf("readValueFromReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && value != tt.wantValue {
				t.Errorf("readValueFromReader() = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestReadValueInteractive(t *testing.T) {
	tests := []struct {
		name      string
		paramType string
		input     string
		wantValue string
	}{
		{
			name:      "string type from stdin",
			paramType: "String",
			input:     "regular-value\n",
			wantValue: "regular-value",
		},
		{
			name:      "securestring type from stdin",
			paramType: "SecureString",
			input:     "secret-value\n",
			wantValue: "secret-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			if _, err := w.WriteString(tt.input); err != nil {
				t.Fatalf("Failed to write to pipe: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Failed to close pipe: %v", err)
			}

			oldStdin := os.Stdin
			os.Stdin = r
			defer func() { os.Stdin = oldStdin }()

			value, err := readValueInteractive(tt.paramType)
			if err != nil {
				t.Errorf("readValueInteractive() error = %v", err)
				return
			}

			if value != tt.wantValue {
				t.Errorf("readValueInteractive() = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func readValueFromReader(r io.Reader) (string, error) {
	buf := make([]byte, 4096)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	value := string(buf[:n])
	if len(value) > 0 && value[len(value)-1] == '\n' {
		value = value[:len(value)-1]
	}

	if value == "" {
		return "", io.EOF
	}

	return value, nil
}

func TestExplicitEmptyValueFlag(t *testing.T) {
	ts := setupTest(t)
	t.Cleanup(ts.cleanup)

	mockClient := &aws.MockSSMClient{
		PutParamFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return &ssm.PutParameterOutput{}, nil
		},
	}
	ts.setupMockClient(mockClient)

	setupCreateFlags()
	testRoot.AddCommand(createCmd)

	// Explicitly pass --value ''
	testRoot.SetArgs([]string{"create", "--path", "/test/param", "--value", "", "--region", "us-west-2"})
	err := testRoot.Execute()

	if err == nil {
		t.Error("expected error for explicit empty value, got nil")
	}
	if err != nil && err.Error() != "value cannot be empty" {
		t.Errorf("unexpected error: %v", err)
	}
}
