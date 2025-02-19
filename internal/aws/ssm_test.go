// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestGetParameter(t *testing.T) {
	tests := []struct {
		name        string
		paramName   string
		mockFunc    func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:      "successful get",
			paramName: "/test/param",
			mockFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				value := "test-value"
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Value: &value,
					},
				}, nil
			},
			want:    "test-value",
			wantErr: false,
		},
		{
			name:        "empty parameter name",
			paramName:   "",
			mockFunc:    nil,
			wantErr:     true,
			errContains: "parameter name is required",
		},
		{
			name:      "parameter not found",
			paramName: "/test/notfound",
			mockFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return nil, &types.ParameterNotFound{}
			},
			wantErr:     true,
			errContains: "parameter not found",
		},
		{
			name:      "nil value",
			paramName: "/test/nil",
			mockFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Value: nil,
					},
				}, nil
			},
			wantErr:     true,
			errContains: "has no value",
		},
		{
			name:      "nil parameter",
			paramName: "/test/nilparam",
			mockFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{
					Parameter: nil,
				}, nil
			},
			wantErr:     true,
			errContains: "has no value",
		},
		{
			name:      "aws error",
			paramName: "/test/error",
			mockFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return nil, fmt.Errorf("AWS error")
			},
			wantErr:     true,
			errContains: "failed to get parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				SSMClient: &MockSSMClient{
					GetParamFunc: tt.mockFunc,
				},
			}

			got, err := client.GetParameter(context.Background(), tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetParameter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !stringContains(err.Error(), tt.errContains) {
					t.Errorf("GetParameter() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if got != tt.want {
				t.Errorf("GetParameter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateParameter(t *testing.T) {
	tests := []struct {
		name        string
		paramName   string
		value       string
		description string
		paramType   string
		kmsKeyID    *string
		overwrite   bool
		mockFunc    func(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful create string",
			paramName:   "/test/param",
			value:       "test-value",
			description: "test description",
			paramType:   ParameterTypeString,
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:        "successful create secure string",
			paramName:   "/test/secret",
			value:       "secret-value",
			description: "test secret",
			paramType:   ParameterTypeSecureString,
			kmsKeyID:    strPtr("test-key"),
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:        "empty parameter name",
			paramName:   "",
			value:       "test-value",
			paramType:   ParameterTypeString,
			wantErr:     true,
			errContains: "parameter name is required",
		},
		{
			name:        "empty parameter value",
			paramName:   "/test/param",
			value:       "",
			paramType:   ParameterTypeString,
			wantErr:     true,
			errContains: "parameter value is required",
		},
		{
			name:        "invalid parameter type",
			paramName:   "/test/param",
			value:       "test-value",
			paramType:   "InvalidType",
			wantErr:     true,
			errContains: "invalid parameter type",
		},
		{
			name:      "parameter already exists",
			paramName: "/test/exists",
			value:     "test-value",
			paramType: ParameterTypeString,
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return nil, &types.ParameterAlreadyExists{}
			},
			wantErr:     true,
			errContains: "parameter already exists",
		},
		{
			name:        "with overwrite",
			paramName:   "/test/overwrite",
			value:       "test-value",
			description: "test description",
			paramType:   ParameterTypeString,
			overwrite:   true,
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:      "aws error",
			paramName: "/test/error",
			value:     "test-value",
			paramType: ParameterTypeString,
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return nil, fmt.Errorf("AWS error")
			},
			wantErr:     true,
			errContains: "failed to create parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				SSMClient: &MockSSMClient{
					PutParamFunc: tt.mockFunc,
				},
			}

			err := client.CreateParameter(context.Background(), tt.paramName, tt.value, tt.description, tt.paramType, tt.kmsKeyID, tt.overwrite)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateParameter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !stringContains(err.Error(), tt.errContains) {
					t.Errorf("CreateParameter() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestModifyParameter(t *testing.T) {
	tests := []struct {
		name        string
		paramName   string
		value       string
		description string
		mockFunc    func(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful modify",
			paramName:   "/test/param",
			value:       "new-value",
			description: "updated description",
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:        "empty parameter name",
			paramName:   "",
			value:       "new-value",
			wantErr:     true,
			errContains: "parameter name is required",
		},
		{
			name:        "empty parameter value",
			paramName:   "/test/param",
			value:       "",
			wantErr:     true,
			errContains: "parameter value is required",
		},
		{
			name:      "parameter not found",
			paramName: "/test/notfound",
			value:     "new-value",
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return nil, &types.ParameterNotFound{}
			},
			wantErr:     true,
			errContains: "parameter not found",
		},
		{
			name:      "aws error",
			paramName: "/test/error",
			value:     "new-value",
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return nil, fmt.Errorf("AWS error")
			},
			wantErr:     true,
			errContains: "failed to modify parameter",
		},
		{
			name:        "with empty description",
			paramName:   "/test/param",
			value:       "new-value",
			description: "",
			mockFunc: func(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				SSMClient: &MockSSMClient{
					PutParamFunc: tt.mockFunc,
				},
			}

			err := client.ModifyParameter(context.Background(), tt.paramName, tt.value, tt.description)
			if (err != nil) != tt.wantErr {
				t.Errorf("ModifyParameter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !stringContains(err.Error(), tt.errContains) {
					t.Errorf("ModifyParameter() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		role      string
		wantErr   bool
		errString string
	}{
		{
			name:    "basic client",
			region:  "us-west-2",
			wantErr: false,
		},
		{
			name:    "with role",
			region:  "us-west-2",
			role:    "arn:aws:iam::123:role/test",
			wantErr: false,
		},
		{
			name:      "empty region",
			region:    "",
			wantErr:   true,
			errString: "region is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(context.Background(), tt.region, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !stringContains(err.Error(), tt.errString) {
				t.Errorf("NewClient() error = %v, want error containing %v", err, tt.errString)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestMockSSMClient(t *testing.T) {
	t.Run("mock get parameter without function", func(t *testing.T) {
		mock := &MockSSMClient{}
		_, err := mock.GetParameter(context.Background(), nil)
		if err == nil || err.Error() != "GetParameter not implemented" {
			t.Errorf("Expected 'GetParameter not implemented' error, got %v", err)
		}
	})

	t.Run("mock put parameter without function", func(t *testing.T) {
		mock := &MockSSMClient{}
		_, err := mock.PutParameter(context.Background(), nil)
		if err == nil || err.Error() != "PutParameter not implemented" {
			t.Errorf("Expected 'PutParameter not implemented' error, got %v", err)
		}
	})
}

func TestDeleteParameter(t *testing.T) {
	tests := []struct {
		name    string
		mock    *MockSSMClient
		wantErr bool
	}{
		{
			name: "successful_delete",
			mock: &MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					return &ssm.DeleteParameterOutput{}, nil
				},
			},
			wantErr: false,
		},
		{
			name: "parameter_not_found",
			mock: &MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					return nil, &types.ParameterNotFound{}
				},
			},
			wantErr: true,
		},
		{
			name: "aws_error",
			mock: &MockSSMClient{
				DeleteParamFunc: func(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
					return nil, fmt.Errorf("AWS error")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{SSMClient: tt.mock}
			err := client.DeleteParameter(context.Background(), "/test/param")
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.DeleteParameter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper functions
func stringContains(s, substr string) bool {
	return s != "" && substr != "" && len(s) >= len(substr) && s[len(s)-len(substr):] == substr || s[:len(substr)] == substr
}

func strPtr(s string) *string {
	return &s
}
