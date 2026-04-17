/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

const (
	recordTypeA = "A"
)

// CreateIntranetDnsDomain 发送创建 lbm-dns 记录的 HTTP 请求。
func (c *Client) CreateIntranetDnsDomain(ctx context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (*CreateIntranetDnsDomainResponse, error) {
	reqBody := &intranetDnsDomainResource{
		RegionCode:   regionCode,
		ServiceName:  serviceName,
		HostRecord:   hostRecord,
		DomainSuffix: domainSuffix,
		RecordValues: []recordValue{
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

	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, statusCode, doErr := c.doRequest(ctx, attr, actionPost, pathCreateIntranetDnsDomain,
		bytes.NewReader(bodyBytes))
	if doErr != nil {
		return nil, fmt.Errorf("send create DNS record request failed: %w", doErr)
	}

	var body CreateIntranetDnsDomainResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS response failed: %w", unmarshalErr)
	}

	return &CreateIntranetDnsDomainResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// GetIntranetDnsDomainTaskStatus 查询 lbm-dns 任务状态。
func (c *Client) GetIntranetDnsDomainTaskStatus(ctx context.Context, taskId string) (*GetIntranetDnsDomainTaskStatusResponse, error) {
	url := fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, taskId)
	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, statusCode, doErr := c.doRequest(ctx, attr, actionGet, url, nil)
	if doErr != nil {
		return nil, fmt.Errorf("query task status failed: %w", doErr)
	}

	var body GetIntranetDnsDomainTaskStatusResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal task status response failed: %w", unmarshalErr)
	}

	return &GetIntranetDnsDomainTaskStatusResponse{Body: body, HTTPStatusCode: statusCode}, nil
}
