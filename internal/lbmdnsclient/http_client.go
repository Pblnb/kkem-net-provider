/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"bytes"
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
	actionPost = "POST"
	actionGet  = "GET"
)

type clientAttr struct {
	Token string `json:"token"`
	Host  string `json:"host"`
}

func sendHTTP(ctx context.Context, attr *clientAttr, method, path string, reqBody io.Reader) ([]byte, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		}},
		Timeout: 3 * time.Minute,
	}

	host := attr.Host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	url := host + path

	var bodyBytes []byte
	if reqBody != nil {
		if readErr := readBody(reqBody, &bodyBytes); readErr != nil {
			return nil, fmt.Errorf("read request body failed: %w", readErr)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	tflog.Debug(ctx, "Sending HTTP request", map[string]any{
		"method": method,
		"url":    url,
		"headers": map[string]string{
			"content-type": "application/json",
			"x-open-token": maskToken(attr.Token),
		},
		"body": string(bodyBytes),
	})

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request failed: %w", err)
	}
	if len(attr.Token) > 0 {
		req.Header.Set("x-open-token", attr.Token)
	}
	req.Header.Add("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send HTTP request failed: %w", err)
	}
	// cmt: 这里 ide 告警有 error 没处理，评估下需不需要加上处理错误的逻辑
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("received failed response, statusCode=%d, body=%s", resp.StatusCode, string(respBody))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}
	return body, nil
}

func readBody(r io.Reader, out *[]byte) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	*out = data
	return nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
