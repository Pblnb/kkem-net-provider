/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	actionPost   = "POST"
	actionGet    = "GET"
	actionPut    = "PUT"
	actionDelete = "DELETE"
)

// Client lbm-dns 客户端，封装 HTTP 调用与异步任务轮询
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// NewClient 创建 LBM DNS Client
// endpoint: LBM DNS 服务地址（如 https://lbm-app-api.myhuaweicloud.com）
// token: x-open-token 认证令牌
func NewClient(endpoint, token string) *Client {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	return &Client{
		endpoint: endpoint,
		token:    token,
		httpClient: &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			}},
			Timeout: 3 * time.Minute,
		},
	}
}

// doRequest 发起 HTTP 请求，返回响应 body 和 HTTP 状态码
func (c *Client) doRequest(ctx context.Context, method, path string, reqBody io.Reader) ([]byte, int,
	error) {
	url := c.endpoint + path

	tflog.Debug(ctx, "Sending HTTP request", map[string]any{
		"method": method,
		"url":    url,
		"headers": map[string]string{
			"content-type": "application/json",
			"x-open-token": maskToken(c.token),
		},
	})

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request failed: %w", err)
	}
	if len(c.token) > 0 {
		req.Header.Set("x-open-token", c.token)
	}
	req.Header.Add("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("send HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			tflog.Warn(ctx, "failed to close response body", map[string]any{
				"error": closeErr.Error(),
			})
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body failed: %w", err)
	}
	return body, resp.StatusCode, nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
