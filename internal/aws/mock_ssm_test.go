// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"testing"
)

func TestMockSSMClientGetParameterWithoutFunction(t *testing.T) {
	mock := &MockSSMClient{}
	_, err := mock.GetParameter(context.Background(), nil)
	if err == nil {
		t.Error("MockSSMClient.GetParameter() expected error, got nil")
	}
}

func TestMockSSMClientPutParameterWithoutFunction(t *testing.T) {
	mock := &MockSSMClient{}
	_, err := mock.PutParameter(context.Background(), nil)
	if err == nil {
		t.Error("MockSSMClient.PutParameter() expected error, got nil")
	}
}

func TestMockSSMClientDeleteParameterWithoutFunction(t *testing.T) {
	mock := &MockSSMClient{}
	_, err := mock.DeleteParameter(context.Background(), nil)
	if err == nil {
		t.Error("MockSSMClient.DeleteParameter() expected error, got nil")
	}
}
