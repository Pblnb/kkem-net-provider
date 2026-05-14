/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	"github.com/stretchr/testify/assert"
)

func Test_isVpcepNotFoundError(t *testing.T) {
	const testVpcepNotFoundErrorCode = "EndPoint.0005"

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "GIVEN 404 status code and vpcep not found error code WHEN isVpcepNotFoundError SHOULD return true",
			err:      &sdkerr.ServiceResponseError{StatusCode: http.StatusNotFound, ErrorCode: testVpcepNotFoundErrorCode},
			expected: true,
		},
		{
			name:     "GIVEN wrapped 404 status code and vpcep not found error code WHEN isVpcepNotFoundError SHOULD return true",
			err:      fmt.Errorf("wrapped error: %w", &sdkerr.ServiceResponseError{StatusCode: http.StatusNotFound, ErrorCode: testVpcepNotFoundErrorCode}),
			expected: true,
		},
		{
			name:     "GIVEN nil error WHEN isVpcepNotFoundError SHOULD return false",
			err:      nil,
			expected: false,
		},
		{
			name:     "GIVEN non sdk error WHEN isVpcepNotFoundError SHOULD return false",
			err:      errors.New("network failed"),
			expected: false,
		},
		{
			name:     "GIVEN 500 status code and vpcep not found error code WHEN isVpcepNotFoundError SHOULD return false",
			err:      &sdkerr.ServiceResponseError{StatusCode: http.StatusInternalServerError, ErrorCode: testVpcepNotFoundErrorCode},
			expected: false,
		},
		{
			name:     "GIVEN 404 status code and non-vpcep error code WHEN isVpcepNotFoundError SHOULD return false",
			err:      &sdkerr.ServiceResponseError{StatusCode: http.StatusNotFound, ErrorCode: "Other.0001"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isVpcepNotFoundError(tc.err)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_boolPtr(t *testing.T) {
	testCases := []struct {
		name     string
		input    bool
		expected bool
	}{
		{
			name:     "GIVEN true value WHEN boolPtr SHOULD return true pointer",
			input:    true,
			expected: true,
		},
		{
			name:     "GIVEN false value WHEN boolPtr SHOULD return false pointer",
			input:    false,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := boolPtr(tc.input)

			assert.NotNil(t, actual)
			assert.Equal(t, tc.expected, *actual)
		})
	}
}

func Test_retryWithBackoff(t *testing.T) {
	expectedErr := errors.New("temporary failed")
	testCases := []struct {
		name             string
		maxRetries       int
		baseInterval     time.Duration
		ctx              func() context.Context
		operation        func(*int) error
		expectedErr      error
		expectedAttempts int
	}{
		{
			name:         "GIVEN operation succeeds first time WHEN retryWithBackoff SHOULD return nil",
			maxRetries:   3,
			baseInterval: time.Millisecond,
			ctx: func() context.Context {
				return context.Background()
			},
			operation: func(_ *int) error {
				return nil
			},
			expectedErr:      nil,
			expectedAttempts: 1,
		},
		{
			name:         "GIVEN operation succeeds after retry WHEN retryWithBackoff SHOULD return nil",
			maxRetries:   3,
			baseInterval: time.Millisecond,
			ctx: func() context.Context {
				return context.Background()
			},
			operation: func(attempts *int) error {
				if *attempts == 1 {
					return expectedErr
				}
				return nil
			},
			expectedErr:      nil,
			expectedAttempts: 2,
		},
		{
			name:         "GIVEN zero max retries WHEN retryWithBackoff SHOULD return nil without operation",
			maxRetries:   0,
			baseInterval: time.Millisecond,
			ctx: func() context.Context {
				return context.Background()
			},
			operation: func(_ *int) error {
				return expectedErr
			},
			expectedErr:      nil,
			expectedAttempts: 0,
		},
		{
			name:         "GIVEN context is canceled WHEN retryWithBackoff SHOULD return context error",
			maxRetries:   3,
			baseInterval: time.Hour,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			operation: func(_ *int) error {
				return expectedErr
			},
			expectedErr:      context.Canceled,
			expectedAttempts: 1,
		},
		{
			name:         "GIVEN operation always fails WHEN retryWithBackoff SHOULD return last error",
			maxRetries:   2,
			baseInterval: time.Millisecond,
			ctx: func() context.Context {
				return context.Background()
			},
			operation: func(_ *int) error {
				return expectedErr
			},
			expectedErr:      expectedErr,
			expectedAttempts: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0

			actualErr := retryWithBackoff(tc.ctx(), tc.maxRetries, tc.baseInterval, func() error {
				attempts++
				return tc.operation(&attempts)
			})

			assert.ErrorIs(t, actualErr, tc.expectedErr)
			assert.Equal(t, tc.expectedAttempts, attempts)
		})
	}
}
