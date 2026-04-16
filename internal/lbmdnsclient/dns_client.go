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

// CreateLbmDnsRecord 发送创建 lbm-dns 记录的 HTTP 请求。
// cmt: [建议] 有一种代码坏味道叫做“基本类型偏执”，从这个函数的返回值是看不出来int是要返回什么，只猜是StatusCode。
// go有没有类型别名？或者go的http库中，有没有对StatusCode的封装类型？
// 我的理解：这里是不是可以用命名返回值解决？还是说用类型别名解决？
func (c *Client) CreateLbmDnsRecord(ctx context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (*domainChangeResponse, int, error) {
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

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal DNS request failed: %w", err)
	}

	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, statusCode, err := c.doRequest(ctx, attr, actionPost, pathCreateIntranetDnsDomain,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, statusCode, fmt.Errorf("send create DNS record request failed: %w", err)
	}

	var resp domainChangeResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, statusCode, fmt.Errorf("unmarshal DNS response failed: %w", err)
	}

	return &resp, statusCode, nil
}

// GetLbmDnsTaskStatus 查询 lbm-dns 任务状态。
func (c *Client) GetLbmDnsTaskStatus(ctx context.Context, taskId string) (*domainTaskStatusResponse, int, error) {
	url := fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, taskId)
	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, statusCode, err := c.doRequest(ctx, attr, actionGet, url, nil)
	if err != nil {
		return nil, statusCode, fmt.Errorf("query task status failed: %w", err)
	}

	var resp domainTaskStatusResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, statusCode, fmt.Errorf("unmarshal task status response failed: %w", err)
	}

	return &resp, statusCode, nil
}
