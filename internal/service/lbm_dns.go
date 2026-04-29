/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"huawei.com/kkem/kkem-net-provider/internal/client/lbmdnsclient"
)

// LbmDnsService - LBM-DNS service 层
type LbmDnsService struct {
	client lbmdnsclient.LbmDnsClient
}

// NewLbmDnsService - 构造函数
func NewLbmDnsService(client lbmdnsclient.LbmDnsClient) *LbmDnsService {
	return &LbmDnsService{client: client}
}

// CreateLbmDnsInput - 创建 lbm-dns 记录的输入参数
type CreateLbmDnsInput struct {
	RegionCode   string
	ServiceName  string
	HostRecord   string
	DomainSuffix string
	EndpointIp   string
}

// LbmDnsRecordValue - lbm-dns 记录值
type LbmDnsRecordValue struct {
	RecordType  string
	RecordValue string
}

// CreateLbmDnsOutput - 创建 lbm-dns 记录的输出
type CreateLbmDnsOutput struct {
	RecordId     string
	RecordValues []LbmDnsRecordValue
}

// CreateIntranetDnsDomain - 创建 IntranetDnsDomain 记录并等待完成
func (s *LbmDnsService) CreateIntranetDnsDomain(ctx context.Context, input CreateLbmDnsInput) (*CreateLbmDnsOutput,
	error) {
	if s.client == nil {
		return nil, fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Creating IntranetDnsDomain record", map[string]any{
		"region_code":   input.RegionCode,
		"service_name":  input.ServiceName,
		"host_record":   input.HostRecord,
		"domain_suffix": input.DomainSuffix,
		"endpoint_ip":   input.EndpointIp,
	})

	var resp *lbmdnsclient.AsyncTaskResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.CreateIntranetDnsDomain(ctx, input.RegionCode, input.ServiceName,
			input.HostRecord, input.DomainSuffix, input.EndpointIp)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateIntranetDnsDomain API failed, retrying", map[string]any{"error": innerErr.Error()})
		}
		return innerErr
	})
	if err != nil {
		return nil, fmt.Errorf("create IntranetDnsDomain record failed after retries: %w", err)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return nil, fmt.Errorf("create DNS record failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}
	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return nil, fmt.Errorf("create DNS record failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
			resp.Body.Code, resp.Body.ErrMsg)
	}
	if resp.Body.TaskId == "" {
		return nil, fmt.Errorf("create DNS record response has no task_id")
	}
	taskId := resp.Body.TaskId

	tflog.Info(ctx, "lbm-dns record creation task started", map[string]any{"task_id": taskId})

	recordId, err := s.waitForLbmDnsRecordReady(ctx, taskId)
	if err != nil {
		return nil, err
	}

	return &CreateLbmDnsOutput{
		RecordId:     recordId,
		RecordValues: []LbmDnsRecordValue{{RecordType: "A", RecordValue: input.EndpointIp}},
	}, nil
}

// waitForLbmDnsRecordReady 轮询等待 lbm-dns 记录创建完成，返回 DNS 记录 ID
func (s *LbmDnsService) waitForLbmDnsRecordReady(ctx context.Context, taskId string) (string, error) {
	resp, err := s.waitForTaskCompleted(ctx, taskId, "DNS record creation")
	if err != nil {
		return "", err
	}
	if resp.Body.Data.ResourceId == "" {
		return "", fmt.Errorf("task completed but no resource_id returned")
	}
	return resp.Body.Data.ResourceId, nil
}

// waitForTaskCompleted 轮询等待 lbm-dns 异步任务完成
func (s *LbmDnsService) waitForTaskCompleted(ctx context.Context,
	taskId, taskName string) (*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, error) {
	timeout := time.After(pollingTimeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for %s: %s", taskName, ctx.Err())
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for %s task: %s", taskName, taskId)
		case <-ticker.C:
			resp, err := s.client.GetIntranetDnsDomainTaskStatus(ctx, taskId)
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return nil, fmt.Errorf("query lbm-dns task status failed: %w", err)
				}
				tflog.Warn(ctx, "query lbm-dns task failed, will retry", map[string]any{
					"task_id":   taskId,
					"error":     err.Error(),
					"err_count": errCount,
				})
				continue
			}
			errCount = 0

			if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
				return nil, fmt.Errorf("query lbm-dns task status failed, http status is %d", resp.HTTPStatusCode)
			}
			if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
				return nil, fmt.Errorf("query task status failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
					resp.Body.Code, resp.Body.ErrMsg)
			}

			status := resp.Body.Data.Status
			tflog.Debug(ctx, taskName+" task status check", map[string]any{"task_id": taskId, "status": status})

			switch status {
			case lbmdnsclient.TaskStatusSuccess:
				return resp, nil
			case lbmdnsclient.TaskStatusFailed:
				return nil, fmt.Errorf("%s task failed: %s", taskName, resp.Body.Data.Message)
			default:
				// running / pending 等中间状态：继续轮询
				continue
			}
		}
	}
}

// DeleteIntranetDnsDomain - 删除 IntranetDnsDomain 记录
func (s *LbmDnsService) DeleteIntranetDnsDomain(ctx context.Context, recordId string) error {
	if s.client == nil {
		return fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Deleting IntranetDnsDomain record", map[string]any{
		"dns_record_id": recordId,
	})

	var resp *lbmdnsclient.AsyncTaskResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.DeleteIntranetDnsDomain(ctx, recordId)
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("call DeleteIntranetDnsDomain API failed: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("response is nil for record %s", recordId)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return fmt.Errorf("httpStatusCode=%d", resp.HTTPStatusCode)
	}

	if lbmdnsclient.IsNotFound(resp.Body.Code) {
		tflog.Info(ctx, "call DeleteIntranetDnsDomain API successfully, but record already have been deleted",
			map[string]any{
				"dns_record_id":        recordId,
				"lbm_response_status":  resp.Body.Status,
				"lbm_response_code":    resp.Body.Code,
				"lbm_response_err_msg": resp.Body.ErrMsg,
			})
		return nil
	}

	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return fmt.Errorf("response from lbm dns server contains unsuccessful code: status=%d, code=%d, errMsg=%s",
			resp.Body.Status, resp.Body.Code, resp.Body.ErrMsg)
	}

	taskId := resp.Body.TaskId
	if taskId == "" {
		return errors.New("delete IntranetDnsDomain record response has no task id")
	}

	if _, err := s.waitForTaskCompleted(ctx, taskId, "IntranetDnsDomain record deletion"); err != nil {
		return err
	}
	tflog.Info(ctx, "IntranetDnsDomain record deleted", map[string]any{
		"dns_record_id": recordId,
		"task_id":       taskId,
	})
	return nil
}

// UpdateRecordValue - 更新 DNS 记录的 IP
func (s *LbmDnsService) UpdateRecordValue(ctx context.Context, recordId, endpointIp string) error {
	if s.client == nil {
		return fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Updating lbm-dns record values", map[string]any{
		"dns_record_id": recordId,
		"endpoint_ip":   endpointIp,
	})

	var resp *lbmdnsclient.AsyncTaskResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.UpdateIntranetDnsDomain(ctx, recordId, endpointIp)
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("call UpdateIntranetDnsDomain API failed: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("response is nil for record %s", recordId)
	}
	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return fmt.Errorf("update DNS record failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}
	if isLbmDnsNoChanges(resp.Body.Status, resp.Body.Code, resp.Body.ErrMsg) {
		tflog.Info(ctx, "lbm-dns record values already up to date", map[string]any{
			"dns_record_id": recordId,
		})
		return nil
	}
	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return fmt.Errorf("update DNS record failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
			resp.Body.Code, resp.Body.ErrMsg)
	}
	if resp.Body.TaskId == "" {
		return errors.New("update lbm-dns record response has no task id")
	}
	if _, err := s.waitForTaskCompleted(ctx, resp.Body.TaskId, "DNS record update"); err != nil {
		return err
	}
	tflog.Info(ctx, "lbm-dns record values updated", map[string]any{
		"dns_record_id": recordId,
		"task_id":       resp.Body.TaskId,
	})
	return nil
}

func isLbmDnsNoChanges(status, code int, msg string) bool {
	return status == lbmdnsclient.StatusCodeNoChanges && code == lbmdnsclient.StatusCodeNoChanges &&
		strings.Contains(msg, "No changes")
}

// getLbmDnsRawResponse 执行 DNS 查询的 API 调用和公共校验，返回原始响应数据。
// 记录不存在时返回 nil, nil。
func (s *LbmDnsService) getLbmDnsRawResponse(ctx context.Context,
	recordId string) (*lbmdnsclient.IntranetDnsDomainResource, error) {
	if s.client == nil {
		return nil, fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	var resp *lbmdnsclient.GetIntranetDnsDomainResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.GetIntranetDnsDomain(ctx, recordId)
		return innerErr
	})
	if err != nil {
		return nil, fmt.Errorf("call GetIntranetDnsDomain API failed: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("response is nil for record %s", recordId)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return nil, fmt.Errorf("query DNS record failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}

	if lbmdnsclient.IsNotFound(resp.Body.Code) {
		return nil, nil
	}

	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return nil, fmt.Errorf("query DNS record failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
			resp.Body.Code, resp.Body.ErrMsg)
	}

	return resp.Body.Data, nil
}

// extractLbmDnsRecordValues 从原始数据中提取记录值列表。
func extractLbmDnsRecordValues(data *lbmdnsclient.IntranetDnsDomainResource) []LbmDnsRecordValue {
	recordValues := make([]LbmDnsRecordValue, len(data.RecordValues))
	for i, rv := range data.RecordValues {
		recordValues[i] = LbmDnsRecordValue{
			RecordType:  rv.RecordType,
			RecordValue: rv.RecordValue,
		}
	}
	return recordValues
}

// GetRecord - 查询 DNS 记录详情，不存在时返回 nil, nil
func (s *LbmDnsService) GetRecord(ctx context.Context, recordId string) (*CreateLbmDnsOutput, error) {
	tflog.Debug(ctx, "Querying lbm-dns record", map[string]any{
		"dns_record_id": recordId,
	})

	data, err := s.getLbmDnsRawResponse(ctx, recordId)
	if err != nil {
		return nil, err
	}
	if data == nil {
		tflog.Info(ctx, "DNS record not found", map[string]any{
			"dns_record_id": recordId,
		})
		return nil, nil
	}

	return &CreateLbmDnsOutput{
		RecordId:     recordId,
		RecordValues: extractLbmDnsRecordValues(data),
	}, nil
}

// GetLbmDnsDetail - 查询 DNS 记录的详细信息（含 RegionCode、ServiceName 等输入属性），用于 Read 回填
func (s *LbmDnsService) GetLbmDnsDetail(ctx context.Context, recordId string) (*LbmDnsDetailOutput, error) {
	data, err := s.getLbmDnsRawResponse(ctx, recordId)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	return &LbmDnsDetailOutput{
		RecordId:     recordId,
		RegionCode:   data.RegionCode,
		ServiceName:  data.ServiceName,
		HostRecord:   data.HostRecord,
		DomainSuffix: data.DomainSuffix,
		RecordValues: extractLbmDnsRecordValues(data),
	}, nil
}

// LbmDnsDetailOutput - 包含输入属性的 DNS 记录详情输出
type LbmDnsDetailOutput struct {
	RecordId     string
	RegionCode   string
	ServiceName  string
	HostRecord   string
	DomainSuffix string
	RecordValues []LbmDnsRecordValue
}
