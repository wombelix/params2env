// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// MockSSMClient implements SSMAPI for testing
type MockSSMClient struct {
	GetParamFunc    func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParamFunc    func(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParamFunc func(context.Context, *ssm.DeleteParameterInput, ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
}

func (m *MockSSMClient) GetParameter(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.GetParamFunc != nil {
		return m.GetParamFunc(ctx, input, opts...)
	}
	return nil, fmt.Errorf("GetParameter not implemented")
}

func (m *MockSSMClient) PutParameter(ctx context.Context, input *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.PutParamFunc != nil {
		return m.PutParamFunc(ctx, input, opts...)
	}
	return nil, fmt.Errorf("PutParameter not implemented")
}

func (m *MockSSMClient) DeleteParameter(ctx context.Context, input *ssm.DeleteParameterInput, opts ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.DeleteParamFunc != nil {
		return m.DeleteParamFunc(ctx, input, opts...)
	}
	return nil, fmt.Errorf("DeleteParameter not implemented")
}
