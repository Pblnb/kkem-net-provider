/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package common

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCommonClientToken = "test-token"

func TestNewClient(t *testing.T) {
	testCases := []struct {
		name             string
		endpoint         string
		token            string
		expectedEndpoint string
		expectedToken    string
	}{
		{
			name:             "GIVEN endpoint with http scheme WHEN NewClient SHOULD keep endpoint",
			endpoint:         "http://localhost",
			token:            testCommonClientToken,
			expectedEndpoint: "http://localhost",
			expectedToken:    testCommonClientToken,
		},
		{
			name:             "GIVEN endpoint with https scheme WHEN NewClient SHOULD keep endpoint",
			endpoint:         "https://localhost",
			token:            testCommonClientToken,
			expectedEndpoint: "https://localhost",
			expectedToken:    testCommonClientToken,
		},
		{
			name:             "GIVEN endpoint without scheme WHEN NewClient SHOULD prepend https scheme",
			endpoint:         "localhost",
			token:            testCommonClientToken,
			expectedEndpoint: "https://localhost",
			expectedToken:    testCommonClientToken,
		},
		{
			name:             "GIVEN empty endpoint WHEN NewClient SHOULD prepend https scheme",
			endpoint:         "",
			token:            testCommonClientToken,
			expectedEndpoint: "https://",
			expectedToken:    testCommonClientToken,
		},
		{
			name:             "GIVEN empty token WHEN NewClient SHOULD keep empty token",
			endpoint:         "localhost",
			token:            "",
			expectedEndpoint: "https://localhost",
			expectedToken:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := NewClient(tc.endpoint, tc.token)

			assert.Equal(t, tc.expectedEndpoint, actual.endpoint)
			assert.Equal(t, tc.expectedToken, actual.token)
			assert.NotNil(t, actual.httpClient)
			assert.Equal(t, 3*time.Minute, actual.httpClient.Timeout)

			transport, ok := actual.httpClient.Transport.(*http.Transport)
			// 后续断言依赖 Transport 类型断言成功，使用 require 避免失败后继续解引用造成额外 panic。
			require.True(t, ok)
			assert.NotNil(t, transport.TLSClientConfig)
			assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
			assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
		})
	}
}

func TestClient_DoRequest(t *testing.T) {
	const (
		testRequestPath  = "/v1/resources"
		testRequestBody  = `{"name":"test"}`
		testResponseBody = `{"id":"resource-1"}`
	)
	testCtx := context.Background()

	testCases := []struct {
		name           string
		ctx            context.Context
		setup          func(t *testing.T) (*Client, func())
		method         string
		path           string
		requestBody    io.Reader
		expectedBody   []byte
		expectedStatus int
		expectedErr    string
		expectedPanic  bool
	}{
		{
			name: "GIVEN valid request WHEN DoRequest SHOULD return response body and status",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodPost, testRequestPath, testCommonClientToken, testRequestBody,
					http.StatusCreated, testResponseBody))
			},
			method:         http.MethodPost,
			path:           testRequestPath,
			requestBody:    strings.NewReader(testRequestBody),
			expectedBody:   []byte(testResponseBody),
			expectedStatus: http.StatusCreated,
		},
		{
			name: "GIVEN empty token WHEN DoRequest SHOULD omit token header",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, "", newAssertableServer(t,
					http.MethodGet, testRequestPath, "", "", http.StatusOK, "ok"))
			},
			method:         http.MethodGet,
			path:           testRequestPath,
			expectedBody:   []byte("ok"),
			expectedStatus: http.StatusOK,
		},
		{
			name: "GIVEN empty path WHEN DoRequest SHOULD send request to root path",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodGet, "/", testCommonClientToken, "", http.StatusOK, "ok"))
			},
			method:         http.MethodGet,
			path:           "",
			expectedBody:   []byte("ok"),
			expectedStatus: http.StatusOK,
		},
		{
			name: "GIVEN empty method WHEN DoRequest SHOULD send get request",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodGet, testRequestPath, testCommonClientToken, "", http.StatusOK, "ok"))
			},
			method:         "",
			path:           testRequestPath,
			expectedBody:   []byte("ok"),
			expectedStatus: http.StatusOK,
		},
		{
			name: "GIVEN nil body WHEN DoRequest SHOULD send request with empty body",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodPost, testRequestPath, testCommonClientToken, "", http.StatusOK, "ok"))
			},
			method:         http.MethodPost,
			path:           testRequestPath,
			requestBody:    nil,
			expectedBody:   []byte("ok"),
			expectedStatus: http.StatusOK,
		},
		{
			name: "GIVEN response body close fails WHEN DoRequest SHOULD still return response body and status",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithTransport(mockTransport{
					t:              t,
					expectedMethod: http.MethodGet,
					expectedPath:   testRequestPath,
					statusCode:     http.StatusAccepted,
					body:           closeErrorBody{reader: strings.NewReader(testResponseBody)},
				})
			},
			method:         http.MethodGet,
			path:           testRequestPath,
			expectedBody:   []byte(testResponseBody),
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "GIVEN response with 5xx status code WHEN DoRequest SHOULD still return response body and status",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodPost, testRequestPath, testCommonClientToken, testRequestBody,
					http.StatusInternalServerError, ""))
			},
			method:         http.MethodPost,
			path:           testRequestPath,
			requestBody:    strings.NewReader(testRequestBody),
			expectedBody:   []byte{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "GIVEN canceled context WHEN DoRequest SHOULD return send request error",
			ctx:  canceledContext(),
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodGet, testRequestPath, testCommonClientToken, "", http.StatusOK, "ok"))
			},
			method:      http.MethodGet,
			path:        testRequestPath,
			expectedErr: "send HTTP request failed",
		},
		{
			name: "GIVEN nil context WHEN DoRequest SHOULD panic",
			ctx:  nil,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithServer(t, testCommonClientToken, newAssertableServer(t,
					http.MethodGet, testRequestPath, testCommonClientToken, "", http.StatusOK, "ok"))
			},
			method:        http.MethodGet,
			path:          testRequestPath,
			expectedPanic: true,
		},
		{
			name: "GIVEN invalid request url WHEN DoRequest SHOULD return create request error",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return NewClient("http://[::1", testCommonClientToken), func() {}
			},
			method:      http.MethodGet,
			path:        testRequestPath,
			expectedErr: "create HTTP request failed",
		},
		{
			name: "GIVEN invalid method WHEN DoRequest SHOULD return create request error",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return NewClient("http://example.com", testCommonClientToken), func() {}
			},
			method:      "BAD METHOD",
			path:        testRequestPath,
			expectedErr: "create HTTP request failed",
		},
		{
			name: "GIVEN closed server WHEN DoRequest SHOULD return send request error",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClosedServerClient(t, testCommonClientToken)
			},
			method:      http.MethodGet,
			path:        testRequestPath,
			expectedErr: "send HTTP request failed",
		},
		{
			name: "GIVEN response timeout WHEN DoRequest SHOULD return send request error",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newTimeoutServerClient(t, testCommonClientToken, time.Nanosecond)
			},
			method:      http.MethodGet,
			path:        testRequestPath,
			expectedErr: "send HTTP request failed",
		},
		{
			name: "GIVEN response body read fails WHEN DoRequest SHOULD return read response body error",
			ctx:  testCtx,
			setup: func(t *testing.T) (*Client, func()) {
				return newClientWithTransport(mockTransport{
					t:              t,
					expectedMethod: http.MethodGet,
					expectedPath:   testRequestPath,
					statusCode:     http.StatusInternalServerError,
					body:           readErrorBody{},
				})
			},
			method:         http.MethodGet,
			path:           testRequestPath,
			expectedStatus: http.StatusInternalServerError,
			expectedErr:    "read response body failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个用例通过 setup 构造独立 client，避免 server 和 fake transport 的配置干扰断言主体。
			client, cleanup := tc.setup(t)
			defer cleanup()
			if tc.expectedPanic {
				assert.Panics(t, func() {
					_, _, _ = client.DoRequest(tc.ctx, tc.method, tc.path, tc.requestBody)
				})
				return
			}

			actualBody, actualStatus, actualErr := client.DoRequest(tc.ctx, tc.method, tc.path,
				tc.requestBody)

			if tc.expectedErr != "" {
				assert.ErrorContains(t, actualErr, tc.expectedErr)
				assert.Nil(t, actualBody)
				assert.Equal(t, tc.expectedStatus, actualStatus)
				return
			}
			assert.NoError(t, actualErr)
			assert.Equal(t, tc.expectedBody, actualBody)
			assert.Equal(t, tc.expectedStatus, actualStatus)
		})
	}
}

func Test_maskToken(t *testing.T) {
	testCases := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "GIVEN empty token WHEN maskToken SHOULD return mask",
			token:    "",
			expected: "****",
		},
		{
			name:     "GIVEN eight chars token WHEN maskToken SHOULD return mask",
			token:    "12345678",
			expected: "****",
		},
		{
			name:     "GIVEN long token WHEN maskToken SHOULD keep prefix and suffix",
			token:    "1234567890abcdef",
			expected: "1234****cdef",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := maskToken(tc.token)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

// newAssertableServer 用于正常 HTTP 场景，同时校验 DoRequest 写入的请求方法、路径、header 和 body。
func newAssertableServer(t *testing.T, expectedMethod, expectedPath, expectedToken, expectedBody string,
	responseStatus int, responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedMethod, r.Method)
		assert.Equal(t, expectedPath, r.URL.Path)
		if expectedToken == "" {
			assert.Empty(t, r.Header.Get("x-open-token"))
		} else {
			assert.Equal(t, expectedToken, r.Header.Get("x-open-token"))
		}
		assert.Equal(t, "application/json", r.Header.Get("content-type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedBody, string(body))

		w.WriteHeader(responseStatus)
		_, err = w.Write([]byte(responseBody))
		assert.NoError(t, err)
	}))
}

func newClientWithServer(t *testing.T, token string, server *httptest.Server) (*Client, func()) {
	t.Helper()

	return NewClient(server.URL, token), server.Close
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// newClientWithTransport 用于标准 httptest.Server 难以自然构造的 Body.Close/Read 异常分支。
func newClientWithTransport(transport http.RoundTripper) (*Client, func()) {
	client := NewClient("http://example.com", testCommonClientToken)
	client.httpClient.Transport = transport
	return client, func() {}
}

func newClosedServerClient(t *testing.T, token string) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	return NewClient(server.URL, token), func() {}
}

// newTimeoutServerClient 让 handler 阻塞直到 cleanup 释放，并把 client timeout 调小以避免真实等待。
func newTimeoutServerClient(t *testing.T, token string, timeout time.Duration) (*Client, func()) {
	t.Helper()

	responseReady := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-responseReady
	}))
	client := NewClient(server.URL, token)
	client.httpClient.Timeout = timeout
	return client, func() {
		close(responseReady)
		server.Close()
	}
}

// mockTransport 只负责构造响应对象，便于测试 DoRequest 读取和关闭 Body 的异常分支。
type mockTransport struct {
	t              *testing.T
	expectedMethod string
	expectedPath   string
	statusCode     int
	body           io.ReadCloser
}

func (f mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Helper()

	assert.Equal(f.t, f.expectedMethod, req.Method)
	assert.Equal(f.t, f.expectedPath, req.URL.Path)
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       f.body,
	}, nil
}

type closeErrorBody struct {
	reader *strings.Reader
}

func (b closeErrorBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b closeErrorBody) Close() error {
	return errors.New("close failed")
}

type readErrorBody struct{}

func (b readErrorBody) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (b readErrorBody) Close() error {
	return nil
}
