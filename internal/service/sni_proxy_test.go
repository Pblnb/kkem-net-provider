/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
)

func TestNewSniProxyService(t *testing.T) {
	testCases := []struct {
		name   string
		client sniproxyclient.SniProxyClient
	}{
		{
			name:   "GIVEN nil client WHEN NewSniProxyService SHOULD create service with nil client",
			client: nil,
		},
		{
			name:   "GIVEN mock client WHEN NewSniProxyService SHOULD create service with client",
			client: &mockSniProxyClient{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := NewSniProxyService(tc.client)

			expected := &SniProxyService{
				client:          tc.client,
				pollingInterval: pollingInterval,
				pollingTimeout:  pollingTimeout,
			}

			assert.Equal(t, expected, actual)
		})
	}
}

func TestSniProxyService_AccessSniProxy(t *testing.T) {
	ctx := context.Background()
	testInput := AccessSniProxyInput{
		RegionCode:       testSniProxyRegionCode,
		ServiceName:      testSniProxyServiceName,
		IamDomainAccount: []string{testSniProxyIamDomainAccount},
	}

	testCases := []struct {
		name            string
		clientNil       bool
		accessResult    accessResult
		getResult       *getResult
		pollingTimeout  time.Duration
		pollingInterval time.Duration
		expectedRes     string
		expectedErrMsg  string
	}{
		{
			name:           "GIVEN nil client WHEN AccessSniProxy SHOULD return error",
			clientNil:      true,
			expectedErrMsg: "sni proxy client is not initialized",
		},
		{
			name: "GIVEN success WHEN AccessSniProxy SHOULD return resource ID",
			accessResult: accessResult{resp: &sniproxyclient.AccessServiceResponse{
				HTTPStatusCode: 200,
				Body: sniproxyclient.AccessServiceResponseBody{
					BaseResponse: sniproxyclient.BaseResponse{Code: 0},
					Data:         sniproxyclient.AccessServiceResponseData{ResourceId: testSniProxyResourceId},
				},
			}},
			getResult: &getResult{resp: &sniproxyclient.GetAccessServiceResponse{
				HTTPStatusCode: 200,
				Body: sniproxyclient.GetAccessServiceResponseBody{
					BaseResponse: sniproxyclient.BaseResponse{Code: 0},
					Data:         sniproxyclient.AccessServiceResponseData{ResourceId: testSniProxyResourceId},
				},
			}},
			expectedRes: testSniProxyResourceId,
		},
		{
			name:           "GIVEN retry failure WHEN AccessSniProxy SHOULD return retry error",
			accessResult:   accessResult{err: errors.New("network timeout")},
			expectedErrMsg: "access SNI Proxy service failed after retries",
		},
		{
			name:           "GIVEN HTTP error WHEN AccessSniProxy SHOULD return HTTP error",
			accessResult:   accessResult{resp: &sniproxyclient.AccessServiceResponse{HTTPStatusCode: 500}},
			expectedErrMsg: "httpStatusCode=500",
		},
		{
			name: "GIVEN business error WHEN AccessSniProxy SHOULD return business error",
			accessResult: accessResult{resp: &sniproxyclient.AccessServiceResponse{
				Body:           sniproxyclient.AccessServiceResponseBody{BaseResponse: sniproxyclient.BaseResponse{Code: 1, Msg: "business error"}},
				HTTPStatusCode: 200,
			}},
			expectedErrMsg: "code=1",
		},
		{
			name: "GIVEN empty resource ID WHEN AccessSniProxy SHOULD return validation error",
			accessResult: accessResult{resp: &sniproxyclient.AccessServiceResponse{
				HTTPStatusCode: 200,
			}},
			expectedErrMsg: "response has no resource_id",
		},
		{
			name: "GIVEN access success but polling failure WHEN AccessSniProxy SHOULD return wait failed error",
			accessResult: accessResult{resp: &sniproxyclient.AccessServiceResponse{
				HTTPStatusCode: 200,
				Body: sniproxyclient.AccessServiceResponseBody{
					BaseResponse: sniproxyclient.BaseResponse{Code: 0},
					Data:         sniproxyclient.AccessServiceResponseData{ResourceId: "res-wait-fail"},
				},
			}},
			getResult:       &getResult{err: errors.New("polling query failed")},
			pollingTimeout:  15 * time.Millisecond,
			pollingInterval: 60 * time.Millisecond,
			expectedErrMsg:  "wait for SNI Proxy access ready failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mockClient *mockSniProxyClient
			if !tc.clientNil {
				mockClient = newMockSniProxyClient()
				mockClient.addAccessResult(tc.accessResult)
				if tc.getResult != nil {
					mockClient.addGetResult(*tc.getResult)
				}
			}

			var service *SniProxyService
			if tc.clientNil {
				service = NewSniProxyService(nil)
			} else if tc.pollingTimeout > 0 && tc.pollingInterval > 0 {
				service = newMockSniProxyService(mockClient, tc.pollingTimeout, tc.pollingInterval)
			} else if tc.getResult != nil {
				service = newMockSniProxyService(mockClient, 300*time.Millisecond, 20*time.Millisecond)
			} else {
				service = NewSniProxyService(mockClient)
			}

			if !tc.clientNil {
				patchRetryWithBackoff(t)
			}

			actualRes, actualErr := service.AccessSniProxy(ctx, testInput)

			if tc.expectedErrMsg != "" {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tc.expectedErrMsg)
			} else {
				assert.Nil(t, actualErr)
				assert.Equal(t, tc.expectedRes, actualRes)
			}
		})
	}
}

func TestSniProxyService_DeleteSniProxy(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name           string
		resourceId     string
		clientNil      bool
		deleteResult   deleteResult
		expectedErrMsg string
	}{
		{
			name:       "GIVEN empty resourceId WHEN DeleteSniProxy SHOULD return nil early",
			resourceId: "",
		},
		{
			name:           "GIVEN nil client WHEN DeleteSniProxy SHOULD return error",
			resourceId:     testSniProxyResourceId,
			clientNil:      true,
			expectedErrMsg: "sni proxy client is not initialized",
		},
		{
			name:       "GIVEN successful delete WHEN DeleteSniProxy SHOULD return nil",
			resourceId: testSniProxyResourceId,
			deleteResult: deleteResult{resp: &sniproxyclient.DeleteAccessServiceResponse{
				HTTPStatusCode: 200,
				Body:           sniproxyclient.BaseResponse{Code: 0},
			}},
		},
		{
			name:           "GIVEN retry failure WHEN DeleteSniProxy SHOULD return error",
			resourceId:     testSniProxyResourceId,
			deleteResult:   deleteResult{err: errors.New("network timeout")},
			expectedErrMsg: "call DeleteAccessService API failed",
		},
		{
			name:           "GIVEN nil response WHEN DeleteSniProxy SHOULD return error",
			resourceId:     testSniProxyResourceId,
			deleteResult:   deleteResult{resp: nil},
			expectedErrMsg: "response is nil",
		},
		{
			name:       "GIVEN HTTP error WHEN DeleteSniProxy SHOULD return HTTP error",
			resourceId: testSniProxyResourceId,
			deleteResult: deleteResult{resp: &sniproxyclient.DeleteAccessServiceResponse{
				HTTPStatusCode: 500,
			}},
			expectedErrMsg: "httpStatusCode=500",
		},
		{
			name:       "GIVEN business error WHEN DeleteSniProxy SHOULD return business error",
			resourceId: testSniProxyResourceId,
			deleteResult: deleteResult{resp: &sniproxyclient.DeleteAccessServiceResponse{
				HTTPStatusCode: 200,
				Body:           sniproxyclient.BaseResponse{Code: 1, Msg: "business error"},
			}},
			expectedErrMsg: "code=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mockClient *mockSniProxyClient
			if !tc.clientNil {
				mockClient = newMockSniProxyClient()
				mockClient.addDeleteResult(tc.deleteResult)
			}

			var service *SniProxyService
			if mockClient != nil {
				service = NewSniProxyService(mockClient)
			} else {
				service = NewSniProxyService(nil)
			}

			if !tc.clientNil {
				patchRetryWithBackoff(t)
			}

			actualErr := service.DeleteSniProxy(ctx, tc.resourceId)

			if tc.expectedErrMsg != "" {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tc.expectedErrMsg)
			} else {
				assert.Nil(t, actualErr)
			}
		})
	}
}

func TestSniProxyService_checkAccessReady(t *testing.T) {
	testCases := []struct {
		name           string
		getResult      getResult
		expectedOutput bool
		expectedErrMsg string
	}{
		{
			name:           "GIVEN get error WHEN checkAccessReady SHOULD return error",
			getResult:      getResult{err: errors.New("network err")},
			expectedOutput: false,
			expectedErrMsg: "network err",
		},
		{
			name:           "GIVEN HTTP error WHEN checkAccessReady SHOULD return error",
			getResult:      getResult{resp: &sniproxyclient.GetAccessServiceResponse{HTTPStatusCode: 404}},
			expectedOutput: false,
			expectedErrMsg: "HTTP",
		},
		{
			name:           "GIVEN nil response when client returns no error WHEN checkAccessReady SHOULD return validation error",
			getResult:      getResult{resp: nil},
			expectedOutput: false,
			expectedErrMsg: "GetAccessService returned nil response with no error",
		},
		{
			name: "GIVEN not exist code WHEN checkAccessReady SHOULD return error",
			getResult: getResult{resp: &sniproxyclient.GetAccessServiceResponse{
				HTTPStatusCode: 200,
				Body:           sniproxyclient.GetAccessServiceResponseBody{BaseResponse: sniproxyclient.BaseResponse{Code: 6082, Msg: "Resource not found"}},
			}},
			expectedOutput: false,
			expectedErrMsg: "code=6082",
		},
		{
			name: "GIVEN success response WHEN checkAccessReady SHOULD return mapped output",
			getResult: getResult{resp: &sniproxyclient.GetAccessServiceResponse{
				HTTPStatusCode: 200,
				Body: sniproxyclient.GetAccessServiceResponseBody{
					BaseResponse: sniproxyclient.BaseResponse{Code: 0},
					Data: sniproxyclient.AccessServiceResponseData{
						ResourceId:       testSniProxyResourceId,
						ServiceName:      testSniProxyServiceName,
						AccessObject:     testSniProxyAccessObject,
						RegionCode:       testSniProxyRegionCode,
						IamDomainAccount: []string{testSniProxyIamDomainAccount},
						EpServiceIds:     []sniproxyclient.EpServiceInfo{{EpServiceId: testSniProxyEpServiceId}},
					},
				},
			}},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := newMockSniProxyClient()
			mockClient.addGetResult(tc.getResult)
			service := NewSniProxyService(mockClient)

			out, err := service.checkAccessReady(context.Background(), testSniProxyResourceId)

			if tc.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, out)
				assert.Equal(t, testSniProxyResourceId, out.ResourceId)
				assert.Equal(t, []string{testSniProxyEpServiceId}, out.EpServiceIds)
			}
		})
	}
}

func TestSniProxyService_waitForSniProxyAccessReady(t *testing.T) {
	testCases := []struct {
		name            string
		getResults      []getResult
		cancelAfterMs   int
		pollingTimeout  time.Duration
		pollingInterval time.Duration
		expectedErrMsg  string
	}{
		{
			name: "GIVEN immediate success WHEN waitForSniProxyAccessReady SHOULD return output",
			getResults: []getResult{
				{resp: &sniproxyclient.GetAccessServiceResponse{
					HTTPStatusCode: 200,
					Body:           sniproxyclient.GetAccessServiceResponseBody{BaseResponse: sniproxyclient.BaseResponse{Code: 0}, Data: sniproxyclient.AccessServiceResponseData{ResourceId: "ready-res"}},
				}},
			},
		},
		{
			name: "GIVEN poll success WHEN waitForSniProxyAccessReady SHOULD return output",
			getResults: []getResult{
				{err: errors.New("initial fail")},
				{resp: &sniproxyclient.GetAccessServiceResponse{
					HTTPStatusCode: 200,
					Body:           sniproxyclient.GetAccessServiceResponseBody{BaseResponse: sniproxyclient.BaseResponse{Code: 0}, Data: sniproxyclient.AccessServiceResponseData{ResourceId: "poll-res"}},
				}},
			},
		},
		{
			name:           "GIVEN context cancelled WHEN waitForSniProxyAccessReady SHOULD return error",
			getResults:     []getResult{{err: errors.New("fail")}, {err: errors.New("fail")}},
			cancelAfterMs:  50,
			expectedErrMsg: "context cancelled",
		},
		{
			name:            "GIVEN timeout WHEN waitForSniProxyAccessReady SHOULD return timeout error",
			getResults:      []getResult{{err: errors.New("fail")}}, // Only one fail result
			pollingTimeout:  15 * time.Millisecond,                  // Shorter than interval
			pollingInterval: 60 * time.Millisecond,                  // Much longer than timeout
			expectedErrMsg:  "timeout",
		},
		{
			name: "GIVEN err count exceeded WHEN waitForSniProxyAccessReady SHOULD return query error",
			getResults: []getResult{
				{err: errors.New("err 0")},
				{err: errors.New("err 1")},
				{err: errors.New("err 2")},
				{err: errors.New("err 3")},
			},
			expectedErrMsg: "query SNI Proxy access status failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := newMockSniProxyClient()
			for _, r := range tc.getResults {
				mockClient.addGetResult(r)
			}

			var service *SniProxyService
			timeout := 300 * time.Millisecond
			interval := 20 * time.Millisecond

			if tc.pollingTimeout > 0 {
				timeout = tc.pollingTimeout
				interval = tc.pollingInterval
			}
			service = newMockSniProxyService(mockClient, timeout, interval)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			var out *AccessSniProxyOutput
			var err error

			go func() {
				defer close(done)
				out, err = service.waitForSniProxyAccessReady(ctx, testSniProxyResourceId)
			}()

			if tc.cancelAfterMs > 0 {
				time.Sleep(time.Duration(tc.cancelAfterMs) * time.Millisecond)
				cancel()
			}

			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Test timed out")
			}

			if tc.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, out)
			}
		})
	}
}

func TestSniProxyService_GetSniProxy(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name           string
		resourceId     string
		clientNil      bool
		getResult      getResult
		expectedOutput bool
		expectedResp   bool
		expectedErrMsg string
	}{
		{
			name:           "GIVEN nil client WHEN GetSniProxy SHOULD return error",
			clientNil:      true,
			resourceId:     testSniProxyResourceId,
			expectedErrMsg: "sni proxy client is not initialized",
		},
		{
			name:       "GIVEN successful query WHEN GetSniProxy SHOULD return output",
			resourceId: testSniProxyResourceId,
			getResult: getResult{resp: &sniproxyclient.GetAccessServiceResponse{
				HTTPStatusCode: 200,
				Body: sniproxyclient.GetAccessServiceResponseBody{
					BaseResponse: sniproxyclient.BaseResponse{Code: 0},
					Data: sniproxyclient.AccessServiceResponseData{
						ResourceId:       testSniProxyResourceId,
						ServiceName:      testSniProxyServiceName,
						AccessObject:     testSniProxyAccessObject,
						RegionCode:       testSniProxyRegionCode,
						IamDomainAccount: []string{testSniProxyIamDomainAccount},
						EpServiceIds:     []sniproxyclient.EpServiceInfo{{EpServiceId: testSniProxyEpServiceId}},
					},
				},
			}},
			expectedOutput: true,
		},
		{
			name:           "GIVEN retry failure WHEN GetSniProxy SHOULD return error",
			resourceId:     testSniProxyResourceId,
			getResult:      getResult{err: errors.New("network timeout")},
			expectedErrMsg: "call GetAccessService API failed",
		},
		{
			name:           "GIVEN nil response WHEN GetSniProxy SHOULD return error",
			resourceId:     testSniProxyResourceId,
			getResult:      getResult{resp: nil},
			expectedErrMsg: "response is nil",
		},
		{
			name:           "GIVEN HTTP error WHEN GetSniProxy SHOULD return HTTP error",
			resourceId:     testSniProxyResourceId,
			getResult:      getResult{resp: &sniproxyclient.GetAccessServiceResponse{HTTPStatusCode: 404}},
			expectedErrMsg: "httpStatusCode=404",
		},
		{
			name:       "GIVEN not exist code WHEN GetSniProxy SHOULD return not exist error",
			resourceId: testSniProxyResourceId,
			getResult: getResult{resp: &sniproxyclient.GetAccessServiceResponse{
				HTTPStatusCode: 200,
				Body:           sniproxyclient.GetAccessServiceResponseBody{BaseResponse: sniproxyclient.BaseResponse{Code: 6082, Msg: "not exist"}},
			}},
			expectedResp:   true,
			expectedErrMsg: "code=6082",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mockClient *mockSniProxyClient
			if !tc.clientNil {
				mockClient = newMockSniProxyClient()
				mockClient.addGetResult(tc.getResult)
			}

			var service *SniProxyService
			if mockClient != nil {
				service = NewSniProxyService(mockClient)
			} else {
				service = NewSniProxyService(nil)
			}

			if !tc.clientNil {
				patchRetryWithBackoff(t)
			}

			out, resp, err := service.GetSniProxy(ctx, tc.resourceId)

			if tc.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				assert.Nil(t, err)
			}

			if tc.expectedOutput {
				assert.NotNil(t, out)
				assert.Equal(t, testSniProxyResourceId, out.ResourceId)
				if len(out.EpServiceIds) > 0 {
					assert.Equal(t, []string{testSniProxyEpServiceId}, out.EpServiceIds)
				}
			} else {
				assert.Nil(t, out)
			}

			if tc.expectedResp {
				assert.NotNil(t, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func patchRetryWithBackoff(t *testing.T) {
	patches := gomonkey.ApplyFunc(retryWithBackoff, func(_ context.Context, _ int, _ time.Duration, op func() error) error {
		return op()
	})
	t.Cleanup(patches.Reset)
}

type mockSniProxyClient struct {
	mu            sync.Mutex
	accessResults []accessResult
	getResults    []getResult
	deleteResults []deleteResult
}

func newMockSniProxyClient() *mockSniProxyClient {
	return &mockSniProxyClient{}
}

func (m *mockSniProxyClient) addAccessResult(r accessResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessResults = append(m.accessResults, r)
}

func (m *mockSniProxyClient) addGetResult(r getResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getResults = append(m.getResults, r)
}

func (m *mockSniProxyClient) addDeleteResult(r deleteResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteResults = append(m.deleteResults, r)
}

func (m *mockSniProxyClient) AccessService(ctx context.Context, req sniproxyclient.AccessServiceRequest) (*sniproxyclient.AccessServiceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.accessResults) > 0 {
		res := m.accessResults[0]
		m.accessResults = m.accessResults[1:]
		return res.resp, res.err
	}
	return &sniproxyclient.AccessServiceResponse{}, nil
}

func (m *mockSniProxyClient) GetAccessService(ctx context.Context, resourceId string) (*sniproxyclient.GetAccessServiceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.getResults) > 0 {
		res := m.getResults[0]
		m.getResults = m.getResults[1:]
		return res.resp, res.err
	}
	return nil, errors.New("mock out of results")
}

func (m *mockSniProxyClient) DeleteAccessService(ctx context.Context, resourceId string) (*sniproxyclient.DeleteAccessServiceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.deleteResults) > 0 {
		res := m.deleteResults[0]
		m.deleteResults = m.deleteResults[1:]
		return res.resp, res.err
	}
	return &sniproxyclient.DeleteAccessServiceResponse{}, nil
}

type accessResult struct {
	resp *sniproxyclient.AccessServiceResponse
	err  error
}

type getResult struct {
	resp *sniproxyclient.GetAccessServiceResponse
	err  error
}

type deleteResult struct {
	resp *sniproxyclient.DeleteAccessServiceResponse
	err  error
}

func newMockSniProxyService(client sniproxyclient.SniProxyClient, timeout time.Duration, interval time.Duration) *SniProxyService {
	return &SniProxyService{
		client:          client,
		pollingTimeout:  timeout,
		pollingInterval: interval,
	}
}
