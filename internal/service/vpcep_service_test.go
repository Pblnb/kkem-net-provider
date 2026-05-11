/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestExtractTcpPortPairs(t *testing.T) {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	nonTcpProtocol := model.PortListProtocol{}
	clientPort := int32(80)
	serverPort := int32(8080)
	anotherClientPort := int32(443)
	anotherServerPort := int32(8443)

	testCases := []struct {
		name     string
		ports    []model.PortList
		expected []PortPair
	}{
		{
			name: "GIVEN tcp port list WHEN extractTcpPortPairs SHOULD return port pairs",
			ports: []model.PortList{
				{ClientPort: &clientPort, ServerPort: &serverPort, Protocol: &tcpProtocol},
				{ClientPort: &anotherClientPort, ServerPort: &anotherServerPort, Protocol: &tcpProtocol},
			},
			expected: []PortPair{
				{ClientPort: clientPort, ServerPort: serverPort},
				{ClientPort: anotherClientPort, ServerPort: anotherServerPort},
			},
		},
		{
			name:     "GIVEN empty port list WHEN extractTcpPortPairs SHOULD return empty list",
			ports:    []model.PortList{},
			expected: []PortPair{},
		},
		{
			name: "GIVEN non tcp and incomplete port list WHEN extractTcpPortPairs SHOULD return empty list",
			ports: []model.PortList{
				{ClientPort: &clientPort, ServerPort: &serverPort, Protocol: &nonTcpProtocol},
				{ClientPort: nil, ServerPort: &serverPort, Protocol: &tcpProtocol},
				{ClientPort: &clientPort, ServerPort: nil, Protocol: &tcpProtocol},
				{ClientPort: &clientPort, ServerPort: &serverPort, Protocol: nil},
			},
			expected: []PortPair{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := extractTcpPortPairs(tc.ports)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetServerType(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GIVEN vm server type WHEN getServerType SHOULD return vm enum",
			input:    "VM",
			expected: "VM",
		},
		{
			name:     "GIVEN lb server type WHEN getServerType SHOULD return lb enum",
			input:    "LB",
			expected: "LB",
		},
		{
			name:     "GIVEN unknown server type WHEN getServerType SHOULD return lb enum",
			input:    "UNKNOWN",
			expected: "LB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getServerType(tc.input)

			assert.Equal(t, tc.expected, actual.Value())
		})
	}
}
