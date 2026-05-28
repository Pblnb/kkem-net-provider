/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestNewVpcepServiceService(t *testing.T) {
	fake := &mockVpcepServiceClient{}

	actual := NewVpcepServiceService(fake)

	assert.NotNil(t, actual)
	assert.Equal(t, fake, actual.client)
	assert.Equal(t, pollingInterval, actual.pollingInterval)
	assert.Equal(t, pollingTimeout, actual.pollingTimeout)
	assert.Equal(t, retryBaseDelay, actual.retryBaseDelay)
}

func TestVpcepService_Create(t *testing.T) {
	testCases := []struct {
		name                             string
		ctx                              context.Context
		service                          *VpcepServiceService
		makeRequestLogMarshalReturnError bool
		expected                         string
		expectedErr                      *string
		expectedCreateCalls              int
	}{
		{
			name: "GIVEN valid input and ready service WHEN Create SHOULD return service id",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{
					{resp: buildCreateEndpointServiceResponse(testVpcepServiceId, "creating")},
				},
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			expected:            testVpcepServiceId,
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN request log marshal fails WHEN Create SHOULD continue and return service id",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{
					{resp: buildCreateEndpointServiceResponse(testVpcepServiceId, "creating")},
				},
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			makeRequestLogMarshalReturnError: true,
			expected:                         testVpcepServiceId,
			expectedCreateCalls:              1,
		},
		{
			name: "GIVEN create api fails once then succeeds WHEN Create SHOULD return service id",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{
					{err: errors.New("create failed")},
					{resp: buildCreateEndpointServiceResponse(testVpcepServiceId, "creating")},
				},
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			expected:            testVpcepServiceId,
			expectedCreateCalls: 2,
		},
		{
			name: "GIVEN create response without id WHEN Create SHOULD return error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{{resp: &model.CreateEndpointServiceResponse{}}},
			}),
			expectedErr:         ptr("createEndpointService response has no ID"),
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api error and canceled context WHEN Create SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{{err: errors.New("create failed")}},
			}),
			expectedErr:         ptr("createEndpointService API failed after retries: context canceled"),
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api keeps failing WHEN Create SHOULD return wrapped create error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
				},
			}),
			expectedErr:         ptr("createEndpointService API failed after retries: create failed"),
			expectedCreateCalls: 3,
		},
		{
			name: "GIVEN wait failed WHEN Create SHOULD return wrapped wait error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				createResults: []vpcepServiceCreateResult{
					{resp: buildCreateEndpointServiceResponse(testVpcepServiceId, "creating")},
				},
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("failed")},
				},
			}),
			expectedErr: ptr(fmt.Sprintf("wait for vpcep-service ready failed: vpcep-service %s status is failed",
				testVpcepServiceId)),
			expectedCreateCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}
			if tc.makeRequestLogMarshalReturnError {
				patches := gomonkey.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
					return nil, errors.New("marshal failed")
				})
				defer patches.Reset()
			}

			actual, err := tc.service.Create(ctx, buildVpcepServiceInput())

			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Empty(t, actual)
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedCreateCalls, fake.createCalls)
				if fake.createVpcepServiceReq == nil {
					return
				}
				expectedReq := &model.CreateEndpointServiceRequest{
					Body: &model.CreateEndpointServiceRequestBody{
						VpcId:           testVpcId,
						PortId:          testVpcepServicePortId,
						ServerType:      model.GetCreateEndpointServiceRequestBodyServerTypeEnum().VM,
						ApprovalEnabled: ptr(false),
						Ports: []model.PortList{
							{ClientPort: &testVpcepServiceClientPort,
								ServerPort: &testVpcepServiceServerPort,
								Protocol:   &testVpcepServiceTcpProtocol},
						},
						Tags: &[]model.TagList{
							{Key: ptr("creator"), Value: ptr("kkem")},
						},
						Description: ptr("Created by kkem-net-provider"),
						IpVersion:   ptr(model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4),
					},
				}
				assert.Equal(t, expectedReq, fake.createVpcepServiceReq)
			}
		})
	}
}

func TestVpcepService_waitForReady(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		service           *VpcepServiceService
		expectedErr       *string
		expectedListCalls int
	}{
		{
			name: "GIVEN available service status WHEN waitForReady SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			expectedListCalls: 1,
		},
		{
			name: "GIVEN creating then available service status WHEN waitForReady SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("creating")},
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			expectedListCalls: 2,
		},
		{
			name: "GIVEN unknown then available service status WHEN waitForReady SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{
					{resp: buildListServiceDetailsResponse("unknown")},
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			expectedListCalls: 2,
		},
		{
			name: "GIVEN canceled context WHEN waitForReady SHOULD return context error",
			ctx:  canceledContext(),
			service: newSlowPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			expectedErr: ptr(fmt.Sprintf("context cancelled while waiting for vpcep-service %s: context canceled",
				testVpcepServiceId)),
			expectedListCalls: 0,
		},
		{
			name: "GIVEN timeout WHEN waitForReady SHOULD return timeout error",
			service: newTimeoutPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("creating")}},
			}),
			expectedErr: ptr(fmt.Sprintf("timeout waiting for vpcep-service %s to be ready",
				testVpcepServiceId)),
			expectedListCalls: 0,
		},
		{
			name: "GIVEN query errors beyond tolerance WHEN waitForReady SHOULD return query error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedErr:       ptr("query vpcep-service status failed: query failed"),
			expectedListCalls: 3,
		},
		{
			name: "GIVEN response without status WHEN waitForReady SHOULD return error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: &model.ListServiceDetailsResponse{}}},
			}),
			expectedErr:       ptr(fmt.Sprintf("vpcep-service %s response has no status", testVpcepServiceId)),
			expectedListCalls: 1,
		},
		{
			name: "GIVEN failed service status WHEN waitForReady SHOULD return error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("failed")}},
			}),
			expectedErr:       ptr(fmt.Sprintf("vpcep-service %s status is failed", testVpcepServiceId)),
			expectedListCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.waitForReady(ctx, testVpcepServiceId)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedListCalls, fake.listCalls)
				for _, req := range fake.listVpcepServiceDetailsReqs {
					assert.Equal(t, &model.ListServiceDetailsRequest{VpcEndpointServiceId: testVpcepServiceId}, req)
				}
			}
		})
	}
}

func TestVpcepService_Delete(t *testing.T) {
	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *VpcepServiceService
		expectedErr         *string
		expectedDeleteCalls int
	}{
		{
			name: "GIVEN delete api succeeds WHEN Delete SHOULD return nil",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				deleteResults: []vpcepServiceDeleteResult{{resp: &model.DeleteEndpointServiceResponse{}}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN service not found WHEN Delete SHOULD return nil",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				deleteResults: []vpcepServiceDeleteResult{{err: vpcepNotFoundError}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN delete api fails once then succeeds WHEN Delete SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				deleteResults: []vpcepServiceDeleteResult{
					{err: errors.New("delete failed")},
					{resp: &model.DeleteEndpointServiceResponse{}},
				},
			}),
			expectedDeleteCalls: 2,
		},
		{
			name: "GIVEN delete api error and canceled context WHEN Delete SHOULD return context error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				deleteResults: []vpcepServiceDeleteResult{{err: errors.New("delete failed")}},
			}),
			expectedErr:         ptr("context canceled"),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN delete api keeps failing WHEN Delete SHOULD return delete error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				deleteResults: []vpcepServiceDeleteResult{
					{err: errors.New("delete failed")},
					{err: errors.New("delete failed")},
					{err: errors.New("delete failed")},
				},
			}),
			expectedErr:         ptr("delete failed"),
			expectedDeleteCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.Delete(ctx, testVpcepServiceId)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedDeleteCalls, fake.deleteCalls)
				if fake.deleteVpcepServiceReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.deleteVpcepServiceReq.VpcEndpointServiceId)
				}
			}
		})
	}
}

func TestVpcepService_AddPermissions(t *testing.T) {
	testCases := []struct {
		name                        string
		ctx                         context.Context
		service                     *VpcepServiceService
		expectedErr                 *string
		expectedAddPermissionsCalls int
	}{
		{
			name: "GIVEN permissions WHEN AddPermissions SHOULD call add api",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{resp: &model.BatchAddEndpointServicePermissionsResponse{}},
				},
			}),
			expectedAddPermissionsCalls: 1,
		},
		{
			name: "GIVEN add api fails once then succeeds WHEN AddPermissions SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{err: errors.New("add failed")},
					{resp: &model.BatchAddEndpointServicePermissionsResponse{}},
				},
			}),
			expectedAddPermissionsCalls: 2,
		},
		{
			name: "GIVEN add api error and canceled context WHEN AddPermissions SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				addPermissionsResults: []vpcepServiceAddPermissionsResult{{err: errors.New("add failed")}},
			}),
			expectedErr:                 ptr("batchAddEndpointServicePermissions API failed after retries: context canceled"),
			expectedAddPermissionsCalls: 1,
		},
		{
			name: "GIVEN add api keeps failing WHEN AddPermissions SHOULD return wrapped add error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{err: errors.New("add failed")},
					{err: errors.New("add failed")},
					{err: errors.New("add failed")},
				},
			}),
			expectedErr:                 ptr("batchAddEndpointServicePermissions API failed after retries: add failed"),
			expectedAddPermissionsCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.AddPermissions(ctx, testVpcepServiceId,
				[]PermissionInput{{Permission: testVpcepServicePermission}})

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedAddPermissionsCalls, fake.addPermissionsCalls)
				if fake.addPermissionsReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.addPermissionsReq.VpcEndpointServiceId)
					assert.Equal(t, testVpcepServicePermission, fake.addPermissionsReq.Body.Permissions[0].Permission)
				}
			}
		})
	}
}

func TestVpcepService_ReconcilePermissions(t *testing.T) {
	testCases := []struct {
		name                           string
		ctx                            context.Context
		service                        *VpcepServiceService
		desired                        []PermissionInput
		expectedErr                    *string
		expectedListPermissionsCalls   int
		expectedAddPermissionsCalls    int
		expectedRemovePermissionsCalls int
	}{
		{
			name: "GIVEN desired adds and removes permissions WHEN ReconcilePermissions SHOULD sync permissions",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServiceExtraPermission)},
					})},
				},
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{resp: &model.BatchAddEndpointServicePermissionsResponse{}},
				},
				removePermissionsResults: []vpcepServiceRemovePermissionsResult{
					{resp: &model.BatchRemoveEndpointServicePermissionsResponse{}},
				},
			}),
			desired:                        []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedListPermissionsCalls:   1,
			expectedAddPermissionsCalls:    1,
			expectedRemovePermissionsCalls: 1,
		},
		{
			name: "GIVEN remote permissions already match desired WHEN ReconcilePermissions SHOULD return nil",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId),
							Permission: ptr(testVpcepServicePermission)},
					})},
				},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN permission query api fails once then succeeds WHEN ReconcilePermissions SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{err: errors.New("api failed")},
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServicePermission)},
					})},
				},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedListPermissionsCalls: 2,
		},
		{
			name: "GIVEN add permission api fails once then succeeds WHEN ReconcilePermissions SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{})},
				},
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{err: errors.New("api failed")},
					{resp: &model.BatchAddEndpointServicePermissionsResponse{}},
				},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedListPermissionsCalls: 1,
			expectedAddPermissionsCalls:  2,
		},
		{
			name: "GIVEN remove api fails once then succeeds WHEN ReconcilePermissions SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServiceExtraPermission)},
					})},
				},
				removePermissionsResults: []vpcepServiceRemovePermissionsResult{
					{err: errors.New("api failed")},
					{resp: &model.BatchRemoveEndpointServicePermissionsResponse{}},
				},
			}),
			desired:                        []PermissionInput{},
			expectedListPermissionsCalls:   1,
			expectedRemovePermissionsCalls: 2,
		},
		{
			name: "GIVEN permission query error and canceled context WHEN ReconcilePermissions SHOULD return error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{{err: errors.New("api failed")}},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedErr:                  ptr("listServicePermissionsDetails API failed after retries: context canceled"),
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN add permission api error and canceled context WHEN ReconcilePermissions SHOULD return error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{})},
				},
				addPermissionsResults: []vpcepServiceAddPermissionsResult{{err: errors.New("api failed")}},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedErr:                  ptr("batchAddEndpointServicePermissions API failed after retries: context canceled"),
			expectedListPermissionsCalls: 1,
			expectedAddPermissionsCalls:  1,
		},
		{
			name: "GIVEN remove api error and canceled context WHEN ReconcilePermissions SHOULD return wrapped error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServiceExtraPermission)},
					})},
				},
				removePermissionsResults: []vpcepServiceRemovePermissionsResult{{err: errors.New("api failed")}},
			}),
			desired:                        []PermissionInput{},
			expectedErr:                    ptr("batchRemoveEndpointServicePermissions API failed after retries: context canceled"),
			expectedListPermissionsCalls:   1,
			expectedRemovePermissionsCalls: 1,
		},
		{
			name: "GIVEN permission query api keeps failing WHEN ReconcilePermissions SHOULD return query error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
				},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedErr:                  ptr("listServicePermissionsDetails API failed after retries: api failed"),
			expectedListPermissionsCalls: 3,
		},
		{
			name: "GIVEN add permission api keeps failing WHEN ReconcilePermissions SHOULD return add error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{})},
				},
				addPermissionsResults: []vpcepServiceAddPermissionsResult{
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
				},
			}),
			desired:                      []PermissionInput{{Permission: testVpcepServicePermission}},
			expectedErr:                  ptr("batchAddEndpointServicePermissions API failed after retries: api failed"),
			expectedListPermissionsCalls: 1,
			expectedAddPermissionsCalls:  3,
		},
		{
			name: "GIVEN remote permission without id WHEN ReconcilePermissions SHOULD return error",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Permission: ptr(testVpcepServiceExtraPermission)},
					})},
				},
			}),
			desired: []PermissionInput{},
			expectedErr: ptr(fmt.Sprintf("vpcep-service permission %s has no id",
				testVpcepServiceExtraPermission)),
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN remove api keeps failing WHEN ReconcilePermissions SHOULD return remove error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServiceExtraPermission)},
					})},
				},
				removePermissionsResults: []vpcepServiceRemovePermissionsResult{
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
					{err: errors.New("api failed")},
				},
			}),
			desired:                        []PermissionInput{},
			expectedErr:                    ptr("batchRemoveEndpointServicePermissions API failed after retries: api failed"),
			expectedListPermissionsCalls:   1,
			expectedRemovePermissionsCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.ReconcilePermissions(ctx, testVpcepServiceId, tc.desired)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedListPermissionsCalls, fake.listPermissionsDetailsCalls)
				assert.Equal(t, tc.expectedAddPermissionsCalls, fake.addPermissionsCalls)
				assert.Equal(t, tc.expectedRemovePermissionsCalls, fake.removePermissionsCalls)
				if fake.listPermissionsDetailsReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.listPermissionsDetailsReq.VpcEndpointServiceId)
				}
				if fake.addPermissionsReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.addPermissionsReq.VpcEndpointServiceId)
					assert.Equal(t, testVpcepServicePermission, fake.addPermissionsReq.Body.Permissions[0].Permission)
				}
				if fake.removePermissionsReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.removePermissionsReq.VpcEndpointServiceId)
					assert.Equal(t, testVpcepServicePermissionId, fake.removePermissionsReq.Body.Permissions[0].Id)
				}
			}
		})
	}
}

func TestVpcepService_GetPermissions(t *testing.T) {
	testCases := []struct {
		name                         string
		ctx                          context.Context
		service                      *VpcepServiceService
		expectedPermissions          map[string]string
		expectedErr                  *string
		expectedListPermissionsCalls int
	}{
		{
			name: "GIVEN single permission response WHEN GetPermissions SHOULD return permission id map",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServicePermission)},
					})},
				},
			}),
			expectedPermissions:          map[string]string{testVpcepServicePermission: testVpcepServicePermissionId},
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN multiple permissions response WHEN GetPermissions SHOULD return permission id map",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServicePermission)},
						{Id: ptr("permission-2"), Permission: ptr("iam:domain::additional")},
						{Permission: nil},
					})},
				},
			}),
			expectedPermissions: map[string]string{
				testVpcepServicePermission: testVpcepServicePermissionId,
				"iam:domain::additional":   "permission-2",
			},
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN permission query api fails once then succeeds WHEN GetPermissions SHOULD return permission id map",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{err: errors.New("query failed")},
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Id: ptr(testVpcepServicePermissionId), Permission: ptr(testVpcepServicePermission)},
					})},
				},
			}),
			expectedPermissions:          map[string]string{testVpcepServicePermission: testVpcepServicePermissionId},
			expectedListPermissionsCalls: 2,
		},
		{
			name: "GIVEN nil permissions WHEN GetPermissions SHOULD return empty map",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: &model.ListServicePermissionsDetailsResponse{}},
				},
			}),
			expectedPermissions:          map[string]string{},
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN permission without id WHEN GetPermissions SHOULD return permission with empty id",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{resp: buildPermissionsResponse([]model.PermissionObject{
						{Permission: ptr(testVpcepServicePermission)},
					})},
				},
			}),
			expectedPermissions:          map[string]string{testVpcepServicePermission: ""},
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN permission query error and canceled context WHEN GetPermissions SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{{err: errors.New("query failed")}},
			}),
			expectedErr:                  ptr("listServicePermissionsDetails API failed after retries: context canceled"),
			expectedListPermissionsCalls: 1,
		},
		{
			name: "GIVEN permission query api keeps failing WHEN GetPermissions SHOULD return query error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listPermissionsResults: []vpcepServiceListPermissionsResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedErr:                  ptr("listServicePermissionsDetails API failed after retries: query failed"),
			expectedListPermissionsCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.GetPermissions(ctx, testVpcepServiceId)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPermissions, actual)
			} else {
				assert.Nil(t, actual)
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedListPermissionsCalls, fake.listPermissionsDetailsCalls)
				if fake.listPermissionsDetailsReq != nil {
					assert.Equal(t, testVpcepServiceId, fake.listPermissionsDetailsReq.VpcEndpointServiceId)
				}
			}
		})
	}
}

func TestVpcepService_UpdateConfig(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		service           *VpcepServiceService
		input             VpcepServiceInput
		expectedErr       *string
		expectedUpdateReq *model.UpdateEndpointServiceRequest
		expectedListCalls int
		expectedCalls     int
	}{
		{
			name: "GIVEN valid config and ready service WHEN UpdateConfig SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{resp: &model.UpdateEndpointServiceResponse{}}},
				listResults:   []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			input:             buildVpcepServiceInput(),
			expectedUpdateReq: buildUpdateEndpointServiceRequest(),
			expectedListCalls: 1,
			expectedCalls:     1,
		},
		{
			name: "GIVEN config without ports WHEN UpdateConfig SHOULD send empty ports",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{resp: &model.UpdateEndpointServiceResponse{}}},
				listResults:   []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			input: func() VpcepServiceInput {
				input := buildVpcepServiceInput()
				input.Ports = []PortPair{}
				return input
			}(),
			expectedUpdateReq: func() *model.UpdateEndpointServiceRequest {
				req := buildUpdateEndpointServiceRequest()
				req.Body.Ports = ptr([]model.PortList{})
				return req
			}(),
			expectedListCalls: 1,
			expectedCalls:     1,
		},
		{
			name: "GIVEN config with nil ports WHEN UpdateConfig SHOULD send empty ports",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{resp: &model.UpdateEndpointServiceResponse{}}},
				listResults:   []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			input: func() VpcepServiceInput {
				input := buildVpcepServiceInput()
				input.Ports = nil
				return input
			}(),
			expectedUpdateReq: func() *model.UpdateEndpointServiceRequest {
				req := buildUpdateEndpointServiceRequest()
				req.Body.Ports = ptr([]model.PortList{})
				return req
			}(),
			expectedListCalls: 1,
			expectedCalls:     1,
		},
		{
			name: "GIVEN config with empty port id WHEN UpdateConfig SHOULD send empty port id",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{resp: &model.UpdateEndpointServiceResponse{}}},
				listResults:   []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			input: func() VpcepServiceInput {
				input := buildVpcepServiceInput()
				input.PortId = ""
				return input
			}(),
			expectedUpdateReq: func() *model.UpdateEndpointServiceRequest {
				req := buildUpdateEndpointServiceRequest()
				req.Body.PortId = ptr("")
				return req
			}(),
			expectedListCalls: 1,
			expectedCalls:     1,
		},
		{
			name: "GIVEN update api fails once then succeeds WHEN UpdateConfig SHOULD return nil",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{
					{err: errors.New("update failed")},
					{resp: &model.UpdateEndpointServiceResponse{}},
				},
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			input:             buildVpcepServiceInput(),
			expectedUpdateReq: buildUpdateEndpointServiceRequest(),
			expectedListCalls: 1,
			expectedCalls:     2,
		},
		{
			name: "GIVEN update api error and canceled context WHEN UpdateConfig SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{err: errors.New("update failed")}},
			}),
			input:             buildVpcepServiceInput(),
			expectedErr:       ptr("updateEndpointService API failed after retries: context canceled"),
			expectedUpdateReq: buildUpdateEndpointServiceRequest(),
			expectedCalls:     1,
		},
		{
			name: "GIVEN update api keeps failing WHEN UpdateConfig SHOULD return update error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{
					{err: errors.New("update failed")},
					{err: errors.New("update failed")},
					{err: errors.New("update failed")},
				},
			}),
			input:             buildVpcepServiceInput(),
			expectedErr:       ptr("updateEndpointService API failed after retries: update failed"),
			expectedUpdateReq: buildUpdateEndpointServiceRequest(),
			expectedCalls:     3,
		},
		{
			name: "GIVEN wait failed WHEN UpdateConfig SHOULD return wrapped wait error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				updateResults: []vpcepServiceUpdateResult{{resp: &model.UpdateEndpointServiceResponse{}}},
				listResults:   []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("failed")}},
			}),
			input: buildVpcepServiceInput(),
			expectedErr: ptr(fmt.Sprintf("wait for vpcep-service ready failed: vpcep-service %s status is failed",
				testVpcepServiceId)),
			expectedUpdateReq: buildUpdateEndpointServiceRequest(),
			expectedListCalls: 1,
			expectedCalls:     1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.UpdateConfig(ctx, testVpcepServiceId, tc.input)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedCalls, fake.updateCalls)
				assert.Equal(t, tc.expectedListCalls, fake.listCalls)
				if fake.updateVpcepServiceReq != nil {
					assert.Equal(t, tc.expectedUpdateReq, fake.updateVpcepServiceReq)
				}
			}
		})
	}
}

func TestVpcepService_Get(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		service           *VpcepServiceService
		expected          *VpcepServiceOutput
		expectedErr       *string
		expectedListCalls int
	}{
		{
			name: "GIVEN service detail response WHEN Get SHOULD return service output",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: buildListServiceDetailsResponse("available")}},
			}),
			expected: &VpcepServiceOutput{
				ServiceId:  testVpcepServiceId,
				Status:     "available",
				ServerType: "VM",
				VpcId:      testVpcId,
				PortId:     testVpcepServicePortId,
				Ports: []PortPair{{
					ClientPort: testVpcepServiceClientPort,
					ServerPort: testVpcepServiceServerPort,
				}},
			},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN query api fails once then succeeds WHEN Get SHOULD return service output",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{
					{err: errors.New("query failed")},
					{resp: buildListServiceDetailsResponse("available")},
				},
			}),
			expected: &VpcepServiceOutput{
				ServiceId:  testVpcepServiceId,
				Status:     "available",
				ServerType: "VM",
				VpcId:      testVpcId,
				PortId:     testVpcepServicePortId,
				Ports: []PortPair{{
					ClientPort: testVpcepServiceClientPort,
					ServerPort: testVpcepServiceServerPort,
				}},
			},
			expectedListCalls: 2,
		},
		{
			name: "GIVEN service detail response with nil fields WHEN Get SHOULD return service id only",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{resp: &model.ListServiceDetailsResponse{}}},
			}),
			expected:          &VpcepServiceOutput{ServiceId: testVpcepServiceId},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN service detail response with partial nil fields WHEN Get SHOULD return non nil fields",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{
					resp: &model.ListServiceDetailsResponse{
						Status: ptr("available"),
						VpcId:  ptr(testVpcId),
					},
				}},
			}),
			expected: &VpcepServiceOutput{
				ServiceId: testVpcepServiceId,
				Status:    "available",
				VpcId:     testVpcId,
			},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN service not found WHEN Get SHOULD return nil output",
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{err: vpcepNotFoundError}},
			}),
			expectedListCalls: 1,
		},
		{
			name: "GIVEN query api error and canceled context WHEN Get SHOULD return query error",
			ctx:  canceledContext(),
			service: NewVpcepServiceService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{{err: errors.New("query failed")}},
			}),
			expectedErr:       ptr("context canceled"),
			expectedListCalls: 1,
		},
		{
			name: "GIVEN query api keeps failing WHEN Get SHOULD return query error",
			service: newFastPollingVpcepService(&mockVpcepServiceClient{
				listResults: []vpcepServiceListResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedErr:       ptr("query failed"),
			expectedListCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.Get(ctx, testVpcepServiceId)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.ErrorContains(t, err, *tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockVpcepServiceClient); ok {
				assert.Equal(t, tc.expectedListCalls, fake.listCalls)
				for _, req := range fake.listVpcepServiceDetailsReqs {
					assert.Equal(t, &model.ListServiceDetailsRequest{VpcEndpointServiceId: testVpcepServiceId}, req)
				}
			}
		})
	}
}

func Test_extractTcpPortPairs(t *testing.T) {
	testCases := []struct {
		name     string
		ports    []model.PortList
		expected []PortPair
	}{
		{
			name: "GIVEN tcp port list WHEN extractTcpPortPairs SHOULD return port pairs",
			ports: []model.PortList{
				{ClientPort: &testVpcepServiceClientPort, ServerPort: &testVpcepServiceServerPort,
					Protocol: &testVpcepServiceTcpProtocol},
				{ClientPort: ptr(int32(443)), ServerPort: ptr(int32(8443)),
					Protocol: &testVpcepServiceTcpProtocol},
			},
			expected: []PortPair{
				{ClientPort: testVpcepServiceClientPort, ServerPort: testVpcepServiceServerPort},
				{ClientPort: 443, ServerPort: 8443},
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
				{ClientPort: &testVpcepServiceClientPort, ServerPort: &testVpcepServiceServerPort,
					Protocol: ptr(model.PortListProtocol{})},
				{ClientPort: nil, ServerPort: &testVpcepServiceServerPort,
					Protocol: &testVpcepServiceTcpProtocol},
				{ClientPort: &testVpcepServiceClientPort, ServerPort: nil,
					Protocol: &testVpcepServiceTcpProtocol},
				{ClientPort: &testVpcepServiceClientPort, ServerPort: &testVpcepServiceServerPort,
					Protocol: nil},
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

func Test_getServerType(t *testing.T) {
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
			name:     "GIVEN lowercase vm server type WHEN getServerType SHOULD return lb enum",
			input:    "vm",
			expected: "LB",
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
		{
			name:     "GIVEN empty string WHEN getServerType SHOULD return lb enum",
			input:    "",
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

type mockVpcepServiceClient struct {
	createVpcepServiceReq *model.CreateEndpointServiceRequest
	createCalls           int
	createResults         []vpcepServiceCreateResult

	deleteVpcepServiceReq *model.DeleteEndpointServiceRequest
	deleteCalls           int
	deleteResults         []vpcepServiceDeleteResult

	updateVpcepServiceReq *model.UpdateEndpointServiceRequest
	updateCalls           int
	updateResults         []vpcepServiceUpdateResult

	listVpcepServiceDetailsReqs []*model.ListServiceDetailsRequest
	listCalls                   int
	listResults                 []vpcepServiceListResult

	addPermissionsReq     *model.BatchAddEndpointServicePermissionsRequest
	addPermissionsCalls   int
	addPermissionsResults []vpcepServiceAddPermissionsResult

	removePermissionsReq     *model.BatchRemoveEndpointServicePermissionsRequest
	removePermissionsCalls   int
	removePermissionsResults []vpcepServiceRemovePermissionsResult

	listPermissionsDetailsReq   *model.ListServicePermissionsDetailsRequest
	listPermissionsDetailsCalls int
	listPermissionsResults      []vpcepServiceListPermissionsResult
}

type vpcepServiceCreateResult struct {
	resp *model.CreateEndpointServiceResponse
	err  error
}

type vpcepServiceDeleteResult struct {
	resp *model.DeleteEndpointServiceResponse
	err  error
}

type vpcepServiceUpdateResult struct {
	resp *model.UpdateEndpointServiceResponse
	err  error
}

type vpcepServiceListResult struct {
	resp *model.ListServiceDetailsResponse
	err  error
}

type vpcepServiceAddPermissionsResult struct {
	resp *model.BatchAddEndpointServicePermissionsResponse
	err  error
}

type vpcepServiceRemovePermissionsResult struct {
	resp *model.BatchRemoveEndpointServicePermissionsResponse
	err  error
}

type vpcepServiceListPermissionsResult struct {
	resp *model.ListServicePermissionsDetailsResponse
	err  error
}

func (f *mockVpcepServiceClient) CreateEndpointService(req *model.CreateEndpointServiceRequest) (
	*model.CreateEndpointServiceResponse, error) {
	f.createVpcepServiceReq = req
	f.createCalls++
	if len(f.createResults) == 0 {
		return nil, errors.New("mock CreateEndpointService result is exhausted")
	}
	result := f.createResults[0]
	f.createResults = f.createResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) DeleteEndpointService(req *model.DeleteEndpointServiceRequest) (
	*model.DeleteEndpointServiceResponse, error) {
	f.deleteVpcepServiceReq = req
	f.deleteCalls++
	if len(f.deleteResults) == 0 {
		return nil, errors.New("mock DeleteEndpointService result is exhausted")
	}
	result := f.deleteResults[0]
	f.deleteResults = f.deleteResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) UpdateEndpointService(req *model.UpdateEndpointServiceRequest) (
	*model.UpdateEndpointServiceResponse, error) {
	f.updateVpcepServiceReq = req
	f.updateCalls++
	if len(f.updateResults) == 0 {
		return nil, errors.New("mock UpdateEndpointService result is exhausted")
	}
	result := f.updateResults[0]
	f.updateResults = f.updateResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) ListServiceDetails(req *model.ListServiceDetailsRequest) (
	*model.ListServiceDetailsResponse, error) {
	f.listVpcepServiceDetailsReqs = append(f.listVpcepServiceDetailsReqs, req)
	f.listCalls++
	if len(f.listResults) == 0 {
		return nil, errors.New("mock ListServiceDetails result is exhausted")
	}
	result := f.listResults[0]
	f.listResults = f.listResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) BatchAddEndpointServicePermissions(
	req *model.BatchAddEndpointServicePermissionsRequest) (*model.BatchAddEndpointServicePermissionsResponse, error) {
	f.addPermissionsReq = req
	f.addPermissionsCalls++
	if len(f.addPermissionsResults) == 0 {
		return nil, errors.New("mock BatchAddEndpointServicePermissions result is exhausted")
	}
	result := f.addPermissionsResults[0]
	f.addPermissionsResults = f.addPermissionsResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) BatchRemoveEndpointServicePermissions(
	req *model.BatchRemoveEndpointServicePermissionsRequest) (*model.BatchRemoveEndpointServicePermissionsResponse,
	error) {
	f.removePermissionsReq = req
	f.removePermissionsCalls++
	if len(f.removePermissionsResults) == 0 {
		return nil, errors.New("mock BatchRemoveEndpointServicePermissions result is exhausted")
	}
	result := f.removePermissionsResults[0]
	f.removePermissionsResults = f.removePermissionsResults[1:]
	return result.resp, result.err
}

func (f *mockVpcepServiceClient) ListServicePermissionsDetails(
	req *model.ListServicePermissionsDetailsRequest) (*model.ListServicePermissionsDetailsResponse, error) {
	f.listPermissionsDetailsReq = req
	f.listPermissionsDetailsCalls++
	if len(f.listPermissionsResults) == 0 {
		return nil, errors.New("mock ListServicePermissionsDetails result is exhausted")
	}
	result := f.listPermissionsResults[0]
	f.listPermissionsResults = f.listPermissionsResults[1:]
	return result.resp, result.err
}

func buildVpcepServiceInput() VpcepServiceInput {
	return VpcepServiceInput{
		VpcId:      testVpcId,
		PortId:     testVpcepServicePortId,
		ServerType: "VM",
		Ports: []PortPair{{
			ClientPort: testVpcepServiceClientPort,
			ServerPort: testVpcepServiceServerPort,
		}},
	}
}

func buildCreateEndpointServiceResponse(serviceId, status string) *model.CreateEndpointServiceResponse {
	return &model.CreateEndpointServiceResponse{Id: ptr(serviceId), Status: ptr(status)}
}

func buildListServiceDetailsResponse(status string) *model.ListServiceDetailsResponse {
	return &model.ListServiceDetailsResponse{
		Status:     ptr(status),
		ServerType: ptr("VM"),
		VpcId:      ptr(testVpcId),
		PortId:     ptr(testVpcepServicePortId),
		Ports: ptr([]model.PortList{{
			ClientPort: &testVpcepServiceClientPort,
			ServerPort: &testVpcepServiceServerPort,
			Protocol:   &testVpcepServiceTcpProtocol,
		}}),
	}
}

func buildUpdateEndpointServiceRequest() *model.UpdateEndpointServiceRequest {
	return &model.UpdateEndpointServiceRequest{
		VpcEndpointServiceId: testVpcepServiceId,
		Body: &model.UpdateEndpointServiceRequestBody{
			PortId: ptr(testVpcepServicePortId),
			Ports: ptr([]model.PortList{{
				ClientPort: &testVpcepServiceClientPort,
				ServerPort: &testVpcepServiceServerPort,
				Protocol:   &testVpcepServiceTcpProtocol,
			}}),
		},
	}
}

func buildPermissionsResponse(permissions []model.PermissionObject) *model.ListServicePermissionsDetailsResponse {
	return &model.ListServicePermissionsDetailsResponse{Permissions: &permissions}
}

func newFastPollingVpcepService(client VpcepServiceClient) *VpcepServiceService {
	service := NewVpcepServiceService(client)
	service.pollingInterval = time.Nanosecond
	service.pollingTimeout = time.Second
	service.retryBaseDelay = time.Nanosecond
	return service
}

func newTimeoutPollingVpcepService(client VpcepServiceClient) *VpcepServiceService {
	service := NewVpcepServiceService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Nanosecond
	return service
}

func newSlowPollingVpcepService(client VpcepServiceClient) *VpcepServiceService {
	service := NewVpcepServiceService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Hour
	return service
}
