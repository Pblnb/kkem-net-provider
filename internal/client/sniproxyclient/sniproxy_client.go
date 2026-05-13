/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package sniproxyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"huawei.com/kkem/kkem-net-provider/internal/client/common"
	"net/http"
)

type SniProxyClient interface {
	AccessService(ctx context.Context, req AccessServiceRequest) (*AccessServiceResponse, error)
	DeleteAccessService(ctx context.Context, resourceId string) (*DeleteAccessServiceResponse, error)
	GetAccessService(ctx context.Context, resourceId string) (*GetAccessServiceResponse, error)
}

type Client struct {
	*common.Client
}

func NewSniProxyClient(endpoint, token string) *Client {
	return &Client{
		Client: common.NewClient(endpoint, token),
	}
}

// AccessService 接入服务
func (c *Client) AccessService(ctx context.Context, req AccessServiceRequest) (*AccessServiceResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	respBody, httpStatus, err := c.DoRequest(ctx, http.MethodPost, pathServiceAccess, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}

	var response AccessServiceResponseBody
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return &AccessServiceResponse{
		Body:           response,
		HTTPStatusCode: httpStatus,
	}, nil
}

// DeleteAccessService 删除服务接入
// resourceId: 资源ID
func (c *Client) DeleteAccessService(ctx context.Context, resourceId string) (*DeleteAccessServiceResponse, error) {
	path := fmt.Sprintf(pathServiceAccessDelete, resourceId)

	respBody, httpStatus, err := c.DoRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}

	var response BaseResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return &DeleteAccessServiceResponse{
		Body:           response,
		HTTPStatusCode: httpStatus,
	}, nil
}

// GetAccessService 查询服务接入详情
func (c *Client) GetAccessService(ctx context.Context, resourceId string) (*GetAccessServiceResponse, error) {
	path := fmt.Sprintf(pathServiceAccessGet, resourceId)

	respBody, httpStatus, err := c.DoRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}

	var response GetAccessServiceResponseBody
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return &GetAccessServiceResponse{
		Body:           response,
		HTTPStatusCode: httpStatus,
	}, nil
}

// IsNotExist 根据响应信息判断是否sni-proxy资源为不存在
func IsNotExist(code int) bool {
	return code == statusCodeNoExist || code == statusCodeNoStaticResource
}
