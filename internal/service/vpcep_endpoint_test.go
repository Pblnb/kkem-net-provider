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
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
	"github.com/stretchr/testify/assert"
)

const (
	testVpcepEndpointId = "endpoint-1"
	testVpcepEndpointIp = "10.0.0.8"
	testSubnetId        = "subnet-1"
	testVpcepServiceId  = "service-1"
	testVpcId           = "vpc-1"
)

var vpcepNotFoundError = &sdkerr.ServiceResponseError{
	StatusCode: http.StatusNotFound,
	ErrorCode:  "EndPoint.0005",
}

func TestNewVpcepEndpointService(t *testing.T) {
	fake := &vpcepEndpointClientFake{}

	actual := NewVpcepEndpointService(fake)

	assert.NotNil(t, actual)
	assert.Equal(t, fake, actual.client)
	assert.Equal(t, pollingInterval, actual.pollingInterval)
	assert.Equal(t, pollingTimeout, actual.pollingTimeout)
	assert.Equal(t, retryBaseDelay, actual.retryBaseDelay)
}

func TestVpcepEndpointService_Create(t *testing.T) {
	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *VpcepEndpointService
		expectedId          string
		expectedIp          string
		expectedErr         string
		expectedCreateCalls int
	}{
		{
			name: "GIVEN valid input and accepted endpoint WHEN Create SHOULD return endpoint id and ip",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{
					{resp: buildCreateEndpointResponse(testVpcepEndpointId, "creating")},
				},
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expectedId:          testVpcepEndpointId,
			expectedIp:          testVpcepEndpointIp,
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api fails once then succeeds WHEN Create SHOULD return endpoint id and ip",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{
					{err: errors.New("create failed")},
					{resp: buildCreateEndpointResponse(testVpcepEndpointId, "creating")},
				},
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expectedId:          testVpcepEndpointId,
			expectedIp:          testVpcepEndpointIp,
			expectedCreateCalls: 2,
		},
		{
			name: "GIVEN nil create response WHEN Create SHOULD return error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{{resp: nil}},
			}),
			expectedErr:         "createEndpoint response is nil",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create response without id WHEN Create SHOULD return error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{{resp: &model.CreateEndpointResponse{}}},
			}),
			expectedErr:         "createEndpoint response has no ID",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create response without status WHEN Create SHOULD return error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{{
					resp: &model.CreateEndpointResponse{Id: stringPtr(testVpcepEndpointId)},
				}},
			}),
			expectedErr:         "createEndpoint response has no status",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN canceled context and create api error WHEN Create SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{{err: errors.New("create failed")}},
			}),
			expectedErr:         "createEndpoint API failed after retries: context canceled",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api keeps failing WHEN Create SHOULD return last create error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
				},
			}),
			expectedErr:         "createEndpoint API failed after retries: create failed",
			expectedCreateCalls: 3,
		},
		{
			name: "GIVEN wait failed WHEN Create SHOULD return wrapped wait error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				createResults: []vpcepEndpointCreateResult{
					{resp: buildCreateEndpointResponse(testVpcepEndpointId, "creating")},
				},
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("failed", "")},
				},
			}),
			expectedErr: fmt.Sprintf("wait for vpcep-endpoint ready failed: vpcep-endpoint %s status is failed",
				testVpcepEndpointId),
			expectedCreateCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actualId, actualIp, err := tc.service.Create(ctx, buildVpcEndpointInput())

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedId, actualId)
				assert.Equal(t, tc.expectedIp, actualIp)
			} else {
				assert.Empty(t, actualId)
				assert.Empty(t, actualIp)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*vpcepEndpointClientFake); ok {
				assert.Equal(t, tc.expectedCreateCalls, fake.createCalls)
				if fake.createReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.createReq.Body.EndpointServiceId)
					assert.Equal(t, testVpcId, fake.createReq.Body.VpcId)
					assert.Equal(t, testSubnetId, *fake.createReq.Body.SubnetId)
				}
			}
		})
	}
}

func TestVpcepEndpointService_waitForReady(t *testing.T) {
	testCases := []struct {
		name        string
		ctx         context.Context
		service     *VpcepEndpointService
		expected    string
		expectedErr string
	}{
		{
			name: "GIVEN accepted endpoint status WHEN waitForReady SHOULD return endpoint ip",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expected: testVpcepEndpointIp,
		},
		{
			name: "GIVEN creating then accepted endpoint status WHEN waitForReady SHOULD return endpoint ip",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("creating", "")},
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expected: testVpcepEndpointIp,
		},
		{
			name: "GIVEN canceled context WHEN waitForReady SHOULD return context error",
			ctx:  canceledContext(),
			service: newSlowVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expectedErr: fmt.Sprintf("context cancelled while waiting for vpcep-endpoint %s", testVpcepEndpointId),
		},
		{
			name: "GIVEN timeout WHEN waitForReady SHOULD return timeout error",
			service: newTimeoutVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{{resp: buildListEndpointInfoDetailsResponse("creating", "")}},
			}),
			expectedErr: fmt.Sprintf("timeout waiting for vpcep-endpoint %s to be ready", testVpcepEndpointId),
		},
		{
			name: "GIVEN query errors beyond tolerance WHEN waitForReady SHOULD return query error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedErr: "query vpcep-endpoint status failed: query failed",
		},
		{
			name: "GIVEN response without status WHEN waitForReady SHOULD return error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{{resp: &model.ListEndpointInfoDetailsResponse{}}},
			}),
			expectedErr: "vpcep-endpoint response has no status",
		},
		{
			name: "GIVEN accepted endpoint without ip WHEN waitForReady SHOULD return error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{{resp: buildListEndpointInfoDetailsResponse("accepted", "")}},
			}),
			expectedErr: fmt.Sprintf("vpcep-endpoint %s is accepted but has no IP", testVpcepEndpointId),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.waitForReady(ctx, testVpcepEndpointId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Empty(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestVpcepEndpointService_Delete(t *testing.T) {
	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *VpcepEndpointService
		expectedDeleteCalls int
		expectedErr         string
	}{
		{
			name: "GIVEN delete api succeeds WHEN Delete SHOULD return nil",
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				deleteResults: []vpcepEndpointDeleteResult{{resp: &model.DeleteEndpointResponse{}}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN endpoint not found WHEN Delete SHOULD return nil",
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				deleteResults: []vpcepEndpointDeleteResult{{err: vpcepNotFoundError}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN delete api fails once then succeeds WHEN Delete SHOULD return nil",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				deleteResults: []vpcepEndpointDeleteResult{
					{err: errors.New("delete failed")},
					{resp: &model.DeleteEndpointResponse{}},
				},
			}),
			expectedDeleteCalls: 2,
		},
		{
			name: "GIVEN canceled context and delete api error WHEN Delete SHOULD return context error",
			ctx:  canceledContext(),
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				deleteResults: []vpcepEndpointDeleteResult{{err: errors.New("delete failed")}},
			}),
			expectedDeleteCalls: 1,
			expectedErr:         "context canceled",
		},
		{
			name: "GIVEN delete api keeps failing WHEN Delete SHOULD return last delete error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				deleteResults: []vpcepEndpointDeleteResult{
					{err: errors.New("delete failed")},
					{err: errors.New("delete failed")},
					{err: errors.New("delete failed")},
				},
			}),
			expectedDeleteCalls: 3,
			expectedErr:         "delete failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.Delete(ctx, testVpcepEndpointId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*vpcepEndpointClientFake); ok {
				assert.Equal(t, tc.expectedDeleteCalls, fake.deleteCalls)
			}
		})
	}
}

func TestVpcepEndpointService_Get(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		service           *VpcepEndpointService
		expected          *VpcepEndpointOutput
		expectedListCalls int
		expectedErr       string
	}{
		{
			name: "GIVEN endpoint detail response WHEN Get SHOULD return endpoint output",
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expected: &VpcepEndpointOutput{
				EndpointId: testVpcepEndpointId,
				Status:     "accepted",
				Ip:         testVpcepEndpointIp,
				VpcId:      testVpcId,
				SubnetId:   testSubnetId,
				ServiceId:  testVpcepServiceId,
			},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN endpoint not found WHEN Get SHOULD return nil output",
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{{err: vpcepNotFoundError}},
			}),
			expected:          nil,
			expectedListCalls: 1,
		},
		{
			name: "GIVEN query api fails once then succeeds WHEN Get SHOULD return endpoint output",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{err: errors.New("query failed")},
					{resp: buildListEndpointInfoDetailsResponse("accepted", testVpcepEndpointIp)},
				},
			}),
			expected: &VpcepEndpointOutput{
				EndpointId: testVpcepEndpointId,
				Status:     "accepted",
				Ip:         testVpcepEndpointIp,
				VpcId:      testVpcId,
				SubnetId:   testSubnetId,
				ServiceId:  testVpcepServiceId,
			},
			expectedListCalls: 2,
		},
		{
			name: "GIVEN query api error WHEN Get SHOULD return error",
			ctx:  canceledContext(),
			service: NewVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{{err: errors.New("query failed")}},
			}),
			expectedListCalls: 1,
			expectedErr:       "context canceled",
		},
		{
			name: "GIVEN query api keeps failing WHEN Get SHOULD return last query error",
			service: newFastVpcepEndpointService(&vpcepEndpointClientFake{
				listResults: []vpcepEndpointListResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedListCalls: 3,
			expectedErr:       "query failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.Get(ctx, testVpcepEndpointId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*vpcepEndpointClientFake); ok {
				assert.Equal(t, tc.expectedListCalls, fake.listCalls)
			}
		})
	}
}

func Test_handleEndpointStatus(t *testing.T) {
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
			endpointIp:       stringPtr(testVpcepEndpointIp),
			expectedIp:       testVpcepEndpointIp,
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
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s is accepted but has no IP", testVpcepEndpointId),
		},
		{
			name:             "GIVEN failed endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "failed",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s status is failed", testVpcepEndpointId),
		},
		{
			name:             "GIVEN rejected endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "rejected",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s status is rejected", testVpcepEndpointId),
		},
		{
			name:             "GIVEN deleting endpoint WHEN handleEndpointStatus SHOULD return error",
			status:           "deleting",
			endpointIp:       nil,
			expectedIp:       "",
			expectedTerminal: true,
			expectedErr:      fmt.Sprintf("vpcep-endpoint %s is being deleted", testVpcepEndpointId),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualIp, actualTerminal, actualErr := handleEndpointStatus(context.Background(), testVpcepEndpointId,
				tc.status, tc.endpointIp)

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

type vpcepEndpointClientFake struct {
	createReq     *model.CreateEndpointRequest
	createCalls   int
	createResults []vpcepEndpointCreateResult
	deleteCalls   int
	deleteResults []vpcepEndpointDeleteResult
	listCalls     int
	listResults   []vpcepEndpointListResult
}

type vpcepEndpointCreateResult struct {
	resp *model.CreateEndpointResponse
	err  error
}

type vpcepEndpointDeleteResult struct {
	resp *model.DeleteEndpointResponse
	err  error
}

type vpcepEndpointListResult struct {
	resp *model.ListEndpointInfoDetailsResponse
	err  error
}

func (f *vpcepEndpointClientFake) CreateEndpoint(req *model.CreateEndpointRequest) (
	*model.CreateEndpointResponse, error) {
	f.createCalls++
	f.createReq = req

	if len(f.createResults) == 0 {
		return nil, nil
	}

	result := f.createResults[0]
	f.createResults = f.createResults[1:]

	return result.resp, result.err
}

func (f *vpcepEndpointClientFake) DeleteEndpoint(req *model.DeleteEndpointRequest) (
	*model.DeleteEndpointResponse, error) {
	f.deleteCalls++

	if len(f.deleteResults) == 0 {
		return nil, nil
	}

	result := f.deleteResults[0]
	f.deleteResults = f.deleteResults[1:]

	return result.resp, result.err
}

func (f *vpcepEndpointClientFake) ListEndpointInfoDetails(req *model.ListEndpointInfoDetailsRequest) (
	*model.ListEndpointInfoDetailsResponse, error) {
	f.listCalls++

	if len(f.listResults) == 0 {
		return nil, nil
	}

	result := f.listResults[0]
	f.listResults = f.listResults[1:]

	return result.resp, result.err
}

func buildVpcEndpointInput() VpcEndpointInput {
	return VpcEndpointInput{
		EndpointServiceId: testVpcepServiceId,
		VpcId:             testVpcId,
		SubnetId:          testSubnetId,
	}
}

func buildCreateEndpointResponse(endpointId, status string) *model.CreateEndpointResponse {
	return &model.CreateEndpointResponse{Id: stringPtr(endpointId), Status: stringPtr(status)}
}

func buildListEndpointInfoDetailsResponse(status, ip string) *model.ListEndpointInfoDetailsResponse {
	resp := &model.ListEndpointInfoDetailsResponse{
		Status:            stringPtr(status),
		VpcId:             stringPtr(testVpcId),
		SubnetId:          stringPtr(testSubnetId),
		EndpointServiceId: stringPtr(testVpcepServiceId),
	}
	if ip != "" {
		resp.Ip = stringPtr(ip)
	}
	return resp
}

func stringPtr(value string) *string {
	return &value
}

func newFastVpcepEndpointService(client VpcepEndpointClient) *VpcepEndpointService {
	service := NewVpcepEndpointService(client)
	service.pollingInterval = time.Nanosecond
	service.pollingTimeout = time.Second
	service.retryBaseDelay = time.Nanosecond
	return service
}

func newTimeoutVpcepEndpointService(client VpcepEndpointClient) *VpcepEndpointService {
	service := NewVpcepEndpointService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Nanosecond
	return service
}

func newSlowVpcepEndpointService(client VpcepEndpointClient) *VpcepEndpointService {
	service := NewVpcepEndpointService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Hour
	return service
}
