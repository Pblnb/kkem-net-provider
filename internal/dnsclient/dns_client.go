/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package dnsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	recordTypeA       = "A"
	pollingInterval   = 3 * time.Second
	pollingTimeout    = 2 * time.Minute
	taskStatusOK      = "ok"
	taskStatusSuccess = "success"
)

// Client 内网 DNS Client，封装 HTTP 调用与异步任务轮询
type Client struct {
	endpoint string
	token    string
}

// NewClient 创建 DNS Client
// endpoint: DNS 服务地址（如 https://dns.example.com）
// token: x-open-token 认证令牌
func NewClient(endpoint, token string) *Client {
	return &Client{
		endpoint: endpoint,
		token:    token,
	}
}

// CreateIntranetRecord 创建内网 DNS 记录
// 参数：
//   - regionCode: 区域代码（如 cn-north-7）
//   - serviceName: 服务名称标识
//   - hostRecord: 主机记录（域名前缀）
//   - domainSuffix: 域名后缀
//   - ip: 记录值（VPCEP-Client IP）
//
// 返回：DNS 记录 ID、错误
func (c *Client) CreateIntranetRecord(ctx context.Context, regionCode, serviceName, hostRecord, domainSuffix, ip string) (string, error) {
	tflog.Debug(ctx, "Creating intranet DNS record", map[string]any{
		"region_code":   regionCode,
		"service_name":  serviceName,
		"host_record":   hostRecord,
		"domain_suffix": domainSuffix,
		"ip":            ip,
	})

	// 构造请求体
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
		return "", fmt.Errorf("marshal DNS request failed: %w", err)
	}

	// 发送创建请求
	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, err := sendHTTP(attr, actionPost, pathCreateIntranetDnsDomain, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("send create DNS record request failed: %w", err)
	}

	// 解析响应获取 TaskId
	var resp domainChangeResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("unmarshal DNS response failed: %w", err)
	}

	if resp.Status != 200 && resp.Status != 0 {
		return "", fmt.Errorf("create DNS record failed: status=%d, code=%d, msg=%s", resp.Status, resp.Code, resp.Msg)
	}

	if resp.TaskId == "" {
		return "", fmt.Errorf("create DNS record response has no task_id")
	}

	tflog.Info(ctx, "Intranet DNS record creation task started", map[string]any{
		"task_id": resp.TaskId,
	})

	// 轮询等待任务完成
	recordId, err := c.waitForTask(ctx, resp.TaskId)
	if err != nil {
		return "", fmt.Errorf("wait for DNS record creation failed: %w", err)
	}

	tflog.Info(ctx, "Intranet DNS record created", map[string]any{
		"record_id": recordId,
	})

	return recordId, nil
}

// waitForTask 轮询等待任务完成，返回 DNS 记录 ID
func (c *Client) waitForTask(ctx context.Context, taskId string) (string, error) {
	timeout := time.After(pollingTimeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	url := fmt.Sprintf(pathGetIntranetDnsDomainTaskStatus, taskId)
	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for DNS record creation task: %s", taskId)
		case <-ticker.C:
			respBytes, err := sendHTTP(attr, actionGet, url, nil)
			if err != nil {
				return "", fmt.Errorf("query task status failed: %w", err)
			}

			var resp domainTaskStatusResponse
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				return "", fmt.Errorf("unmarshal task status response failed: %w", err)
			}

			if resp.Status != 200 && resp.Status != 0 {
				return "", fmt.Errorf("query task status failed: status=%d, code=%d, msg=%s", resp.Status, resp.Code, resp.Msg)
			}

			status := resp.Data.Status
			tflog.Debug(ctx, "DNS record creation task status check", map[string]any{
				"task_id": taskId,
				"status":  status,
			})

			switch status {
			case taskStatusOK, taskStatusSuccess:
				if resp.Data.ResourceId == "" {
					return "", fmt.Errorf("task completed but no resource_id returned")
				}
				return resp.Data.ResourceId, nil
			case taskStatusFailed:
				return "", fmt.Errorf("dns record creation task failed: %s", resp.Data.ErrorMessage)
			case taskStatusCancel:
				return "", fmt.Errorf("dns record creation task was cancelled")
			default:
				// 继续轮询
				continue
			}
		}
	}
}
