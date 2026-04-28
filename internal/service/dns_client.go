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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

type Dnsclient interface {
	CreatePrivateZone(request *model.CreatePrivateZoneRequest) (*model.CreatePrivateZoneResponse, error)
	ShowPrivateZone(request *model.ShowPrivateZoneRequest) (*model.ShowPrivateZoneResponse, error)
	DeletePrivateZone(request *model.DeletePrivateZoneRequest) (*model.DeletePrivateZoneResponse, error)

	CreateRecordSetWithLine(request *model.CreateRecordSetWithLineRequest) (*model.CreateRecordSetWithLineResponse, error)
	ShowRecordSetWithLine(request *model.ShowRecordSetWithLineRequest) (*model.ShowRecordSetWithLineResponse, error)
}

type PrivateZoneInput struct {
	DomainName   string
	DomainRouter model.Router
}

// CreatePrivateZone 创建 PrivateZone
func CreatePrivateZone(
	ctx context.Context,
	client Dnsclient,
	input PrivateZoneInput,
) (string, error) {
	createReq := &model.CreatePrivateZoneRequest{
		Body: &model.CreatePrivateZoneReq{
			Name:     input.DomainName,
			ZoneType: DnsZoneType,
			Router:   &input.DomainRouter,
		},
	}

	tflog.Debug(ctx, "Creating intranet domain", map[string]any{
		"Name":   input.DomainName,
		"Router": input.DomainRouter,
	})

	var createResp *model.CreatePrivateZoneResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		createResp, innerErr = client.CreatePrivateZone(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreatePrivateZone API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("CreatePrivateZone API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("CreatePrivateZone response has no ID")
	}

	tflog.Info(ctx, "PrivateZone created", map[string]any{
		"privatezone_id": *createResp.Id,
		"status":         *createResp.Status,
	})

	return *createResp.Id, nil
}

// DeletePrivateZone 删除 PrivateZone
func DeletePrivateZone(
	ctx context.Context,
	client Dnsclient,
	PrivateZoneId string,
) error {
	deleteReq := &model.DeletePrivateZoneRequest{
		ZoneId: PrivateZoneId,
	}

	tflog.Debug(ctx, "Deleting PrivateZone", map[string]any{
		"PrivateZoneId": PrivateZoneId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := client.DeletePrivateZone(deleteReq)
		if innerErr != nil {
			if status.Code(innerErr) == codes.NotFound {
				return nil
			}
			tflog.Warn(ctx, "DeletePrivateZone API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "PrivateZone deleted", map[string]any{
		"PrivateZoneId": PrivateZoneId,
	})
	return nil
}

// CreateRecordSetWithLine 创建带线路解析的记录集
func CreateRecordSetWithLine(ctx context.Context,
	client Dnsclient,
	privateZoneId string,
	domainName string,
	clientip string,
) (string, error) {
	createReq := &model.CreateRecordSetWithLineRequest{
		ZoneId: privateZoneId,
		Body: &model.CreateRecordSetWithLineRequestBody{
			Name:    domainName,
			Type:    RecordSet,
			Records: &[]string{clientip},
		},
	}

	tflog.Debug(ctx, "Creating RecordSetWithLine", map[string]any{
		"ZoneId": privateZoneId,
	})

	var createResp *model.CreateRecordSetWithLineResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var err error
		createResp, err = client.CreateRecordSetWithLine(createReq)
		if err != nil {
			if utils.IsDnsNotFoundError(err) {
				return err
			}
			tflog.Warn(ctx, "CreateRecordSetWithLine API failed, retrying", map[string]any{
				"error": err.Error(),
			})
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	tflog.Info(ctx, "RecordSetWithLine created/exists", map[string]any{
		"RecordSetId": createResp.Id,
	})
	return *createResp.Id, nil
}

// ShowRecordSetWithLine 查询记录集
func ShowRecordSetWithLine(
	ctx context.Context,
	client Dnsclient,
	privateZoneId string,
	recordSetId string,
) (string, error) {
	showReq := &model.ShowRecordSetWithLineRequest{
		ZoneId:      privateZoneId,
		RecordsetId: recordSetId,
	}

	tflog.Debug(ctx, "Showing RecordSetWithLine", map[string]any{
		"ZoneId":      privateZoneId,
		"RecordSetId": recordSetId,
	})

	var showResp *model.ShowRecordSetWithLineResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var err error
		showResp, err = client.ShowRecordSetWithLine(showReq)
		if err != nil {
			if utils.IsDnsNotFoundError(err) {
				return err
			}
			tflog.Warn(ctx, "ShowRecordSetWithLine API failed, retrying", map[string]any{
				"error": err.Error(),
			})
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if showResp.Id == nil {
		return "", fmt.Errorf("ShowRecordSetWithLine response has no ID")
	}
	tflog.Info(ctx, "RecordSetWithLine exists", map[string]any{
		"ZoneId":      privateZoneId,
		"RecordSetId": recordSetId,
	})
	return *showResp.Id, nil
}

func WaitForIntranetDomainReady(
	ctx context.Context,
	client Dnsclient,
	PrivateZoneId string,
) error {
	timeout := time.After(PollingTimeout)
	ticker := time.NewTicker(PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for intranet domain %s", PrivateZoneId)
		case <-timeout:
			return fmt.Errorf("timeout waiting for intranet domain %s to be ready", PrivateZoneId)
		case <-ticker.C:
			showReq := &model.ShowPrivateZoneRequest{
				ZoneId: PrivateZoneId,
			}

			showResp, err := client.ShowPrivateZone(showReq)
			if err != nil {
				return fmt.Errorf("query intranet domain status failed: %w", err)
			}

			if showResp.Status == nil {
				return fmt.Errorf("intranet domain response has no status")
			}

			status := *showResp.Status
			tflog.Debug(ctx, "intranet domain status check", map[string]any{
				"PrivateZoneId": PrivateZoneId,
				"status":        status,
			})

			switch status {
			case "ACTIVE":
				tflog.Info(ctx, "intranet domain is ready", map[string]any{
					"PrivateZoneId": PrivateZoneId,
					"status":        status,
				})
				return nil
			case "ERROR", "PENDING_DISABLE":
				return fmt.Errorf("intranet domain %s status is %s", PrivateZoneId, status)
			case "PENDING_CREATE", "PENDING_UPDATE", "PENDING_DELETE":
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "intranet domain unknown status", map[string]any{
					"PrivateZoneId": PrivateZoneId,
					"status":        status,
				})
			}
		}
	}
}
