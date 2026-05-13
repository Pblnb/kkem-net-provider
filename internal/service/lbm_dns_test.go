/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"huawei.com/kkem/kkem-net-provider/internal/client/lbmdnsclient"
)

const (
	testLbmDnsRecordId = "dns-record-1"
	testEndpointIp     = "10.0.0.8"
	testLbmDnsTaskId   = "task-1"
)

func TestNewLbmDnsService(t *testing.T) {
	fake := &lbmDnsClientFake{}

	actual := NewLbmDnsService(fake)

	assert.NotNil(t, actual)
	assert.Equal(t, fake, actual.client)
	assert.Equal(t, pollingInterval, actual.pollingInterval)
	assert.Equal(t, pollingTimeout, actual.pollingTimeout)
}

func TestLbmDnsService_CreateIntranetDnsDomain(t *testing.T) {
	apiErr := errors.New("create failed")
	testCases := []struct {
		name         string
		ctx          context.Context
		service      *LbmDnsService
		expected     *CreateLbmDnsOutput
		expectedErr  string
		expectedCall bool
	}{
		{
			name: "GIVEN successful response and ready record WHEN CreateIntranetDnsDomain SHOULD return dns output",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				createResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsTaskId),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
			}),
			expected: &CreateLbmDnsOutput{
				RecordId:     testLbmDnsRecordId,
				RecordValues: []LbmDnsRecordValue{{RecordType: "A", RecordValue: testEndpointIp}},
			},
			expectedCall: true,
		},
		{
			name:        "GIVEN nil client WHEN CreateIntranetDnsDomain SHOULD return error",
			service:     NewLbmDnsService(nil),
			expectedErr: "m3 lbm-dns client is not initialized",
		},
		{
			name: "GIVEN non-2xx http response WHEN CreateIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				createResp: buildAsyncTaskResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "create DNS record failed: httpStatusCode=500",
			expectedCall: true,
		},
		{
			name: "GIVEN unsuccessful business response WHEN CreateIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				createResp: buildAsyncTaskResponse(http.StatusOK, 1, 1, "create failed", ""),
			}),
			expectedErr:  "create DNS record failed: status=1, code=1, errMsg=create failed",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response without task id WHEN CreateIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				createResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "create DNS record response has no task_id",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response and failed wait WHEN CreateIntranetDnsDomain SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				createResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-fail"),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusFailed, "failed"),
			}),
			expectedErr:  "DNS record creation task failed: failed",
			expectedCall: true,
		},
		{
			name: "GIVEN canceled context and create api error WHEN CreateIntranetDnsDomain SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewLbmDnsService(&lbmDnsClientFake{
				createErr: apiErr,
			}),
			expectedErr:  "create IntranetDnsDomain record failed after retries: context canceled",
			expectedCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.CreateIntranetDnsDomain(ctx, buildCreateLbmDnsInput())

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok && tc.expectedCall {
				assert.Equal(t, buildCreateLbmDnsInput(), fake.createInput)
			}
		})
	}
}

func TestLbmDnsService_waitForLbmDnsRecordReady(t *testing.T) {
	testCases := []struct {
		name        string
		service     *LbmDnsService
		expected    string
		expectedErr string
	}{
		{
			name: "GIVEN completed task with resource id WHEN waitForLbmDnsRecordReady SHOULD return record id",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
			}),
			expected: testLbmDnsRecordId,
		},
		{
			name: "GIVEN completed task without resource id WHEN waitForLbmDnsRecordReady SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusSuccess, ""),
			}),
			expectedErr: "task completed but no resource_id returned",
		},
		{
			name: "GIVEN failed task wait WHEN waitForLbmDnsRecordReady SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusFailed, "failed"),
			}),
			expectedErr: "DNS record creation task failed: failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.service.waitForLbmDnsRecordReady(context.Background(), testLbmDnsTaskId)

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

func TestLbmDnsService_waitForTaskCompleted(t *testing.T) {
	apiErr := errors.New("query failed")
	testCases := []struct {
		name        string
		ctx         context.Context
		service     *LbmDnsService
		expected    *lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse
		expectedErr string
	}{
		{
			name: "GIVEN success task status WHEN waitForTaskCompleted SHOULD return task response",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
			}),
			expected: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
				lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
		},
		{
			name: "GIVEN running then success task status WHEN waitForTaskCompleted SHOULD return task response",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResponses: []*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse{
					buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
						lbmdnsclient.StatusCodeSuccess, "", "", "running", ""),
					buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
						lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
				},
			}),
			expected: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
				lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
		},
		{
			name: "GIVEN canceled context WHEN waitForTaskCompleted SHOULD return context error",
			ctx:  canceledContext(),
			service: newSlowLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, ""),
			}),
			expectedErr: "context cancelled while waiting for DNS record creation: context canceled",
		},
		{
			name: "GIVEN timeout WHEN waitForTaskCompleted SHOULD return timeout error",
			service: newTimeoutLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, ""),
			}),
			expectedErr: "timeout waiting for DNS record creation task: task-1",
		},
		{
			name: "GIVEN query api errors beyond tolerance WHEN waitForTaskCompleted SHOULD return query error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusErr: apiErr,
			}),
			expectedErr: "query lbm-dns task status failed: query failed",
		},
		{
			name: "GIVEN non-2xx http response WHEN waitForTaskCompleted SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, ""),
			}),
			expectedErr: "query lbm-dns task status failed, http status is 500",
		},
		{
			name: "GIVEN unsuccessful business response WHEN waitForTaskCompleted SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponse(http.StatusOK, 1, 1, "query failed"),
			}),
			expectedErr: "query task status failed: status=1, code=1, errMsg=query failed",
		},
		{
			name: "GIVEN failed task status WHEN waitForTaskCompleted SHOULD return task failed error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusFailed, "failed"),
			}),
			expectedErr: "DNS record creation task failed: failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			actual, err := tc.service.waitForTaskCompleted(ctx, testLbmDnsTaskId, "DNS record creation")

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.Nil(t, actual)
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestLbmDnsService_DeleteIntranetDnsDomain(t *testing.T) {
	apiErr := errors.New("delete failed")
	testCases := []struct {
		name         string
		ctx          context.Context
		service      *LbmDnsService
		expectedErr  string
		expectedCall bool
	}{
		{
			name: "GIVEN not found response WHEN DeleteIntranetDnsDomain SHOULD return nil",
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeResourceNotFound, "not found", ""),
			}),
			expectedCall: true,
		},
		{
			name: "GIVEN successful response and completed task WHEN DeleteIntranetDnsDomain SHOULD return nil",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsTaskId),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
			}),
			expectedCall: true,
		},
		{
			name:        "GIVEN nil client WHEN DeleteIntranetDnsDomain SHOULD return error",
			service:     NewLbmDnsService(nil),
			expectedErr: "m3 lbm-dns client is not initialized",
		},
		{
			name: "GIVEN nil response WHEN DeleteIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteResp: nil,
			}),
			expectedErr:  "response is nil for record dns-record-1",
			expectedCall: true,
		},
		{
			name: "GIVEN non-2xx http response WHEN DeleteIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusInternalServerError, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "httpStatusCode=500",
			expectedCall: true,
		},
		{
			name: "GIVEN unsuccessful business response WHEN DeleteIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusOK, 1, 1, "delete failed", ""),
			}),
			expectedErr:  "response from lbm dns server contains unsuccessful code: status=1, code=1, errMsg=delete failed",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response without task id WHEN DeleteIntranetDnsDomain SHOULD return error",
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", ""),
			}),
			expectedErr:  "delete IntranetDnsDomain record response has no task id",
			expectedCall: true,
		},
		{
			name: "GIVEN successful response and failed task wait WHEN DeleteIntranetDnsDomain SHOULD return error",
			service: newFastLbmDnsService(&lbmDnsClientFake{
				deleteResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-fail"),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusFailed, "failed"),
			}),
			expectedErr:  "IntranetDnsDomain record deletion task failed: failed",
			expectedCall: true,
		},
		{
			name: "GIVEN canceled context and delete api error WHEN DeleteIntranetDnsDomain SHOULD return wrapped context error",
			ctx:  canceledContext(),
			service: NewLbmDnsService(&lbmDnsClientFake{
				deleteErr: apiErr,
			}),
			expectedErr:  "call DeleteIntranetDnsDomain API failed: context canceled",
			expectedCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx
			}

			err := tc.service.DeleteIntranetDnsDomain(ctx, testLbmDnsRecordId)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
			if fake, ok := tc.service.client.(*lbmDnsClientFake); ok && tc.expectedCall {
				assert.Equal(t, testLbmDnsRecordId, fake.deleteRecordId)
			}
		})
	}
}

func TestLbmDnsService_UpdateRecordValue(t *testing.T) {
	apiErr := errors.New("update failed")
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
			service: newFastLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-success"),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", testLbmDnsRecordId, lbmdnsclient.TaskStatusSuccess, ""),
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
			service: newFastLbmDnsService(&lbmDnsClientFake{
				updateResp: buildAsyncTaskResponse(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "task-fail"),
				taskStatusResp: buildTaskStatusResponseWithData(http.StatusOK, lbmdnsclient.StatusCodeSuccess,
					lbmdnsclient.StatusCodeSuccess, "", "", lbmdnsclient.TaskStatusFailed, "failed"),
			}),
			expectedErr:  "DNS record update task failed: failed",
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

func Test_isLbmDnsNoChanges(t *testing.T) {
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

func TestLbmDnsService_getLbmDnsRawResponse(t *testing.T) {
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

func Test_extractLbmDnsRecordValues(t *testing.T) {
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

func TestLbmDnsService_GetRecord(t *testing.T) {
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

func TestLbmDnsService_GetLbmDnsDetail(t *testing.T) {
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
	createResp          *lbmdnsclient.AsyncTaskResponse
	createErr           error
	deleteResp          *lbmdnsclient.AsyncTaskResponse
	deleteErr           error
	updateResp          *lbmdnsclient.AsyncTaskResponse
	updateErr           error
	getResp             *lbmdnsclient.GetIntranetDnsDomainResponse
	getErr              error
	taskStatusResp      *lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse
	taskStatusResponses []*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse
	taskStatusErr       error
	createInput         CreateLbmDnsInput
	deleteRecordId      string
	updateRecordId      string
	updateIp            string
	getRecordId         string
	taskId              string
}

func (f *lbmDnsClientFake) CreateIntranetDnsDomain(_ context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (*lbmdnsclient.AsyncTaskResponse,
	error) {
	f.createInput = CreateLbmDnsInput{
		RegionCode:   regionCode,
		ServiceName:  serviceName,
		HostRecord:   hostRecord,
		DomainSuffix: domainSuffix,
		EndpointIp:   ip,
	}
	return f.createResp, f.createErr
}

func (f *lbmDnsClientFake) GetIntranetDnsDomainTaskStatus(_ context.Context,
	taskId string) (*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, error) {
	f.taskId = taskId
	if f.taskStatusErr != nil {
		return nil, f.taskStatusErr
	}
	if len(f.taskStatusResponses) > 0 {
		resp := f.taskStatusResponses[0]
		f.taskStatusResponses = f.taskStatusResponses[1:]
		return resp, nil
	}
	return f.taskStatusResp, nil
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
	resourceId string) (*lbmdnsclient.AsyncTaskResponse, error) {
	f.deleteRecordId = resourceId
	return f.deleteResp, f.deleteErr
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

func buildTaskStatusResponseWithData(httpStatusCode, status, code int, errMsg, resourceId, taskStatus,
	message string) *lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse {
	resp := buildTaskStatusResponse(httpStatusCode, status, code, errMsg)
	resp.Body.Data.ResourceId = resourceId
	resp.Body.Data.Status = taskStatus
	resp.Body.Data.Message = message
	return resp
}

func buildCreateLbmDnsInput() CreateLbmDnsInput {
	return CreateLbmDnsInput{
		RegionCode:   "cn-north-4",
		ServiceName:  "service-1",
		HostRecord:   "api",
		DomainSuffix: "example.com",
		EndpointIp:   testEndpointIp,
	}
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

func newFastLbmDnsService(client lbmdnsclient.LbmDnsClient) *LbmDnsService {
	service := NewLbmDnsService(client)
	service.pollingInterval = time.Nanosecond
	service.pollingTimeout = time.Second
	return service
}

func newTimeoutLbmDnsService(client lbmdnsclient.LbmDnsClient) *LbmDnsService {
	service := NewLbmDnsService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Nanosecond
	return service
}

func newSlowLbmDnsService(client lbmdnsclient.LbmDnsClient) *LbmDnsService {
	service := NewLbmDnsService(client)
	service.pollingInterval = time.Hour
	service.pollingTimeout = time.Hour
	return service
}
