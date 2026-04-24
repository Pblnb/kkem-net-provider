/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

const (
	recordTypeA = "A"
)

// CreateIntranetDnsDomain 发送创建 lbm-dns 记录的 HTTP 请求。
func (c *Client) CreateIntranetDnsDomain(ctx context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (*CreateIntranetDnsDomainResponse, error) {
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

	var respBytes []byte
	var statusCode int
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var doErr error
		respBytes, statusCode, doErr = c.doRequest(ctx, actionPost, pathCreateIntranetDnsDomain,
			bytes.NewReader(bodyBytes))
		return doErr
	})
	if err != nil {
		return nil, fmt.Errorf("send create DNS record request failed: %w", err)
	}

	var body CreateIntranetDnsDomainResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS response failed: %w", unmarshalErr)
	}

	return &CreateIntranetDnsDomainResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// GetIntranetDnsDomainTaskStatus 查询 lbm-dns 任务状态。
func (c *Client) GetIntranetDnsDomainTaskStatus(ctx context.Context,
	taskId string) (*GetIntranetDnsDomainTaskStatusResponse, error) {
	url := fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, taskId)

	var respBytes []byte
	var statusCode int
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var doErr error
		respBytes, statusCode, doErr = c.doRequest(ctx, actionGet, url, nil)
		return doErr
	})
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

	var respBytes []byte
	var statusCode int
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var doErr error
		respBytes, statusCode, doErr = c.doRequest(ctx, actionGet, url, nil)
		return doErr
	})
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
	resourceId, ip string) (*UpdateIntranetDnsDomainResponse, error) {
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

	var respBytes []byte
	var statusCode int
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var doErr error
		respBytes, statusCode, doErr = c.doRequest(ctx, actionPut, url, bytes.NewReader(bodyBytes))
		return doErr
	})
	if err != nil {
		return nil, fmt.Errorf("send update DNS record request failed: %w", err)
	}

	var body UpdateIntranetDnsDomainResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS update response failed: %w", unmarshalErr)
	}

	return &UpdateIntranetDnsDomainResponse{Body: body, HTTPStatusCode: statusCode}, nil
}

// DeleteIntranetDnsDomain 发送删除 lbm-dns 记录的 HTTP 请求。
func (c *Client) DeleteIntranetDnsDomain(ctx context.Context,
	resourceId string) (*DeleteIntranetDnsDomainResponse, error) {
	url := fmt.Sprintf(pathIntranetDnsDomainResource, resourceId)

	var respBytes []byte
	var statusCode int
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var doErr error
		respBytes, statusCode, doErr = c.doRequest(ctx, actionDelete, url, nil)
		return doErr
	})
	if err != nil {
		return nil, fmt.Errorf("send delete DNS record request failed: %w", err)
	}

	var body DeleteIntranetDnsDomainResponseBody
	if unmarshalErr := json.Unmarshal(respBytes, &body); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal DNS delete response failed: %w", unmarshalErr)
	}

	return &DeleteIntranetDnsDomainResponse{Body: body, HTTPStatusCode: statusCode}, nil
}
