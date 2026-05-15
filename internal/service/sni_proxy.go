/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
)

const (
	changeReason    = "Create network connection from M3 to M1"
	sniAccessObject = "APIGW"
)

// SniProxyService - SNI Proxy service 层
type SniProxyService struct {
	client          sniproxyclient.SniProxyClient
	pollingInterval time.Duration
	pollingTimeout  time.Duration
}

// NewSniProxyService - 构造函数
func NewSniProxyService(client sniproxyclient.SniProxyClient) *SniProxyService {
	return &SniProxyService{
		client:          client,
		pollingInterval: pollingInterval,
		pollingTimeout:  pollingTimeout,
	}
}

// AccessSniProxyInput - 接入 SNI Proxy 的输入参数
type AccessSniProxyInput struct {
	RegionCode       string
	ServiceName      string
	IamDomainAccount []string
}

// AccessSniProxyOutput - 接入 SNI Proxy 的输出
type AccessSniProxyOutput struct {
	ResourceId       string
	ServiceName      string
	AccessObject     string
	RegionCode       string
	IamDomainAccount []string
	EpServiceIds     []string
}

// AccessSniProxy - 接入 SNI Proxy 服务并等待就绪
func (s *SniProxyService) AccessSniProxy(ctx context.Context, input AccessSniProxyInput) (string, error) {
	if s.client == nil {
		return "", errors.New("sni proxy client is not initialized")
	}

	tflog.Debug(ctx, "Accessing SNI Proxy service", map[string]any{
		"region_code":        input.RegionCode,
		"service_name":       input.ServiceName,
		"iam_domain_account": input.IamDomainAccount,
	})

	req := sniproxyclient.AccessServiceRequest{
		RegionCode:       input.RegionCode,
		ServiceName:      input.ServiceName,
		AccessObject:     sniAccessObject,
		IamDomainAccount: input.IamDomainAccount,
		ChangeReason:     changeReason,
	}

	var resp *sniproxyclient.AccessServiceResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.AccessService(ctx, req)
		if innerErr != nil {
			tflog.Warn(ctx, "AccessSniProxy API failed, retrying", map[string]any{"error": innerErr.Error()})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("access SNI Proxy service failed after retries: %w", err)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return "", fmt.Errorf("access SNI Proxy service failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}
	if resp.Body.Code != 0 {
		return "", fmt.Errorf("access SNI Proxy service failed: code=%d, msg=%s", resp.Body.Code, resp.Body.Msg)
	}
	if resp.Body.Data.ResourceId == "" {
		return "", errors.New("access SNI Proxy service response has no resource_id")
	}

	resourceId := resp.Body.Data.ResourceId
	tflog.Info(ctx, "SNI Proxy service access task started", map[string]any{"resource_id": resourceId})

	output, err := s.waitForSniProxyAccessReady(ctx, resourceId)
	if err != nil {
		return "", fmt.Errorf("wait for SNI Proxy access ready failed: %w", err)
	}

	return output.ResourceId, nil
}

// waitForSniProxyAccessReady - 轮询等待 SNI Proxy 接入就绪
func (s *SniProxyService) waitForSniProxyAccessReady(ctx context.Context, resourceId string) (*AccessSniProxyOutput, error) {
	// 立即执行一次检查
	result, err := s.checkAccessReady(ctx, resourceId)
	if err == nil {
		return result, nil
	}

	// Use service fields to support dependency injection for faster testing
	timer := time.NewTimer(s.pollingTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for SNI Proxy access ready, resourceId: %s", resourceId)
		case <-ticker.C:
			result, err := s.checkAccessReady(ctx, resourceId)
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return nil, fmt.Errorf("query SNI Proxy access status failed after retries: %w", err)
				}
				tflog.Warn(ctx, "Query failed, retrying", map[string]any{
					"resource_id": resourceId,
					"error":       err.Error(),
					"err_count":   errCount,
				})
				continue
			}
			errCount = 0
			return result, nil
		}
	}
}

func (s *SniProxyService) checkAccessReady(ctx context.Context, resourceId string) (*AccessSniProxyOutput, error) {
	resp, err := s.client.GetAccessService(ctx, resourceId)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("GetAccessService returned nil response with no error")
	}
	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.HTTPStatusCode)
	}
	if resp.Body.Code != 0 {
		if sniproxyclient.IsNotExist(resp.Body.Code) {
			tflog.Warn(ctx, "SNI Proxy access already has been deleted", map[string]any{
				"resource_id":   resourceId,
				"response_code": resp.Body.Code,
				"response_msg":  resp.Body.Msg,
			})
			return nil, fmt.Errorf("code=%d, msg=%s", resp.Body.Code, resp.Body.Msg)
		}
	}

	epServiceIds := make([]string, len(resp.Body.Data.EpServiceIds))
	for i, ep := range resp.Body.Data.EpServiceIds {
		epServiceIds[i] = ep.EpServiceId
	}
	return &AccessSniProxyOutput{
		ResourceId:       resp.Body.Data.ResourceId,
		ServiceName:      resp.Body.Data.ServiceName,
		AccessObject:     resp.Body.Data.AccessObject,
		RegionCode:       resp.Body.Data.RegionCode,
		IamDomainAccount: resp.Body.Data.IamDomainAccount,
		EpServiceIds:     epServiceIds,
	}, nil
}

// DeleteSniProxy - 删除 SNI Proxy 接入
func (s *SniProxyService) DeleteSniProxy(ctx context.Context, resourceId string) error {
	if resourceId == "" {
		tflog.Info(ctx, "SNI Proxy access deleted", map[string]any{
			"resource_id": resourceId,
		})
		return nil
	}

	if s.client == nil {
		return errors.New("sni proxy client is not initialized")
	}

	tflog.Debug(ctx, "Deleting SNI Proxy access", map[string]any{
		"resource_id": resourceId,
	})

	var resp *sniproxyclient.DeleteAccessServiceResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.DeleteAccessService(ctx, resourceId)
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("call DeleteAccessService API failed: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("response is nil for resource %s", resourceId)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return fmt.Errorf("delete SNI Proxy access failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}

	if resp.Body.Code != 0 {
		return fmt.Errorf("delete SNI Proxy access failed: code=%d, msg=%s", resp.Body.Code, resp.Body.Msg)
	}

	tflog.Info(ctx, "SNI Proxy access deleted", map[string]any{
		"resource_id": resourceId,
	})
	return nil
}

// GetSniProxy - 查询 SNI Proxy 接入详情，不存在时返回 nil, nil
func (s *SniProxyService) GetSniProxy(ctx context.Context, resourceId string) (*AccessSniProxyOutput, *sniproxyclient.GetAccessServiceResponse, error) {
	if s.client == nil {
		return nil, nil, errors.New("sni proxy client is not initialized")
	}

	tflog.Debug(ctx, "Querying SNI Proxy access", map[string]any{
		"resource_id": resourceId,
	})

	var resp *sniproxyclient.GetAccessServiceResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		resp, innerErr = s.client.GetAccessService(ctx, resourceId)
		return innerErr
	})
	if err != nil {
		return nil, nil, fmt.Errorf("call GetAccessService API failed: %w", err)
	}

	if resp == nil {
		return nil, nil, fmt.Errorf("response is nil for resource %s", resourceId)
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return nil, nil, fmt.Errorf("query SNI Proxy access failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}

	if resp.Body.Code != 0 {
		if sniproxyclient.IsNotExist(resp.Body.Code) {
			return nil, resp, fmt.Errorf("query SNI Proxy access failed: code=%d, msg=%s", resp.Body.Code, resp.Body.Msg)
		}
	}

	epServiceIds := make([]string, len(resp.Body.Data.EpServiceIds))
	for i, ep := range resp.Body.Data.EpServiceIds {
		epServiceIds[i] = ep.EpServiceId
	}

	return &AccessSniProxyOutput{
		ResourceId:       resp.Body.Data.ResourceId,
		ServiceName:      resp.Body.Data.ServiceName,
		AccessObject:     resp.Body.Data.AccessObject,
		RegionCode:       resp.Body.Data.RegionCode,
		IamDomainAccount: resp.Body.Data.IamDomainAccount,
		EpServiceIds:     epServiceIds,
	}, nil, nil
}
