/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	recordTypeA = "A"
)

// CreateIntranetDnsDomain 发送创建 lbm-dns 记录的 HTTP 请求。
func (c *Client) CreateIntranetDnsDomain(ctx context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (*AsyncTaskResponse, error) {
	reqBody := &IntranetDnsDomainResource{
		RegionCode:   regionCode,
		ServiceName:  serviceName,
		HostRecord:   hostRecord,
		DomainSuffix: domainSuffix,
		RecordValues: []IntranetDnsRecordValue{
			{
				RecordType:  recordTypeA,
				RecordValue: ip,
			},
		},
	}

	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal DNS request failed: %w", marshalErr)
	}

	respBytes, statusCode, err := c.doRequest(ctx, http.MethodPost, pathCreateIntranetDnsDomain,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("send create DNS record request failed: %w", err)
	}

	var body AsyncTaskResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS response failed: %w", unmarshalErr)
	}

	return &AsyncTaskResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// GetIntranetDnsDomainTaskStatus 查询 lbm-dns 任务状态。
func (c *Client) GetIntranetDnsDomainTaskStatus(ctx context.Context,
	taskId string) (*GetIntranetDnsDomainTaskStatusResponse, error) {
	url := fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, taskId)

	respBytes, statusCode, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query task status failed: %w", err)
	}

	var body GetIntranetDnsDomainTaskStatusResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal task status response failed: %w", unmarshalErr)
	}

	return &GetIntranetDnsDomainTaskStatusResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// GetIntranetDnsDomain 查询 lbm-dns 记录详情。
func (c *Client) GetIntranetDnsDomain(ctx context.Context, resourceId string) (*GetIntranetDnsDomainResponse, error) {
	url := fmt.Sprintf(pathIntranetDnsDomainResource, resourceId)

	respBytes, statusCode, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query DNS record failed: %w", err)
	}

	var body GetIntranetDnsDomainResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS response failed: %w", unmarshalErr)
	}

	return &GetIntranetDnsDomainResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// UpdateIntranetDnsDomain 发送更新 lbm-dns 记录值的 HTTP 请求。
func (c *Client) UpdateIntranetDnsDomain(ctx context.Context,
	resourceId, ip string) (*AsyncTaskResponse, error) {
	url := fmt.Sprintf(pathIntranetDnsDomainResource, resourceId)
	reqBody := &IntranetDnsDomainRecordValues{
		RecordValues: []IntranetDnsRecordValue{
			{
				RecordType:  recordTypeA,
				RecordValue: ip,
			},
		},
	}

	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal DNS update request failed: %w", marshalErr)
	}

	respBytes, statusCode, err := c.doRequest(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("send update DNS record request failed: %w", err)
	}

	var body AsyncTaskResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS update response failed: %w", unmarshalErr)
	}

	return &AsyncTaskResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// DeleteIntranetDnsDomain 发送删除 lbm-dns 记录的 HTTP 请求。
func (c *Client) DeleteIntranetDnsDomain(ctx context.Context,
	resourceId string) (*AsyncTaskResponse, error) {
	url := fmt.Sprintf(pathIntranetDnsDomainResource, resourceId)

	respBytes, statusCode, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("send delete DNS record request failed: %w", err)
	}

	var body AsyncTaskResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS delete response failed: %w", unmarshalErr)
	}

	return &AsyncTaskResponse{Body: body, HTTPStatusCode: statusCode}, nil
}
