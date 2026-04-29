/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
)

const (
	dnsZoneType      = "private"
	dnsRecordSetType = "A"
)

// DnsServiceClient - DNS Service 专用接口，仅暴露 DNS 相关的 SDK 方法。
type DnsServiceClient interface {
	CreatePrivateZone(req *model.CreatePrivateZoneRequest) (*model.CreatePrivateZoneResponse, error)
	ShowPrivateZone(req *model.ShowPrivateZoneRequest) (*model.ShowPrivateZoneResponse, error)
	DeletePrivateZone(req *model.DeletePrivateZoneRequest) (*model.DeletePrivateZoneResponse, error)
	CreateRecordSetWithLine(req *model.CreateRecordSetWithLineRequest) (*model.CreateRecordSetWithLineResponse, error)
	ShowRecordSetWithLine(req *model.ShowRecordSetWithLineRequest) (*model.ShowRecordSetWithLineResponse, error)
}

// DnsService - DNS service 层
type DnsService struct {
	client DnsServiceClient
}

// NewDnsService - 构造函数
func NewDnsService(client DnsServiceClient) *DnsService {
	return &DnsService{client: client}
}

// DnsZoneInput - 创建 Private Zone 的输入参数
type DnsZoneInput struct {
	DomainName string
	RouterId   string
}

// DnsRecordSetInput - 创建 Record Set 的输入参数
type DnsRecordSetInput struct {
	ZoneId  string
	Name    string
	Records []string
}

// DnsZoneOutput - Get Private Zone 的输出结构体
type DnsZoneOutput struct {
	ZoneId string
	Status string
}

// CreatePrivateZone - 创建 Private Zone 并等待就绪
func (s *DnsService) CreatePrivateZone(ctx context.Context, input DnsZoneInput) (string, error) {
	createReq := &model.CreatePrivateZoneRequest{
		Body: &model.CreatePrivateZoneReq{
			Name:     input.DomainName,
			ZoneType: dnsZoneType,
			Router: &model.Router{
				RouterId: input.RouterId,
			},
		},
	}

	tflog.Debug(ctx, "Creating private zone", map[string]any{
		"domain_name": input.DomainName,
		"router_id":   input.RouterId,
	})

	var createResp *model.CreatePrivateZoneResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		createResp, innerErr = s.client.CreatePrivateZone(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreatePrivateZone API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("createPrivateZone API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createPrivateZone response has no ID")
	}

	tflog.Info(ctx, "Private zone created", map[string]any{
		"zone_id": *createResp.Id,
		"status":  *createResp.Status,
	})

	if err := s.waitForZoneReady(ctx, *createResp.Id); err != nil {
		return "", fmt.Errorf("wait for private zone ready failed: %w", err)
	}

	return *createResp.Id, nil
}

// waitForZoneReady 轮询等待 Private Zone 状态变为 ACTIVE
func (s *DnsService) waitForZoneReady(ctx context.Context, zoneId string) error {
	timeout := time.After(pollingTimeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for private zone %s: %s", zoneId, ctx.Err())
		case <-timeout:
			return fmt.Errorf("timeout waiting for private zone %s to be ready", zoneId)
		case <-ticker.C:
			resp, err := s.client.ShowPrivateZone(&model.ShowPrivateZoneRequest{ZoneId: zoneId})
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return fmt.Errorf("query private zone status failed: %w", err)
				}
				tflog.Warn(ctx, "query private zone failed, will retry", map[string]any{
					"zone_id":   zoneId,
					"error":     err.Error(),
					"err_count": errCount,
				})
				continue
			}
			errCount = 0

			if resp.Status == nil {
				return fmt.Errorf("private zone response has no status")
			}

			status := *resp.Status
			tflog.Debug(ctx, "Private zone status check", map[string]any{
				"zone_id": zoneId,
				"status":  status,
			})

			switch status {
			case "ACTIVE":
				tflog.Info(ctx, "Private zone is ready", map[string]any{
					"zone_id": zoneId,
				})
				return nil
			case "ERROR", "PENDING_DISABLE":
				return fmt.Errorf("private zone %s status is %s", zoneId, status)
			case "PENDING_CREATE", "PENDING_UPDATE", "PENDING_DELETE":
				continue
			default:
				tflog.Warn(ctx, "Private zone unknown status", map[string]any{
					"zone_id": zoneId,
					"status":  status,
				})
			}
		}
	}
}

// CreateRecordSet - 创建 Record Set
func (s *DnsService) CreateRecordSet(ctx context.Context, input DnsRecordSetInput) (string, error) {
	createReq := &model.CreateRecordSetWithLineRequest{
		ZoneId: input.ZoneId,
		Body: &model.CreateRecordSetWithLineRequestBody{
			Name:    input.Name,
			Type:    dnsRecordSetType,
			Records: &input.Records,
		},
	}

	tflog.Debug(ctx, "Creating record set", map[string]any{
		"zone_id": input.ZoneId,
		"name":    input.Name,
	})

	var createResp *model.CreateRecordSetWithLineResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		createResp, innerErr = s.client.CreateRecordSetWithLine(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateRecordSetWithLine API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("createRecordSetWithLine API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createRecordSetWithLine response has no ID")
	}

	tflog.Info(ctx, "Record set created", map[string]any{
		"zone_id":       input.ZoneId,
		"record_set_id": *createResp.Id,
	})

	return *createResp.Id, nil
}

// DeletePrivateZone - 删除 Private Zone
func (s *DnsService) DeletePrivateZone(ctx context.Context, zoneId string) error {
	deleteReq := &model.DeletePrivateZoneRequest{
		ZoneId: zoneId,
	}

	tflog.Debug(ctx, "Deleting private zone", map[string]any{
		"zone_id": zoneId,
	})

	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.DeletePrivateZone(deleteReq)
		if innerErr != nil {
			if isDnsNotFoundError(innerErr) {
				return nil
			}
			tflog.Warn(ctx, "DeletePrivateZone API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("deletePrivateZone API failed after retries: %w", err)
	}

	tflog.Info(ctx, "Private zone deleted", map[string]any{
		"zone_id": zoneId,
	})
	return nil
}

// GetPrivateZone - 查询 Private Zone，不存在时返回 nil, nil
func (s *DnsService) GetPrivateZone(ctx context.Context, zoneId string) (*DnsZoneOutput, error) {
	var getResp *model.ShowPrivateZoneResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		getResp, innerErr = s.client.ShowPrivateZone(&model.ShowPrivateZoneRequest{ZoneId: zoneId})
		if isDnsNotFoundError(innerErr) {
			return nil
		}
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	if getResp == nil || getResp.Id == nil {
		return nil, nil
	}

	output := &DnsZoneOutput{ZoneId: zoneId}
	if getResp.Status != nil {
		output.Status = *getResp.Status
	}
	return output, nil
}
