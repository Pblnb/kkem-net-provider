/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
)

const (
	testToken        = "test-token"
	testRegionCode   = "cn-test-1"
	testServiceName  = "test-service"
	testHostRecord   = "test-host"
	testDomainSuffix = "example.com"
	testRecordId     = "record-1"
	testTaskId       = "task-1"
	testEndpointIp   = "10.0.0.8"
)

func TestNewDnsClient(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		token    string
	}{
		{
			name:     "GIVEN endpoint with scheme WHEN NewDnsClient SHOULD return initialized client",
			endpoint: "http://localhost",
			token:    testToken,
		},
		{
			name:     "GIVEN endpoint without scheme WHEN NewDnsClient SHOULD return initialized client",
			endpoint: "localhost",
			token:    testToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := NewDnsClient(tc.endpoint, tc.token)

			assert.NotNil(t, actual)
			assert.NotNil(t, actual.Client)
		})
	}
}

func TestClient_CreateIntranetDnsDomain(t *testing.T) {
	marshalErr := errors.New("marshal failed")
	testCases := []struct {
		name               string
		server             func(t *testing.T) *httptest.Server
		patchMarshal       bool
		expectedTask       string
		expectedHTTPStatus int
		expectedError      string
	}{
		{
			name: "GIVEN valid create response WHEN CreateIntranetDnsDomain SHOULD return task response",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodPost, pathCreateIntranetDnsDomain, func(t *testing.T,
					body []byte) {
					var actual IntranetDnsDomainResource
					assert.NoError(t, json.Unmarshal(body, &actual))
					assert.Equal(t, testRegionCode, actual.RegionCode)
					assert.Equal(t, testServiceName, actual.ServiceName)
					assert.Equal(t, testHostRecord, actual.HostRecord)
					assert.Equal(t, testDomainSuffix, actual.DomainSuffix)
					assert.Equal(t, []IntranetDnsRecordValue{{RecordType: recordTypeA, RecordValue: testEndpointIp}},
						actual.RecordValues)
				}, `{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask: testTaskId,
		},
		{
			name: "GIVEN 500 create response WHEN CreateIntranetDnsDomain SHOULD return response status code",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsServerWithStatus(t, http.StatusInternalServerError, http.MethodPost,
					pathCreateIntranetDnsDomain, nil, `{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask:       testTaskId,
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "GIVEN invalid json response WHEN CreateIntranetDnsDomain SHOULD return unmarshal error",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodPost, pathCreateIntranetDnsDomain, func(t *testing.T,
					body []byte) {
					assert.NotEmpty(t, body)
				}, `{`)
			},
			expectedError: "unmarshal DNS response failed",
		},
		{
			name: "GIVEN closed server WHEN CreateIntranetDnsDomain SHOULD return request error",
			server: func(t *testing.T) *httptest.Server {
				server := newAssertableLbmDnsOKServer(t, http.MethodPost, pathCreateIntranetDnsDomain, nil, `{}`)
				server.Close()
				return server
			},
			expectedError: "send create DNS record request failed",
		},
		{
			name: "GIVEN marshal fails WHEN CreateIntranetDnsDomain SHOULD return marshal error without request",
			server: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Fail(t, "CreateIntranetDnsDomain should not send request when marshal fails")
				}))
			},
			patchMarshal:  true,
			expectedError: "marshal DNS request failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server(t)
			defer server.Close()
			if tc.patchMarshal {
				patches := gomonkey.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
					return nil, marshalErr
				})
				defer patches.Reset()
			}
			client := NewDnsClient(server.URL, testToken)

			actual, actualErr := client.CreateIntranetDnsDomain(context.Background(), testRegionCode, testServiceName,
				testHostRecord, testDomainSuffix, testEndpointIp)

			if tc.expectedError != "" {
				assert.ErrorContains(t, actualErr, tc.expectedError)
				assert.Nil(t, actual)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, expectedHTTPStatus(tc.expectedHTTPStatus), actual.HTTPStatusCode)
			assert.Equal(t, tc.expectedTask, actual.Body.TaskId)
		})
	}
}

func TestClient_GetIntranetDnsDomainTaskStatus(t *testing.T) {
	testCases := []struct {
		name               string
		server             func(t *testing.T) *httptest.Server
		expectedStatus     string
		expectedHTTPStatus int
		expectedError      string
	}{
		{
			name: "GIVEN valid task response WHEN GetIntranetDnsDomainTaskStatus SHOULD return task status",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, testTaskId), nil,
					`{"status":0,"code":0,"data":{"resourceId":"record-1","status":"success","msg":"done"}}`)
			},
			expectedStatus: TaskStatusSuccess,
		},
		{
			name: "GIVEN 400 task response WHEN GetIntranetDnsDomainTaskStatus SHOULD return response status code",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsServerWithStatus(t, http.StatusBadRequest, http.MethodGet,
					fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, testTaskId), nil,
					`{"status":0,"code":0,"data":{"resourceId":"record-1","status":"success","msg":"done"}}`)
			},
			expectedStatus:     TaskStatusSuccess,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "GIVEN invalid json response WHEN GetIntranetDnsDomainTaskStatus SHOULD return unmarshal error",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, testTaskId), nil, `{`)
			},
			expectedError: "unmarshal task status response failed",
		},
		{
			name: "GIVEN closed server WHEN GetIntranetDnsDomainTaskStatus SHOULD return request error",
			server: func(t *testing.T) *httptest.Server {
				server := newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, testTaskId), nil, `{}`)
				server.Close()
				return server
			},
			expectedError: "query task status failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server(t)
			defer server.Close()
			client := NewDnsClient(server.URL, testToken)

			actual, actualErr := client.GetIntranetDnsDomainTaskStatus(context.Background(), testTaskId)

			if tc.expectedError != "" {
				assert.ErrorContains(t, actualErr, tc.expectedError)
				assert.Nil(t, actual)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, expectedHTTPStatus(tc.expectedHTTPStatus), actual.HTTPStatusCode)
			assert.Equal(t, tc.expectedStatus, actual.Body.Data.Status)
			assert.Equal(t, testRecordId, actual.Body.Data.ResourceId)
		})
	}
}

func TestClient_GetIntranetDnsDomain(t *testing.T) {
	testCases := []struct {
		name               string
		server             func(t *testing.T) *httptest.Server
		expectedHTTPStatus int
		expectedError      string
	}{
		{
			name: "GIVEN valid record response WHEN GetIntranetDnsDomain SHOULD return record resource",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil,
					`{"status":0,"code":0,"data":{"regionCode":"cn-test-1","serviceName":"test-service","hostRecord":"test-host","domainSuffix":"example.com","recordValues":[{"recordType":"A","recordValue":"10.0.0.8"}]}}`)
			},
		},
		{
			name: "GIVEN 500 record response WHEN GetIntranetDnsDomain SHOULD return response status code",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsServerWithStatus(t, http.StatusInternalServerError, http.MethodGet,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil,
					`{"status":0,"code":0,"data":{"regionCode":"cn-test-1","serviceName":"test-service","hostRecord":"test-host","domainSuffix":"example.com","recordValues":[{"recordType":"A","recordValue":"10.0.0.8"}]}}`)
			},
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "GIVEN invalid json response WHEN GetIntranetDnsDomain SHOULD return unmarshal error",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{`)
			},
			expectedError: "unmarshal DNS response failed",
		},
		{
			name: "GIVEN closed server WHEN GetIntranetDnsDomain SHOULD return request error",
			server: func(t *testing.T) *httptest.Server {
				server := newAssertableLbmDnsOKServer(t, http.MethodGet,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{}`)
				server.Close()
				return server
			},
			expectedError: "query DNS record failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server(t)
			defer server.Close()
			client := NewDnsClient(server.URL, testToken)

			actual, actualErr := client.GetIntranetDnsDomain(context.Background(), testRecordId)

			if tc.expectedError != "" {
				assert.ErrorContains(t, actualErr, tc.expectedError)
				assert.Nil(t, actual)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, expectedHTTPStatus(tc.expectedHTTPStatus), actual.HTTPStatusCode)
			assert.Equal(t, testRegionCode, actual.Body.Data.RegionCode)
			assert.Equal(t, testEndpointIp, actual.Body.Data.RecordValues[0].RecordValue)
		})
	}
}

func TestClient_UpdateIntranetDnsDomain(t *testing.T) {
	marshalErr := errors.New("marshal failed")
	testCases := []struct {
		name               string
		server             func(t *testing.T) *httptest.Server
		patchMarshal       bool
		expectedTask       string
		expectedHTTPStatus int
		expectedError      string
	}{
		{
			name: "GIVEN valid update response WHEN UpdateIntranetDnsDomain SHOULD return task response",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodPut,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), func(t *testing.T, body []byte) {
						var actual IntranetDnsDomainRecordValues
						assert.NoError(t, json.Unmarshal(body, &actual))
						assert.Equal(t,
							[]IntranetDnsRecordValue{{RecordType: recordTypeA, RecordValue: testEndpointIp}},
							actual.RecordValues)
					}, `{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask: testTaskId,
		},
		{
			name: "GIVEN 400 update response WHEN UpdateIntranetDnsDomain SHOULD return response status code",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsServerWithStatus(t, http.StatusBadRequest, http.MethodPut,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil,
					`{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask:       testTaskId,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "GIVEN invalid json response WHEN UpdateIntranetDnsDomain SHOULD return unmarshal error",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodPut,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{`)
			},
			expectedError: "unmarshal DNS update response failed",
		},
		{
			name: "GIVEN closed server WHEN UpdateIntranetDnsDomain SHOULD return request error",
			server: func(t *testing.T) *httptest.Server {
				server := newAssertableLbmDnsOKServer(t, http.MethodPut,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{}`)
				server.Close()
				return server
			},
			expectedError: "send update DNS record request failed",
		},
		{
			name: "GIVEN marshal fails WHEN UpdateIntranetDnsDomain SHOULD return marshal error without request",
			server: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Fail(t, "UpdateIntranetDnsDomain should not send request when marshal fails")
				}))
			},
			patchMarshal:  true,
			expectedError: "marshal DNS update request failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server(t)
			defer server.Close()
			if tc.patchMarshal {
				patches := gomonkey.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
					return nil, marshalErr
				})
				defer patches.Reset()
			}
			client := NewDnsClient(server.URL, testToken)

			actual, actualErr := client.UpdateIntranetDnsDomain(context.Background(), testRecordId, testEndpointIp)

			if tc.expectedError != "" {
				assert.ErrorContains(t, actualErr, tc.expectedError)
				assert.Nil(t, actual)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, expectedHTTPStatus(tc.expectedHTTPStatus), actual.HTTPStatusCode)
			assert.Equal(t, tc.expectedTask, actual.Body.TaskId)
		})
	}
}

func TestClient_DeleteIntranetDnsDomain(t *testing.T) {
	testCases := []struct {
		name               string
		server             func(t *testing.T) *httptest.Server
		expectedTask       string
		expectedHTTPStatus int
		expectedError      string
	}{
		{
			name: "GIVEN valid delete response WHEN DeleteIntranetDnsDomain SHOULD return task response",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodDelete,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil,
					`{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask: testTaskId,
		},
		{
			name: "GIVEN 500 delete response WHEN DeleteIntranetDnsDomain SHOULD return response status code",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsServerWithStatus(t, http.StatusInternalServerError, http.MethodDelete,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil,
					`{"status":0,"code":0,"msg":"","data":"task-1"}`)
			},
			expectedTask:       testTaskId,
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "GIVEN invalid json response WHEN DeleteIntranetDnsDomain SHOULD return unmarshal error",
			server: func(t *testing.T) *httptest.Server {
				return newAssertableLbmDnsOKServer(t, http.MethodDelete,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{`)
			},
			expectedError: "unmarshal DNS delete response failed",
		},
		{
			name: "GIVEN closed server WHEN DeleteIntranetDnsDomain SHOULD return request error",
			server: func(t *testing.T) *httptest.Server {
				server := newAssertableLbmDnsOKServer(t, http.MethodDelete,
					fmt.Sprintf(pathIntranetDnsDomainResource, testRecordId), nil, `{}`)
				server.Close()
				return server
			},
			expectedError: "send delete DNS record request failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server(t)
			defer server.Close()
			client := NewDnsClient(server.URL, testToken)

			actual, actualErr := client.DeleteIntranetDnsDomain(context.Background(), testRecordId)

			if tc.expectedError != "" {
				assert.ErrorContains(t, actualErr, tc.expectedError)
				assert.Nil(t, actual)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, expectedHTTPStatus(tc.expectedHTTPStatus), actual.HTTPStatusCode)
			assert.Equal(t, tc.expectedTask, actual.Body.TaskId)
		})
	}
}

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

func newAssertableLbmDnsOKServer(t *testing.T, expectedMethod, expectedPath string, assertBody func(*testing.T, []byte),
	responseBody string) *httptest.Server {
	t.Helper()

	return newAssertableLbmDnsServerWithStatus(t, http.StatusOK, expectedMethod, expectedPath, assertBody, responseBody)
}

func newAssertableLbmDnsServerWithStatus(t *testing.T, responseStatus int, expectedMethod, expectedPath string,
	assertBody func(*testing.T, []byte), responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedMethod, r.Method)
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, testToken, r.Header.Get("x-open-token"))
		assert.Equal(t, "application/json", r.Header.Get("content-type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		if assertBody != nil {
			assertBody(t, body)
		}

		w.WriteHeader(responseStatus)
		_, err = w.Write([]byte(responseBody))
		assert.NoError(t, err)
	}))
}

func expectedHTTPStatus(status int) int {
	if status == 0 {
		// 测试用例未显式指定 HTTP status 时，默认使用 OK 状态。
		return http.StatusOK
	}
	return status
}
