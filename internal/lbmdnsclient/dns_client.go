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

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	recordTypeA     = "A"
	pollingInterval = 3 * time.Second
	pollingTimeout  = 2 * time.Minute
)

// CreateLbmDnsRecord 创建 lbm-dns 记录
func (c *Client) CreateLbmDnsRecord(ctx context.Context,
	regionCode, serviceName, hostRecord, domainSuffix, ip string) (string, error) {
	tflog.Debug(ctx, "Creating lbm-dns record", map[string]any{
		"region_code":   regionCode,
		"service_name":  serviceName,
		"host_record":   hostRecord,
		"domain_suffix": domainSuffix,
		"ip":            ip,
	})

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

	attr := &clientAttr{
		Token: c.token,
		Host:  c.endpoint,
	}

	respBytes, statusCode, err := c.doRequest(ctx, attr, actionPost, pathCreateIntranetDnsDomain,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("send create DNS record request failed: %w", err)
	}

	// HTTP 状态码非 2XX 视为失败
	if statusCode < 200 || statusCode >= 300 {
		return "", fmt.Errorf("create DNS record failed: httpStatusCode=%d, body=%s", statusCode, string(respBytes))
	}

	var resp domainChangeResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		tflog.Warn(ctx, "unmarshal DNS response failed, raw body", map[string]any{
			"body": string(respBytes),
		})
		return "", fmt.Errorf("unmarshal DNS response failed: %w", err)
	}

	// 业务状态码判断：只有 status=0 表示成功
	if resp.Status != statusCodeSuccess {
		return "", fmt.Errorf("create DNS record failed: status=%d, code=%d, errMsg=%s", resp.Status, resp.Code,
			resp.ErrMsg)
	}

	if resp.TaskId == "" {
		return "", fmt.Errorf("create DNS record response has no task_id")
	}

	tflog.Info(ctx, "lbm-dns record creation task started", map[string]any{
		"task_id": resp.TaskId,
	})

	recordId, err := c.waitForTask(ctx, resp.TaskId)
	if err != nil {
		return "", fmt.Errorf("wait for DNS record creation failed: %w", err)
	}

	tflog.Info(ctx, "lbm-dns record created", map[string]any{
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
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for DNS record: %s", ctx.Err())
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for DNS record creation task: %s", taskId)
		case <-ticker.C:
			respBytes, statusCode, err := c.doRequest(ctx, attr, actionGet, url, nil)
			// cmt: 如果 HTTP 请求临时失败（网络抖动、服务端限流等），会立即返回错误，不应该因为一次查询失败就放弃整个操作 应该重试几次
			// 我的建议： 这里只需要 continue 是不是就 OK 了？
			if err != nil {
				return "", fmt.Errorf("query task status failed: %w", err)
			}

			// HTTP 状态码非 2XX 也视为失败
			if statusCode < 200 || statusCode >= 300 {
				return "", fmt.Errorf("query task status failed: httpStatusCode=%d, body=%s", statusCode,
					string(respBytes))
			}

			var resp domainTaskStatusResponse
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				tflog.Warn(ctx, "unmarshal task status response failed, raw body", map[string]any{
					"body": string(respBytes),
				})
				return "", fmt.Errorf("unmarshal task status response failed: %w", err)
			}

			// 业务状态码判断：status=0 表示成功
			if resp.Status != statusCodeSuccess {
				return "", fmt.Errorf("query task status failed: status=%d, code=%d, errMsg=%s", resp.Status, resp.Code,
					resp.ErrMsg)
			}

			status := resp.Data.Status
			tflog.Debug(ctx, "DNS record creation task status check", map[string]any{
				"task_id": taskId,
				"status":  status,
			})

			switch status {
			case taskStatusSuccess:
				if resp.Data.ResourceId == "" {
					return "", fmt.Errorf("task completed but no resource_id returned")
				}
				return resp.Data.ResourceId, nil
			case taskStatusFailed:
				return "", fmt.Errorf("dns record creation task failed: %s", resp.Data.Message)
			default:
				continue
			}
		}
	}
}
