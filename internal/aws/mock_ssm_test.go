// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"testing"
)

func TestMockSSMClientWithoutFunctions(t *testing.T) {
	tests := []struct {
		name    string
		mock    *MockSSMClient
		wantErr bool
	}{
		{
			name:    "mock_get_parameter_without_function",
			mock:    &MockSSMClient{},
			wantErr: true,
		},
		{
			name:    "mock_put_parameter_without_function",
			mock:    &MockSSMClient{},
			wantErr: true,
		},
		{
			name:    "mock_delete_parameter_without_function",
			mock:    &MockSSMClient{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "mock_get_parameter_without_function":
				_, err := tt.mock.GetParameter(context.Background(), nil)
				if (err != nil) != tt.wantErr {
					t.Errorf("MockSSMClient.GetParameter() error = %v, wantErr %v", err, tt.wantErr)
				}
			case "mock_put_parameter_without_function":
				_, err := tt.mock.PutParameter(context.Background(), nil)
				if (err != nil) != tt.wantErr {
					t.Errorf("MockSSMClient.PutParameter() error = %v, wantErr %v", err, tt.wantErr)
				}
			case "mock_delete_parameter_without_function":
				_, err := tt.mock.DeleteParameter(context.Background(), nil)
				if (err != nil) != tt.wantErr {
					t.Errorf("MockSSMClient.DeleteParameter() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
