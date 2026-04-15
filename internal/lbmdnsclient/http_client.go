/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	actionPost = "POST"
	actionGet  = "GET"
)

type clientAttr struct {
	Token string `json:"token"`
	Host  string `json:"host"`
}

func sendHTTP(attr *clientAttr, method, path string, reqBody io.Reader) ([]byte, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		}},
		Timeout: 3 * time.Minute,
	}
	req, err := http.NewRequest(method, attr.Host+path, reqBody)
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
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("received failed response, statusCode=%d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}
	return body, nil
}
