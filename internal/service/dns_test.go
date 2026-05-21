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
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	"github.com/stretchr/testify/assert"
)

const (
	testZoneId      = "zone-1"
	testDomainName  = "example.com"
	testRouterId    = "vpc-1"
	testRecordName  = "test.example.com"
	testDnsRecordId = "dns-record-1"
)

var dnsNotFoundError = &sdkerr.ServiceResponseError{
	StatusCode: http.StatusNotFound,
	ErrorCode:  huaweiCloudDnsNotFoundErrorCode,
}

var recordSetNotFoundError = &sdkerr.ServiceResponseError{
	StatusCode: http.StatusNotFound,
	ErrorCode:  huaweiCloudRecordSetNotFoundErrorCode,
}

type mockDnsClient struct {
	createZoneCalls   int
	createZoneResults []dnsCreateZoneResult
	createZoneReqs    []*model.CreatePrivateZoneRequest

	showZoneCalls   int
	showZoneResults []dnsShowZoneResult
	showZoneReqs    []*model.ShowPrivateZoneRequest

	deleteZoneCalls   int
	deleteZoneResults []dnsDeleteZoneResult
	deleteZoneReqs    []*model.DeletePrivateZoneRequest

	createRecordSetCalls   int
	createRecordSetResults []dnsCreateRecordSetResult
	createRecordSetReqs    []*model.CreateRecordSetWithLineRequest

	showRecordSetCalls   int
	showRecordSetResults []dnsShowRecordSetResult
	showRecordSetReqs    []*model.ShowRecordSetWithLineRequest
}

type dnsCreateZoneResult struct {
	resp *model.CreatePrivateZoneResponse
	err  error
}

type dnsShowZoneResult struct {
	resp *model.ShowPrivateZoneResponse
	err  error
}

type dnsDeleteZoneResult struct {
	resp *model.DeletePrivateZoneResponse
	err  error
}

type dnsCreateRecordSetResult struct {
	resp *model.CreateRecordSetWithLineResponse
	err  error
}

type dnsShowRecordSetResult struct {
	resp *model.ShowRecordSetWithLineResponse
	err  error
}

func (f *mockDnsClient) CreatePrivateZone(req *model.CreatePrivateZoneRequest) (
	*model.CreatePrivateZoneResponse, error) {
	f.createZoneCalls++
	f.createZoneReqs = append(f.createZoneReqs, req)

	if len(f.createZoneResults) == 0 {
		return nil, nil
	}

	result := f.createZoneResults[0]
	f.createZoneResults = f.createZoneResults[1:]

	return result.resp, result.err
}

func (f *mockDnsClient) ShowPrivateZone(req *model.ShowPrivateZoneRequest) (
	*model.ShowPrivateZoneResponse, error) {
	f.showZoneCalls++
	f.showZoneReqs = append(f.showZoneReqs, req)

	if len(f.showZoneResults) == 0 {
		return nil, nil
	}

	result := f.showZoneResults[0]
	f.showZoneResults = f.showZoneResults[1:]

	return result.resp, result.err
}

func (f *mockDnsClient) DeletePrivateZone(req *model.DeletePrivateZoneRequest) (
	*model.DeletePrivateZoneResponse, error) {
	f.deleteZoneCalls++
	f.deleteZoneReqs = append(f.deleteZoneReqs, req)

	if len(f.deleteZoneResults) == 0 {
		return nil, nil
	}

	result := f.deleteZoneResults[0]
	f.deleteZoneResults = f.deleteZoneResults[1:]

	return result.resp, result.err
}

func (f *mockDnsClient) CreateRecordSetWithLine(req *model.CreateRecordSetWithLineRequest) (
	*model.CreateRecordSetWithLineResponse, error) {
	f.createRecordSetCalls++
	f.createRecordSetReqs = append(f.createRecordSetReqs, req)

	if len(f.createRecordSetResults) == 0 {
		return nil, nil
	}

	result := f.createRecordSetResults[0]
	f.createRecordSetResults = f.createRecordSetResults[1:]

	return result.resp, result.err
}

func (f *mockDnsClient) ShowRecordSetWithLine(req *model.ShowRecordSetWithLineRequest) (
	*model.ShowRecordSetWithLineResponse, error) {
	f.showRecordSetCalls++
	f.showRecordSetReqs = append(f.showRecordSetReqs, req)

	if len(f.showRecordSetResults) == 0 {
		return nil, nil
	}

	result := f.showRecordSetResults[0]
	f.showRecordSetResults = f.showRecordSetResults[1:]

	return result.resp, result.err
}

func TestNewDnsService(t *testing.T) {
	fake := &mockDnsClient{}

	actual := NewDnsService(fake)

	assert.NotNil(t, actual)
	assert.Equal(t, fake, actual.client)
	assert.Equal(t, pollingInterval, actual.pollingInterval)
	assert.Equal(t, pollingTimeout, actual.pollingTimeout)
}

func TestDnsService_CreatePrivateZone(t *testing.T) {
	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *DnsService
		expectedZoneId      string
		expectedErr         string
		expectedCreateCalls int
	}{
		{
			name: "GIVEN valid input WHEN CreatePrivateZone SHOULD return zone id",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{
					{resp: &model.CreatePrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_CREATE")}},
				},
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
			expectedZoneId:      testZoneId,
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api fails twice then succeeds WHEN CreatePrivateZone SHOULD return zone id",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
					{resp: &model.CreatePrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_CREATE")}},
				},
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
			expectedZoneId:      testZoneId,
			expectedCreateCalls: 3,
		},
		{
			name: "GIVEN create response without id WHEN CreatePrivateZone SHOULD return error",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{{resp: &model.CreatePrivateZoneResponse{}}},
			}),
			expectedErr:         "createPrivateZone response has no ID",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN nil create response WHEN CreatePrivateZone SHOULD return error",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{{resp: nil}},
			}),
			expectedErr:         "createPrivateZone returned nil response",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api keeps failing WHEN CreatePrivateZone SHOULD return last create error",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
				},
			}),
			expectedErr:         "createPrivateZone API failed after retries: create failed",
			expectedCreateCalls: 3,
		},
		{
			name: "GIVEN canceled context and create api error WHEN CreatePrivateZone SHOULD return context error",
			ctx:  canceledContext(),
			service: NewDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{{err: errors.New("create failed")}},
			}),
			expectedErr:         "createPrivateZone API failed after retries: context canceled",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN canceled context WHEN CreatePrivateZone waitForReady SHOULD return context error",
			ctx:  canceledContext(),
			service: newSlowPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{
					{resp: &model.CreatePrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_CREATE")}},
				},
			}),
			expectedErr:         fmt.Sprintf("wait for private zone ready failed: context cancelled while waiting for private zone %s: context canceled", testZoneId),
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN wait failed WHEN CreatePrivateZone SHOULD return wrapped wait error",
			service: newFastPollingDnsService(&mockDnsClient{
				createZoneResults: []dnsCreateZoneResult{
					{resp: &model.CreatePrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_CREATE")}},
				},
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ERROR")}},
				},
			}),
			expectedErr:         fmt.Sprintf("wait for private zone ready failed: private zone %s status is ERROR", testZoneId),
			expectedCreateCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actualZoneId, err := tc.service.CreatePrivateZone(ctx, DnsZoneInput{
				DomainName: testDomainName,
				RouterId:   testRouterId,
			})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedZoneId, actualZoneId)
			} else {
				assert.Empty(t, actualZoneId)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockDnsClient); ok {
				assert.Equal(t, tc.expectedCreateCalls, fake.createZoneCalls)
				if len(fake.createZoneReqs) > 0 {
					req := fake.createZoneReqs[0]
					assert.Equal(t, testDomainName, req.Body.Name)
					assert.Equal(t, dnsZoneType, req.Body.ZoneType)
					assert.Equal(t, testRouterId, req.Body.Router.RouterId)
				}
			}
		})
	}
}

func TestDnsService_waitForZoneReady(t *testing.T) {
	testCases := []struct {
		name        string
		ctx         context.Context
		service     *DnsService
		expectedErr string
	}{
		{
			name: "GIVEN active zone status WHEN waitForZoneReady SHOULD return nil",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
		},
		{
			name: "GIVEN pending then active zone status WHEN waitForZoneReady SHOULD return nil",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_CREATE")}},
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
		},
		{
			name: "GIVEN canceled context WHEN waitForZoneReady SHOULD return context error",
			ctx:  canceledContext(),
			service: newSlowPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
			expectedErr: fmt.Sprintf("context cancelled while waiting for private zone %s", testZoneId),
		},
		{
			name: "GIVEN timeout WHEN waitForZoneReady SHOULD return timeout error",
			service: newTimeoutPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: &model.ShowPrivateZoneResponse{
					Id: ptr(testZoneId), Status: ptr("PENDING_CREATE"),
				}}},
			}),
			expectedErr: fmt.Sprintf("timeout waiting for private zone %s to be ready", testZoneId),
		},
		{
			name: "GIVEN query errors beyond tolerance WHEN waitForZoneReady SHOULD return query error",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
					{err: errors.New("query failed")},
				},
			}),
			expectedErr: "query private zone status failed: query failed",
		},
		{
			name: "GIVEN response without status WHEN waitForZoneReady SHOULD return error",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId)}}},
			}),
			expectedErr: "private zone response has no status",
		},
		{
			name: "GIVEN error zone status WHEN waitForZoneReady SHOULD return error",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: &model.ShowPrivateZoneResponse{
					Id: ptr(testZoneId), Status: ptr("ERROR"),
				}}},
			}),
			expectedErr: fmt.Sprintf("private zone %s status is ERROR", testZoneId),
		},
		{
			name: "GIVEN pending_disable zone status WHEN waitForZoneReady SHOULD return error",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: &model.ShowPrivateZoneResponse{
					Id: ptr(testZoneId), Status: ptr("PENDING_DISABLE"),
				}}},
			}),
			expectedErr: fmt.Sprintf("private zone %s status is PENDING_DISABLE", testZoneId),
		},
		{
			name: "GIVEN pending_delete status WHEN waitForZoneReady SHOULD continue polling",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("PENDING_DELETE")}},
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
		},
		{
			name: "GIVEN unknown status THEN active WHEN waitForZoneReady SHOULD continue polling then succeed",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("UNKNOWN")}},
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.waitForZoneReady(ctx, testZoneId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}

func TestDnsService_CreateRecordSet(t *testing.T) {
	records := []string{"10.0.0.1", "10.0.0.2"}

	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *DnsService
		expectedRecordSetId string
		expectedErr         string
		expectedCreateCalls int
	}{
		{
			name: "GIVEN valid input WHEN CreateRecordSet SHOULD return record set id",
			service: NewDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{
					{resp: &model.CreateRecordSetWithLineResponse{Id: ptr(testDnsRecordId)}},
				},
			}),
			expectedRecordSetId: testDnsRecordId,
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api fails once then succeeds WHEN CreateRecordSet SHOULD return record set id",
			service: newFastPollingDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{
					{err: errors.New("create failed")},
					{resp: &model.CreateRecordSetWithLineResponse{Id: ptr(testDnsRecordId)}},
				},
			}),
			expectedRecordSetId: testDnsRecordId,
			expectedCreateCalls: 2,
		},
		{
			name: "GIVEN nil create response WHEN CreateRecordSet SHOULD return error",
			service: NewDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{{resp: nil}},
			}),
			expectedErr:         "createRecordSetWithLine returned nil response",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create response without id WHEN CreateRecordSet SHOULD return error",
			service: NewDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{{resp: &model.CreateRecordSetWithLineResponse{}}},
			}),
			expectedErr:         "createRecordSetWithLine response has no ID",
			expectedCreateCalls: 1,
		},
		{
			name: "GIVEN create api keeps failing WHEN CreateRecordSet SHOULD return last create error",
			service: newFastPollingDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
					{err: errors.New("create failed")},
				},
			}),
			expectedErr:         "createRecordSetWithLine API failed after retries: create failed",
			expectedCreateCalls: 3,
		},
		{
			name: "GIVEN canceled context and create api error WHEN CreateRecordSet SHOULD return context error",
			ctx:  canceledContext(),
			service: NewDnsService(&mockDnsClient{
				createRecordSetResults: []dnsCreateRecordSetResult{{err: errors.New("create failed")}},
			}),
			expectedErr:         "createRecordSetWithLine API failed after retries: context canceled",
			expectedCreateCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actualId, err := tc.service.CreateRecordSet(ctx, DnsRecordSetInput{
				ZoneId:  testZoneId,
				Name:    testRecordName,
				Records: records,
			})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedRecordSetId, actualId)
			} else {
				assert.Empty(t, actualId)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockDnsClient); ok {
				assert.Equal(t, tc.expectedCreateCalls, fake.createRecordSetCalls)
				if len(fake.createRecordSetReqs) > 0 {
					req := fake.createRecordSetReqs[0]
					assert.Equal(t, testZoneId, req.ZoneId)
					assert.Equal(t, testRecordName, req.Body.Name)
					assert.Equal(t, dnsRecordSetType, req.Body.Type)
					assert.Equal(t, &records, req.Body.Records)
				}
			}
		})
	}
}

func TestDnsService_DeletePrivateZone(t *testing.T) {
	testCases := []struct {
		name                string
		ctx                 context.Context
		service             *DnsService
		expectedErr         string
		expectedDeleteCalls int
	}{
		{
			name: "GIVEN delete api succeeds WHEN DeletePrivateZone SHOULD return nil",
			service: NewDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{{resp: &model.DeletePrivateZoneResponse{}}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN zone not found WHEN DeletePrivateZone SHOULD return nil",
			service: NewDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{{err: dnsNotFoundError}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN record set not found error WHEN DeletePrivateZone SHOULD return nil",
			service: NewDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{{err: recordSetNotFoundError}},
			}),
			expectedDeleteCalls: 1,
		},
		{
			name: "GIVEN delete api fails once then succeeds WHEN DeletePrivateZone SHOULD return nil",
			service: newFastPollingDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{
					{err: errors.New("delete failed")},
					{resp: &model.DeletePrivateZoneResponse{}},
				},
			}),
			expectedDeleteCalls: 2,
		},
		{
			name: "GIVEN canceled context and delete api error WHEN DeletePrivateZone SHOULD return context error",
			ctx:  canceledContext(),
			service: NewDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{{err: errors.New("delete failed")}},
			}),
			expectedDeleteCalls: 1,
			expectedErr:         "context canceled",
		},
		{
			name: "GIVEN delete api keeps failing WHEN DeletePrivateZone SHOULD return last delete error",
			service: newFastPollingDnsService(&mockDnsClient{
				deleteZoneResults: []dnsDeleteZoneResult{
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

			err := tc.service.DeletePrivateZone(ctx, testZoneId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockDnsClient); ok {
				assert.Equal(t, tc.expectedDeleteCalls, fake.deleteZoneCalls)
			}
		})
	}
}

func TestDnsService_GetPrivateZone(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		service           *DnsService
		expected          *DnsZoneOutput
		expectedListCalls int
		expectedErr       string
	}{
		{
			name: "GIVEN zone exists WHEN GetPrivateZone SHOULD return zone output",
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
			expected: &DnsZoneOutput{
				ZoneId: testZoneId,
				Status: "ACTIVE",
			},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN zone exists without status WHEN GetPrivateZone SHOULD return zone output with empty status",
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId)}},
				},
			}),
			expected: &DnsZoneOutput{
				ZoneId: testZoneId,
				Status: "",
			},
			expectedListCalls: 1,
		},
		{
			name: "GIVEN zone not found WHEN GetPrivateZone SHOULD return nil output",
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{err: dnsNotFoundError}},
			}),
			expected:          nil,
			expectedListCalls: 1,
		},
		{
			name: "GIVEN response with nil id WHEN GetPrivateZone SHOULD return nil output",
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: &model.ShowPrivateZoneResponse{}}},
			}),
			expected:          nil,
			expectedListCalls: 1,
		},
		{
			name: "GIVEN nil response WHEN GetPrivateZone SHOULD return nil output",
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{resp: nil}},
			}),
			expected:          nil,
			expectedListCalls: 1,
		},
		{
			name: "GIVEN query api fails once then succeeds WHEN GetPrivateZone SHOULD return zone output",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
					{err: errors.New("query failed")},
					{resp: &model.ShowPrivateZoneResponse{Id: ptr(testZoneId), Status: ptr("ACTIVE")}},
				},
			}),
			expected: &DnsZoneOutput{
				ZoneId: testZoneId,
				Status: "ACTIVE",
			},
			expectedListCalls: 2,
		},
		{
			name: "GIVEN query api error WHEN GetPrivateZone SHOULD return error",
			ctx:  canceledContext(),
			service: NewDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{{err: errors.New("query failed")}},
			}),
			expectedListCalls: 1,
			expectedErr:       "context canceled",
		},
		{
			name: "GIVEN query api keeps failing WHEN GetPrivateZone SHOULD return last query error",
			service: newFastPollingDnsService(&mockDnsClient{
				showZoneResults: []dnsShowZoneResult{
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

			actual, err := tc.service.GetPrivateZone(ctx, testZoneId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.Contains(t, err.Error(), tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*mockDnsClient); ok {
				assert.Equal(t, tc.expectedListCalls, fake.showZoneCalls)
			}
		})
	}
}

func newFastPollingDnsService(client DnsServiceClient) *DnsService {
	service := NewDnsService(client)
	service.pollingInterval = time.Nanosecond
	service.pollingTimeout = time.Second
	return service
}

func newTimeoutPollingDnsService(client DnsServiceClient) *DnsService {
	service := NewDnsService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Nanosecond
	return service
}

func newSlowPollingDnsService(client DnsServiceClient) *DnsService {
	service := NewDnsService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Hour
	return service
}
