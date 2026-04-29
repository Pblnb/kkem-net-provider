/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
)

// VpcepEndpointClient - VPCEP Endpoint 专用接口，仅暴露 Endpoint 相关的 SDK 方法。
type VpcepEndpointClient interface {
	CreateEndpoint(req *model.CreateEndpointRequest) (*model.CreateEndpointResponse, error)
	DeleteEndpoint(req *model.DeleteEndpointRequest) (*model.DeleteEndpointResponse, error)
	ListEndpointInfoDetails(req *model.ListEndpointInfoDetailsRequest) (*model.ListEndpointInfoDetailsResponse, error)
}

// VpcepEndpointService - VPCEP Endpoint service 层
type VpcepEndpointService struct {
	client VpcepEndpointClient
}

// NewVpcepEndpointService - 构造函数
func NewVpcepEndpointService(client VpcepEndpointClient) *VpcepEndpointService {
	return &VpcepEndpointService{client: client}
}

// VpcEndpointInput - 创建 VPCEP Endpoint 的输入参数
type VpcEndpointInput struct {
	EndpointServiceId string
	VpcId             string
	SubnetId          string
}

// VpcepEndpointOutput - Get/WaitForReady 的输出结构体
type VpcepEndpointOutput struct {
	EndpointId string
	Status     string
	Ip         string
	VpcId      string
	SubnetId   string
	ServiceId  string
}

// Create - 创建 VPCEP Endpoint 并等待就绪，返回 endpoint ID 和 IP
func (s *VpcepEndpointService) Create(ctx context.Context, input VpcEndpointInput) (string, string, error) {
	createReq := &model.CreateEndpointRequest{
		Body: &model.CreateEndpointRequestBody{
			EndpointServiceId: input.EndpointServiceId,
			VpcId:             input.VpcId,
			SubnetId:          &input.SubnetId,
		},
	}

	tflog.Debug(ctx, "Creating vpcep-endpoint", map[string]any{
		"endpoint_service_id": input.EndpointServiceId,
		"vpc_id":              input.VpcId,
		"subnet_id":           input.SubnetId,
	})

	var createResp *model.CreateEndpointResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		createResp, innerErr = s.client.CreateEndpoint(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateEndpoint API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", "", fmt.Errorf("createEndpoint API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", "", fmt.Errorf("createEndpoint response has no ID")
	}

	endpointId := *createResp.Id
	tflog.Info(ctx, "Vpcep-endpoint created", map[string]any{
		"endpoint_id": endpointId,
		"status":      *createResp.Status,
	})

	endpointIp, err := s.waitForReady(ctx, endpointId)
	if err != nil {
		return "", "", fmt.Errorf("wait for vpcep-endpoint ready failed: %w", err)
	}

	return endpointId, endpointIp, nil
}

// waitForReady 轮询等待 VPCEP Endpoint 状态变为 accepted，返回 endpoint IP
func (s *VpcepEndpointService) waitForReady(ctx context.Context, endpointId string) (string, error) {
	timeout := time.After(pollingTimeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for vpcep-endpoint %s", endpointId)
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for vpcep-endpoint %s to be ready", endpointId)
		case <-ticker.C:
			getResp, err := s.client.ListEndpointInfoDetails(&model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: endpointId,
			})
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return "", fmt.Errorf("query vpcep-endpoint status failed: %w", err)
				}
				tflog.Warn(ctx, "query vpcep-endpoint failed, will retry", map[string]any{
					"endpoint_id": endpointId,
					"error":       err.Error(),
					"err_count":   errCount,
				})
				continue
			}
			errCount = 0

			if getResp.Status == nil {
				return "", fmt.Errorf("vpcep-endpoint response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "Vpcep-endpoint status check", map[string]any{
				"endpoint_id": endpointId,
				"status":      status,
			})

			endpointIp, isReady, err := handleEndpointStatus(ctx, endpointId, status, getResp.Ip)
			if err != nil || isReady {
				return endpointIp, err
			}
		}
	}
}

// Delete - 删除 VPCEP Endpoint
func (s *VpcepEndpointService) Delete(ctx context.Context, endpointId string) error {
	deleteReq := &model.DeleteEndpointRequest{
		VpcEndpointId: endpointId,
	}

	tflog.Debug(ctx, "Deleting vpcep-endpoint", map[string]any{
		"endpoint_id": endpointId,
	})

	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.DeleteEndpoint(deleteReq)
		if innerErr != nil {
			if isVpcepNotFoundError(innerErr) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpoint API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "Vpcep-endpoint deleted", map[string]any{
		"endpoint_id": endpointId,
	})
	return nil
}

// Get - 查询 VPCEP Endpoint 详情，不存在时返回 nil, nil
func (s *VpcepEndpointService) Get(ctx context.Context, endpointId string) (*VpcepEndpointOutput, error) {
	endpointNotFound := false
	var getResp *model.ListEndpointInfoDetailsResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		getResp, innerErr = s.client.ListEndpointInfoDetails(&model.ListEndpointInfoDetailsRequest{
			VpcEndpointId: endpointId,
		})
		if isVpcepNotFoundError(innerErr) {
			endpointNotFound = true
			return nil
		}
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	if endpointNotFound {
		return nil, nil
	}

	output := &VpcepEndpointOutput{EndpointId: endpointId}
	if getResp.Status != nil {
		output.Status = *getResp.Status
	}
	if getResp.Ip != nil {
		output.Ip = *getResp.Ip
	}
	if getResp.VpcId != nil {
		output.VpcId = *getResp.VpcId
	}
	if getResp.SubnetId != nil {
		output.SubnetId = *getResp.SubnetId
	}
	if getResp.EndpointServiceId != nil {
		output.ServiceId = *getResp.EndpointServiceId
	}
	return output, nil
}

// handleEndpointStatus - 处理 endpoint 状态响应
func handleEndpointStatus(ctx context.Context, endpointId, status string,
	endpointIp *string) (readyEndpointIp string, isReady bool, err error) {
	switch status {
	case "accepted":
		if endpointIp == nil {
			return "", true, fmt.Errorf("vpcep-endpoint is accepted but has no IP")
		}
		tflog.Info(ctx, "Vpcep-endpoint is ready", map[string]any{
			"endpoint_id": endpointId,
			"ip":          *endpointIp,
		})
		return *endpointIp, true, nil
	case "failed", "rejected":
		return "", true, fmt.Errorf("vpcep-endpoint %s status is %s", endpointId, status)
	case "deleting":
		return "", true, fmt.Errorf("vpcep-endpoint %s is being deleted", endpointId)
	case "creating", "pendingAcceptance":
		return "", false, nil
	default:
		tflog.Warn(ctx, "Vpcep-endpoint unknown status", map[string]any{
			"endpoint_id": endpointId,
			"status":      status,
		})
		return "", false, nil
	}
}
