/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package sniproxyclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testServiceName      = "test-service"
	testAccessObject     = "APIGW"
	testRegionCode       = "region-1"
	testIamDomainAccount = "account-1"
)

func TestClient_AccessService(t *testing.T) {
	ctx := context.Background()
	testResourceID := "test-resource-1"

	successBody, err := json.Marshal(AccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: 0, Msg: "success"},
		Data: AccessServiceResponseData{
			ResourceId:       testResourceID,
			ServiceName:      testServiceName,
			AccessObject:     testAccessObject,
			RegionCode:       testRegionCode,
			IamDomainAccount: []string{testIamDomainAccount},
			EpServiceIds:     []EpServiceInfo{{EpServiceId: "ep-1"}},
		},
	})
	require.NoError(t, err)

	errorBody, err := json.Marshal(AccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: 500, Msg: "internal error"},
		Data:         AccessServiceResponseData{},
	})
	require.NoError(t, err)

	req := AccessServiceRequest{
		RegionCode:       testRegionCode,
		ServiceName:      testServiceName,
		AccessObject:     testAccessObject,
		IamDomainAccount: []string{testIamDomainAccount},
		ChangeReason:     "test reason",
	}

	testCases := []struct {
		name                string
		httpStatusCode      int
		responseBody        []byte
		simulateError       bool
		marshalErr          bool
		expectedErrContains string
	}{
		{
			name:           "GIVEN success response WHEN AccessService SHOULD return response",
			httpStatusCode: 200,
			responseBody:   successBody,
		},
		{
			name:           "GIVEN HTTP error WHEN AccessService SHOULD return response with error status",
			httpStatusCode: 500,
			responseBody:   errorBody,
		},
		{
			name:                "GIVEN connection error WHEN AccessService SHOULD return error",
			simulateError:       true,
			expectedErrContains: "do request failed",
		},
		{
			name:                "GIVEN invalid JSON response WHEN AccessService SHOULD return unmarshal error",
			httpStatusCode:      200,
			responseBody:        []byte(`invalid json`),
			expectedErrContains: "unmarshal response failed",
		},
		{
			name:                "GIVEN request marshal error WHEN AccessService SHOULD return marshal error",
			httpStatusCode:      0,
			responseBody:        nil,
			marshalErr:          true,
			expectedErrContains: "marshal request failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var server *httptest.Server
			if tc.simulateError {
				server = httptest.NewServer(http.HandlerFunc(nil))
				server.Close()
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodPost, r.Method)
					assert.Equal(t, pathServiceAccess, r.URL.Path)

					bodyBytes, readErr := io.ReadAll(r.Body)
					assert.NoError(t, readErr, "failed to read request body")

					var receivedReq AccessServiceRequest
					unmarshalErr := json.Unmarshal(bodyBytes, &receivedReq)
					assert.NoError(t, unmarshalErr, "failed to unmarshal request body")

					assert.Equal(t, req.RegionCode, receivedReq.RegionCode)
					assert.Equal(t, req.ServiceName, receivedReq.ServiceName)
					assert.Equal(t, req.AccessObject, receivedReq.AccessObject)
					assert.Equal(t, req.IamDomainAccount, receivedReq.IamDomainAccount)
					assert.Equal(t, req.ChangeReason, receivedReq.ChangeReason)

					w.WriteHeader(tc.httpStatusCode)
					_, err = w.Write(tc.responseBody)
					assert.NoError(t, err)
				}))
			}
			defer server.Close()

			if tc.marshalErr {
				marshalPatches := gomonkey.ApplyFunc(json.Marshal, func(_ any) ([]byte, error) {
					return nil, errors.New(tc.expectedErrContains)
				})
				defer marshalPatches.Reset()
			}

			client := NewSniProxyClient(server.URL, "test-token")

			resp, actualErr := client.AccessService(ctx, req)

			if tc.expectedErrContains != "" {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tc.expectedErrContains)
				assert.Nil(t, resp)
			} else {
				assert.Nil(t, actualErr)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.httpStatusCode, resp.HTTPStatusCode)
				if tc.httpStatusCode == 200 && resp.Body.Code == 0 {
					assert.Equal(t, testResourceID, resp.Body.Data.ResourceId)
					assert.Equal(t, testServiceName, resp.Body.Data.ServiceName)
					assert.Equal(t, testAccessObject, resp.Body.Data.AccessObject)
				}
			}
		})
	}
}

func TestClient_DeleteAccessService(t *testing.T) {
	ctx := context.Background()
	testResourceID := "test-resource-2"

	successBody, err := json.Marshal(BaseResponse{Code: 0, Msg: "success"})
	require.NoError(t, err)

	noStaticBody, err := json.Marshal(BaseResponse{Code: statusCodeNoStaticResource, Msg: "no static resource"})
	require.NoError(t, err)

	errorBody, err := json.Marshal(BaseResponse{Code: 500, Msg: "server error"})
	require.NoError(t, err)

	testCases := []struct {
		name                string
		httpStatusCode      int
		responseBody        []byte
		simulateError       bool
		expectedErrContains string
		expectedCode        int
	}{
		{
			name:           "GIVEN success delete WHEN DeleteAccessService SHOULD return success response",
			httpStatusCode: 200,
			responseBody:   successBody,
			expectedCode:   0,
		},
		{
			name:           "GIVEN not exist code WHEN DeleteAccessService SHOULD return response",
			httpStatusCode: 200,
			responseBody:   noStaticBody,
			expectedCode:   statusCodeNoStaticResource,
		},
		{
			name:           "GIVEN HTTP error WHEN DeleteAccessService SHOULD return HTTP error status",
			httpStatusCode: 500,
			responseBody:   errorBody,
			expectedCode:   500,
		},
		{
			name:                "GIVEN connection error WHEN DeleteAccessService SHOULD return error",
			simulateError:       true,
			expectedErrContains: "do request failed",
		},
		{
			name:                "GIVEN invalid JSON WHEN DeleteAccessService SHOULD return unmarshal error",
			httpStatusCode:      200,
			responseBody:        []byte(`invalid json`),
			expectedErrContains: "unmarshal response failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var server *httptest.Server
			if tc.simulateError {
				server = httptest.NewServer(http.HandlerFunc(nil))
				server.Close()
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodDelete, r.Method)
					assert.True(t, strings.HasSuffix(r.URL.Path, testResourceID))
					w.WriteHeader(tc.httpStatusCode)
					_, err = w.Write(tc.responseBody)
					assert.NoError(t, err)
				}))
			}
			defer server.Close()

			client := NewSniProxyClient(server.URL, "test-token")

			resp, actualErr := client.DeleteAccessService(ctx, testResourceID)

			if tc.expectedErrContains != "" {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tc.expectedErrContains)
				assert.Nil(t, resp)
			} else {
				assert.Nil(t, actualErr)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.httpStatusCode, resp.HTTPStatusCode)
				assert.Equal(t, tc.expectedCode, resp.Body.Code)
			}
		})
	}
}

func TestClient_GetAccessService(t *testing.T) {
	ctx := context.Background()
	testResourceID := "test-resource-3"

	successBody, err := json.Marshal(GetAccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: 0, Msg: "success"},
		Data: AccessServiceResponseData{
			ResourceId:       testResourceID,
			ServiceName:      testServiceName,
			AccessObject:     testAccessObject,
			RegionCode:       testRegionCode,
			IamDomainAccount: []string{testIamDomainAccount},
			EpServiceIds:     []EpServiceInfo{{EpServiceId: "ep-1"}},
		},
	})
	require.NoError(t, err, "failed to marshal test response body")

	multiEpBody, err := json.Marshal(GetAccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: 0, Msg: "success"},
		Data: AccessServiceResponseData{
			ResourceId:       testResourceID,
			ServiceName:      testServiceName,
			AccessObject:     testAccessObject,
			RegionCode:       testRegionCode,
			IamDomainAccount: []string{testIamDomainAccount, "account-2"},
			EpServiceIds: []EpServiceInfo{
				{RegionCode: "region-1", Az: "az1", EpServiceId: "ep-1", EpServiceName: "ep-service-1"},
				{RegionCode: "region-2", Az: "az2", EpServiceId: "ep-2", EpServiceName: "ep-service-2"},
			},
		},
	})
	require.NoError(t, err, "failed to marshal test response body")

	emptyEpBody, err := json.Marshal(GetAccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: 0, Msg: "success"},
		Data: AccessServiceResponseData{
			ResourceId:       testResourceID,
			ServiceName:      testServiceName,
			AccessObject:     testAccessObject,
			RegionCode:       testRegionCode,
			IamDomainAccount: []string{},
			EpServiceIds:     []EpServiceInfo{},
		},
	})
	require.NoError(t, err, "failed to marshal test response body")

	notExistBody, err := json.Marshal(GetAccessServiceResponseBody{
		BaseResponse: BaseResponse{Code: statusCodeNoExist, Msg: "not exist"},
	})
	require.NoError(t, err, "failed to marshal test response body")

	testCases := []struct {
		name                 string
		resourceId           string
		httpStatusCode       int
		responseBody         []byte
		simulateError        bool
		expectedErrContains  string
		expectedDataNotNil   bool
		expectedCode         int
		expectedEpServiceIds []string
		expectedAccounts     []string
	}{
		{
			name:                 "GIVEN success response WHEN GetAccessService SHOULD return response data",
			resourceId:           testResourceID,
			httpStatusCode:       200,
			responseBody:         successBody,
			expectedDataNotNil:   true,
			expectedCode:         0,
			expectedEpServiceIds: []string{"ep-1"},
			expectedAccounts:     []string{testIamDomainAccount},
		},
		{
			name:                 "GIVEN success response with multiple ep services WHEN GetAccessService SHOULD return all ep service ids",
			resourceId:           testResourceID,
			httpStatusCode:       200,
			responseBody:         multiEpBody,
			expectedDataNotNil:   true,
			expectedCode:         0,
			expectedEpServiceIds: []string{"ep-1", "ep-2"},
			expectedAccounts:     []string{testIamDomainAccount, "account-2"},
		},
		{
			name:                 "GIVEN success response with empty ep services WHEN GetAccessService SHOULD return empty slice",
			resourceId:           testResourceID,
			httpStatusCode:       200,
			responseBody:         emptyEpBody,
			expectedDataNotNil:   true,
			expectedCode:         0,
			expectedEpServiceIds: []string{},
			expectedAccounts:     []string{},
		},
		{
			name:               "GIVEN not exist code WHEN GetAccessService SHOULD return response with error code",
			resourceId:         testResourceID,
			httpStatusCode:     200,
			responseBody:       notExistBody,
			expectedDataNotNil: false,
			expectedCode:       statusCodeNoExist,
		},
		{
			name:           "GIVEN HTTP error WHEN GetAccessService SHOULD return response with error status",
			resourceId:     testResourceID,
			httpStatusCode: 404,
			responseBody:   []byte(`{"code":404,"msg":"not found"}`),
		},
		{
			name:                "GIVEN connection error WHEN GetAccessService SHOULD return error",
			resourceId:          testResourceID,
			simulateError:       true,
			expectedErrContains: "do request failed",
		},
		{
			name:                "GIVEN invalid JSON WHEN GetAccessService SHOULD return unmarshal error",
			resourceId:          testResourceID,
			httpStatusCode:      200,
			responseBody:        []byte(`invalid json`),
			expectedErrContains: "unmarshal response failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var server *httptest.Server
			if tc.simulateError {
				server = httptest.NewServer(http.HandlerFunc(nil))
				server.Close()
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodGet, r.Method)
					assert.True(t, strings.Contains(r.URL.Path, tc.resourceId))
					assert.True(t, strings.Contains(r.URL.Path, pathServiceAccess))
					w.WriteHeader(tc.httpStatusCode)
					_, err = w.Write(tc.responseBody)
					assert.NoError(t, err)
				}))
			}
			defer server.Close()

			client := NewSniProxyClient(server.URL, "test-token")

			actualResp, actualErr := client.GetAccessService(ctx, tc.resourceId)

			if tc.expectedErrContains != "" {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tc.expectedErrContains)
				assert.Nil(t, actualResp)
			} else {
				assert.Nil(t, actualErr)
				assert.NotNil(t, actualResp)
				assert.Equal(t, tc.httpStatusCode, actualResp.HTTPStatusCode)
				if tc.expectedDataNotNil {
					assert.Equal(t, tc.expectedCode, actualResp.Body.Code)
					assert.Equal(t, testResourceID, actualResp.Body.Data.ResourceId)
					assert.Equal(t, testServiceName, actualResp.Body.Data.ServiceName)
				}
				if tc.expectedEpServiceIds != nil {
					actualIds := make([]string, len(actualResp.Body.Data.EpServiceIds))
					for i, ep := range actualResp.Body.Data.EpServiceIds {
						actualIds[i] = ep.EpServiceId
					}
					assert.Equal(t, tc.expectedEpServiceIds, actualIds)
				}
				if tc.expectedAccounts != nil {
					assert.Equal(t, tc.expectedAccounts, actualResp.Body.Data.IamDomainAccount)
				}
			}
		})
	}
}

func TestIsNotExist(t *testing.T) {
	testCases := []struct {
		name     string
		code     int
		expected bool
	}{
		{
			name:     "GIVEN notExistCode WHEN IsNotExist SHOULD return true",
			code:     statusCodeNoExist,
			expected: true,
		},
		{
			name:     "GIVEN noStaticResource WHEN IsNotExist SHOULD return true",
			code:     statusCodeNoStaticResource,
			expected: true,
		},
		{
			name:     "GIVEN success code WHEN IsNotExist SHOULD return false",
			code:     0,
			expected: false,
		},
		{
			name:     "GIVEN other error code WHEN IsNotExist SHOULD return false",
			code:     400,
			expected: false,
		},
		{
			name:     "GIVEN negative code WHEN IsNotExist SHOULD return false",
			code:     -100,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsNotExist(tc.code)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
