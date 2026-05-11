/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNotFound(t *testing.T) {
	testCases := []struct {
		name     string
		code     int
		expected bool
	}{
		{
			name:     "GIVEN resource not found code WHEN IsNotFound SHOULD return true",
			code:     StatusCodeResourceNotFound,
			expected: true,
		},
		{
			name:     "GIVEN success code WHEN IsNotFound SHOULD return false",
			code:     StatusCodeSuccess,
			expected: false,
		},
		{
			name:     "GIVEN no changes code WHEN IsNotFound SHOULD return false",
			code:     StatusCodeNoChanges,
			expected: false,
		},
		{
			name:     "GIVEN unknown code WHEN IsNotFound SHOULD return false",
			code:     -1,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsNotFound(tc.code)

			assert.Equal(t, tc.expected, actual)
		})
	}
}
