/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleEndpointStatus(t *testing.T) {
	const endpointId = "endpoint-1"
	endpointIp := "10.0.0.8"

	testCases := []struct {
		name             string
		status           string
		endpointIp       *string
		expectedIp       string
		expectedTerminal bool
		expectedErr      string
	}{
		{
			name:             "GIVEN accepted endpoint with ip WHEN handleEndpointStatus SHOULD return ready ip and terminal status",
			status:           "accepted",
			endpointIp:       &endpointIp,
			expectedIp:       endpointIp,
			expectedTerminal: true,
		},
		{
			name:             "GIVEN creating endpoint WHEN handleEndpointStatus SHOULD return non-terminal status",
			status:           "creating",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: false,
		},
		{
			name:             "GIVEN pending endpoint WHEN handleEndpointStatus SHOULD return non-terminal status",
			status:           "pendingAcceptance",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: false,
		},
		{
			name:             "GIVEN unknown endpoint status WHEN handleEndpointStatus SHOULD return non-terminal status",
			status:           "unknown",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: false,
		},
		{
			name:             "GIVEN accepted endpoint without ip WHEN handleEndpointStatus SHOULD return error",
			status:           "accepted",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s is accepted but has no IP", endpointId),
		},
		{
			name:             "GIVEN failed endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "failed",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s status is failed", endpointId),
		},
		{
			name:             "GIVEN rejected endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "rejected",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s status is rejected", endpointId),
		},
		{
			name:             "GIVEN deleting endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "deleting",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s is being deleted", endpointId),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualIp, actualTerminal, actualErr := handleEndpointStatus(context.Background(), endpointId, tc.status,
				tc.endpointIp)

			assert.Equal(t, tc.expectedIp, actualIp)
			assert.Equal(t, tc.expectedTerminal, actualTerminal)
			if tc.expectedErr == "" {
				assert.NoError(t, actualErr)
				return
			}
			assert.EqualError(t, actualErr, tc.expectedErr)
		})
	}
}
