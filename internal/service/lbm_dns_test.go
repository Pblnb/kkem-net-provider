/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"huawei.com/kkem/kkem-net-provider/internal/client/lbmdnsclient"
)

const (
	testLbmDnsRecordId = "dns-record-1"
	testEndpointIp     = "10.0.0.8"
)

func TestUpdateRecordValue(t *testing.T) {
	apiErr := errors.New("update failed")
	waitErr := errors.New("wait failed")
	patches := gomonkey.ApplyFunc((*LbmDnsService).waitForTaskCompleted,
		func(_ *LbmDnsService, _ context.Context,
			taskId, _ string) (*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, error) {
			if taskId == "task-fail" {
				return nil, waitErr
			}
			return buildTaskStatusResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
				lbmdnsclient.StatusCodeSuccess, ""), nil
		})
	defer patches.Reset()

	testCases := []struct {
		name         string
		ctx          context.Context
		service      *LbmDnsService
		expectedErr  string
		expectedCall bool
	}{
		{
			name: "GIVEN no changes response WHEN UpdateRecordValue SHOULD return nil",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeNoChanges,
					lbmdnsclient.StatusCodeNoChanges, "No changes detected", ""),
			}),
			expectedCall: true,
		},
		{
			name: "GIVEN successful response and completed task WHEN UpdateRecordValue SHOULD return nil",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-success"),
			}),
			expectedCall: true,
		},
		{
			name:        "GIVEN nil client WHEN UpdateRecordValue SHOULD return error",
			service:     NewLbmDnsService(nil),
			expectedErr: "m3 lbm-dns client is not initialized",
		},
		{
			name: "GIVEN nil response WHEN UpdateRecordValue SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: nil,
			}),
			expectedErr:  "response is nil for record dns-record-1",
			expectedCall: true,
		},
		{
			name: "GIVEN non-2xx http response WHEN UpdateRecordValue SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "update DNS record failed: httpStatusCode=500",
			expectedCall: true,
		},
		{
			name: "GIVEN unsuccessful business response WHEN UpdateRecordValue SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, 1, 1, "update failed", ""),
			}),
			expectedErr:  "update DNS record failed: status=1, code=1, errMsg=update failed",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response without task id WHEN UpdateRecordValue SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "update lbm-dns record response has no task id",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response and failed task wait WHEN UpdateRecordValue SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-fail"),
			}),
			expectedErr:  "wait failed",
			expectedCall: true,
		},
		{
			name: "GIVEN canceled context and update api error WHEN UpdateRecordValue SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewLbmDnsService(&lbmDnsClientFake{
				updateErr: apiErr,
			}),
			expectedErr:  "call UpdateIntranetDnsDomain API failed: context canceled",
			expectedCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.UpdateRecordValue(ctx, testLbmDnsRecordId, testEndpointIp)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok && tc.expectedCall {
				assert.Equal(t, testLbmDnsRecordId, fake.updateRecordId)
				assert.Equal(t, testEndpointIp, fake.updateIp)
			}
		})
	}
}

func TestIsLbmDnsNoChanges(t *testing.T) {
	testCases := []struct {
		name     string
		status   int
		code     int
		msg      string
		expected bool
	}{
		{
			name:     "GIVEN no changes status code and message WHEN isLbmDnsNoChanges SHOULD return true",
			status:   lbmdnsclient.StatusCodeNoChanges,
			code:     lbmdnsclient.StatusCodeNoChanges,
			msg:      "No changes detected",
			expected: true,
		},
		{
			name:     "GIVEN success status WHEN isLbmDnsNoChanges SHOULD return false",
			status:   lbmdnsclient.StatusCodeSuccess,
			code:     lbmdnsclient.StatusCodeNoChanges,
			msg:      "No changes detected",
			expected: false,
		},
		{
			name:     "GIVEN success code WHEN isLbmDnsNoChanges SHOULD return false",
			status:   lbmdnsclient.StatusCodeNoChanges,
			code:     lbmdnsclient.StatusCodeSuccess,
			msg:      "No changes detected",
			expected: false,
		},
		{
			name:     "GIVEN changed message WHEN isLbmDnsNoChanges SHOULD return false",
			status:   lbmdnsclient.StatusCodeNoChanges,
			code:     lbmdnsclient.StatusCodeNoChanges,
			msg:      "record updated",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isLbmDnsNoChanges(tc.status, tc.code, tc.msg)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetLbmDnsRawResponse(t *testing.T) {
	resource := buildLbmDnsResource()
	apiErr := errors.New("query failed")
	testCases := []struct {
		name             string
		ctx              context.Context
		service          *LbmDnsService
		expectedResource *lbmdnsclient.IntranetDnsDomainResource
		expectedErr      string
		expectedCall     bool
	}{
		{
			name: "GIVEN successful response WHEN getLbmDnsRawResponse SHOULD return resource",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", resource),
			}),
			expectedResource: resource,
			expectedCall:     true,
		},
		{
			name: "GIVEN not found response WHEN getLbmDnsRawResponse SHOULD return nil resource",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeResourceNotFound, "not found", nil),
			}),
			expectedResource: nil,
			expectedCall:     true,
		},
		{
			name:        "GIVEN nil client WHEN getLbmDnsRawResponse SHOULD return error",
			service:     NewLbmDnsService(nil),
			expectedErr: "m3 lbm-dns client is not initialized",
		},
		{
			name: "GIVEN nil response WHEN getLbmDnsRawResponse SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: nil,
			}),
			expectedErr:  "response is nil for record dns-record-1",
			expectedCall: true,
		},
		{
			name: "GIVEN non-2xx http response WHEN getLbmDnsRawResponse SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", nil),
			}),
			expectedErr:  "query DNS record failed: httpStatusCode=500",
			expectedCall: true,
		},
		{
			name: "GIVEN unsuccessful business response WHEN getLbmDnsRawResponse SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, 1, 1, "query failed", nil),
			}),
			expectedErr:  "query DNS record failed: status=1, code=1, errMsg=query failed",
			expectedCall: true,
		},
		{
			name: "GIVEN canceled context and query api error WHEN getLbmDnsRawResponse SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewLbmDnsService(&lbmDnsClientFake{
				getErr: apiErr,
			}),
			expectedErr:  "call GetIntranetDnsDomain API failed: context canceled",
			expectedCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.getLbmDnsRawResponse(ctx, testLbmDnsRecordId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResource, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok && tc.expectedCall {
				assert.Equal(t, testLbmDnsRecordId, fake.getRecordId)
			}
		})
	}
}

func TestExtractLbmDnsRecordValues(t *testing.T) {
	testCases := []struct {
		name          string
		data          *lbmdnsclient.IntranetDnsDomainResource
		expected      []LbmDnsRecordValue
		expectedPanic bool
	}{
		{
			name: "GIVEN record values WHEN extractLbmDnsRecordValues SHOULD return converted record values",
			data: buildLbmDnsResourceWithValues([]lbmdnsclient.IntranetDnsRecordValue{
				{RecordType: "A", RecordValue: testEndpointIp},
				{RecordType: "AAAA", RecordValue: "::1"},
			}),
			expected: []LbmDnsRecordValue{
				{RecordType: "A", RecordValue: testEndpointIp},
				{RecordType: "AAAA", RecordValue: "::1"},
			},
		},
		{
			name:     "GIVEN empty record values WHEN extractLbmDnsRecordValues SHOULD return empty list",
			data:     buildLbmDnsResourceWithValues([]lbmdnsclient.IntranetDnsRecordValue{}),
			expected: []LbmDnsRecordValue{},
		},
		{
			name:          "GIVEN nil resource WHEN extractLbmDnsRecordValues SHOULD panic",
			data:          nil,
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedPanic {
				assert.Panics(t, func() {
					extractLbmDnsRecordValues(tc.data)
				})
				return
			}

			actual := extractLbmDnsRecordValues(tc.data)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetRecord(t *testing.T) {
	testCases := []struct {
		name        string
		service     *LbmDnsService
		expected    *CreateLbmDnsOutput
		expectedErr string
	}{
		{
			name: "GIVEN successful response WHEN GetRecord SHOULD return dns record output",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", buildLbmDnsResource()),
			}),
			expected: &CreateLbmDnsOutput{
				RecordId: testLbmDnsRecordId,
				RecordValues: []LbmDnsRecordValue{
					{RecordType: "A", RecordValue: testEndpointIp},
				},
			},
		},
		{
			name: "GIVEN not found response WHEN GetRecord SHOULD return nil output",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeResourceNotFound, "not found", nil),
			}),
			expected: nil,
		},
		{
			name: "GIVEN invalid response WHEN GetRecord SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", nil),
			}),
			expectedErr: "query DNS record failed: httpStatusCode=500",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.service.GetRecord(context.Background(), testLbmDnsRecordId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok {
				assert.Equal(t, testLbmDnsRecordId, fake.getRecordId)
			}
		})
	}
}

func TestGetLbmDnsDetail(t *testing.T) {
	testCases := []struct {
		name        string
		service     *LbmDnsService
		expected    *LbmDnsDetailOutput
		expectedErr string
	}{
		{
			name: "GIVEN successful response WHEN GetLbmDnsDetail SHOULD return dns detail output",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", buildLbmDnsResource()),
			}),
			expected: &LbmDnsDetailOutput{
				RecordId:     testLbmDnsRecordId,
				RegionCode:   "cn-north-4",
				ServiceName:  "service-1",
				HostRecord:   "api",
				DomainSuffix: "example.com",
				RecordValues: []LbmDnsRecordValue{
					{RecordType: "A", RecordValue: testEndpointIp},
				},
			},
		},
		{
			name: "GIVEN not found response WHEN GetLbmDnsDetail SHOULD return nil output",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeResourceNotFound, "not found", nil),
			}),
			expected: nil,
		},
		{
			name: "GIVEN invalid response WHEN GetLbmDnsDetail SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				getResp: buildGetRecordResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", nil),
			}),
			expectedErr: "query DNS record failed: httpStatusCode=500",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.service.GetLbmDnsDetail(context.Background(), testLbmDnsRecordId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok {
				assert.Equal(t, testLbmDnsRecordId, fake.getRecordId)
			}
		})
	}
}

type lbmDnsClientFake struct {
	updateResp     *lbmdnsclient.AsyncTaskResponse
	updateErr      error
	getResp        *lbmdnsclient.GetIntranetDnsDomainResponse
	getErr         error
	updateRecordId string
	updateIp       string
	getRecordId    string
}

func (f *lbmDnsClientFake) CreateIntranetDnsDomain(_ context.Context,
	_, _, _, _, _ string) (*lbmdnsclient.AsyncTaskResponse,
	error) {
	return nil, nil
}

func (f *lbmDnsClientFake) GetIntranetDnsDomainTaskStatus(_ context.Context,
	_ string) (*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, error) {
	return nil, nil
}

func (f *lbmDnsClientFake) GetIntranetDnsDomain(_ context.Context,
	resourceId string) (*lbmdnsclient.GetIntranetDnsDomainResponse, error) {
	f.getRecordId = resourceId
	return f.getResp, f.getErr
}

func (f *lbmDnsClientFake) UpdateIntranetDnsDomain(_ context.Context,
	resourceId, ip string) (*lbmdnsclient.AsyncTaskResponse, error) {
	f.updateRecordId = resourceId
	f.updateIp = ip
	return f.updateResp, f.updateErr
}

func (f *lbmDnsClientFake) DeleteIntranetDnsDomain(_ context.Context,
	_ string) (*lbmdnsclient.AsyncTaskResponse, error) {
	return nil, nil
}

func buildAsyncTaskResponse(httpStatusCode, status, code int, errMsg, taskId string) *lbmdnsclient.AsyncTaskResponse {
	body := lbmdnsclient.AsyncTaskResponseBody{TaskId: taskId}
	body.Status = status
	body.Code = code
	body.ErrMsg = errMsg
	return &lbmdnsclient.AsyncTaskResponse{HTTPStatusCode: httpStatusCode, Body: body}
}

func buildGetRecordResponse(httpStatusCode, status, code int, errMsg string,
	data *lbmdnsclient.IntranetDnsDomainResource) *lbmdnsclient.GetIntranetDnsDomainResponse {
	body := lbmdnsclient.GetIntranetDnsDomainResponseBody{Data: data}
	body.Status = status
	body.Code = code
	body.ErrMsg = errMsg
	return &lbmdnsclient.GetIntranetDnsDomainResponse{HTTPStatusCode: httpStatusCode, Body: body}
}

func buildTaskStatusResponse(httpStatusCode, status, code int,
	errMsg string) *lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse {
	body := lbmdnsclient.GetIntranetDnsDomainTaskStatusResponseBody{}
	body.Status = status
	body.Code = code
	body.ErrMsg = errMsg
	return &lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse{HTTPStatusCode: httpStatusCode, Body: body}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func buildLbmDnsResource() *lbmdnsclient.IntranetDnsDomainResource {
	return buildLbmDnsResourceWithValues([]lbmdnsclient.IntranetDnsRecordValue{
		{RecordType: "A", RecordValue: testEndpointIp},
	})
}

func buildLbmDnsResourceWithValues(values []lbmdnsclient.IntranetDnsRecordValue) *lbmdnsclient.IntranetDnsDomainResource {
	return &lbmdnsclient.IntranetDnsDomainResource{
		RegionCode:   "cn-north-4",
		ServiceName:  "service-1",
		HostRecord:   "api",
		DomainSuffix: "example.com",
		RecordValues: values,
	}
}
