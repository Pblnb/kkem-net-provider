/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
)

// VpcepServiceClient - VPCEP Service 专用接口，仅暴露 Service 相关的 SDK 方法。
type VpcepServiceClient interface {
	CreateEndpointService(req *model.CreateEndpointServiceRequest) (*model.CreateEndpointServiceResponse, error)
	DeleteEndpointService(req *model.DeleteEndpointServiceRequest) (*model.DeleteEndpointServiceResponse, error)
	UpdateEndpointService(req *model.UpdateEndpointServiceRequest) (*model.UpdateEndpointServiceResponse, error)
	ListServiceDetails(req *model.ListServiceDetailsRequest) (*model.ListServiceDetailsResponse, error)
	BatchAddEndpointServicePermissions(req *model.BatchAddEndpointServicePermissionsRequest) (*model.BatchAddEndpointServicePermissionsResponse,
		error)
	BatchRemoveEndpointServicePermissions(req *model.BatchRemoveEndpointServicePermissionsRequest) (*model.BatchRemoveEndpointServicePermissionsResponse,
		error)
	ListServicePermissionsDetails(req *model.ListServicePermissionsDetailsRequest) (*model.ListServicePermissionsDetailsResponse,
		error)
}

// VpcepService - VPCEP Service service 层
type VpcepService struct {
	client VpcepServiceClient
}

// NewVpcepService - 构造函数
func NewVpcepService(client VpcepServiceClient) *VpcepService {
	return &VpcepService{client: client}
}

// PortPair - VPCEP Service 端口对
type PortPair struct {
	ClientPort int32
	ServerPort int32
}

// VpcepServiceInput - 创建/更新 VPCEP Service 的输入参数
type VpcepServiceInput struct {
	VpcId      string
	PortId     string
	ServerType string
	Ports      []PortPair
}

// VpcepServiceOutput - Get 的输出结构体
type VpcepServiceOutput struct {
	ServiceId  string
	Status     string
	ServerType string
	VpcId      string
	PortId     string
	Ports      []PortPair
}

// PermissionInput - 权限输入
type PermissionInput struct {
	Permission string
}

// Create - 创建 VPCEP Service 并等待就绪
func (s *VpcepService) Create(ctx context.Context, input VpcepServiceInput) (string, error) {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	ports := make([]model.PortList, len(input.Ports))
	for i := range input.Ports {
		ports[i] = model.PortList{
			ClientPort: &input.Ports[i].ClientPort,
			ServerPort: &input.Ports[i].ServerPort,
			Protocol:   &tcpProtocol,
		}
	}

	ipVersion := model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4
	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:           input.VpcId,
			PortId:          input.PortId,
			ServerType:      getServerType(input.ServerType),
			ApprovalEnabled: boolPtr(false),
			Ports:           ports,
			IpVersion:       &ipVersion,
		},
	}

	requestJson, err := json.Marshal(createReq.Body)
	if err != nil {
		tflog.Warn(ctx, "Failed to marshal vpcep-service request", map[string]any{
			"error": err.Error(),
		})
	}
	tflog.Debug(ctx, "Creating vpcep-service", map[string]any{
		"request": string(requestJson),
	})

	var createResp *model.CreateEndpointServiceResponse
	err = retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		createResp, innerErr = s.client.CreateEndpointService(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("createEndpointService API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createEndpointService response has no ID")
	}

	tflog.Info(ctx, "Vpcep-service created", map[string]any{
		"service_id": *createResp.Id,
		"status":     *createResp.Status,
	})

	if err := s.waitForReady(ctx, *createResp.Id); err != nil {
		return "", fmt.Errorf("wait for vpcep-service ready failed: %w", err)
	}

	return *createResp.Id, nil
}

// waitForReady 轮询等待 VPCEP Service 状态变为 available
func (s *VpcepService) waitForReady(ctx context.Context, serviceId string) error {
	timeout := time.After(pollingTimeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for vpcep-service %s: %s", serviceId, ctx.Err())
		case <-timeout:
			return fmt.Errorf("timeout waiting for vpcep-service %s to be ready", serviceId)
		case <-ticker.C:
			getResp, err := s.client.ListServiceDetails(&model.ListServiceDetailsRequest{
				VpcEndpointServiceId: serviceId,
			})
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return fmt.Errorf("query vpcep-service status failed: %w", err)
				}
				tflog.Warn(ctx, "query vpcep-service failed, will retry", map[string]any{
					"service_id": serviceId,
					"error":      err.Error(),
					"err_count":  errCount,
				})
				continue
			}
			errCount = 0

			if getResp.Status == nil {
				return fmt.Errorf("vpcep-service response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "Vpcep-service status check", map[string]any{
				"service_id": serviceId,
				"status":     status,
			})

			switch status {
			case "available":
				tflog.Info(ctx, "Vpcep-service is ready", map[string]any{
					"service_id": serviceId,
				})
				return nil
			case "failed":
				return fmt.Errorf("vpcep-service %s status is failed", serviceId)
			case "creating":
				continue
			default:
				tflog.Warn(ctx, "Vpcep-service unknown status", map[string]any{
					"service_id": serviceId,
					"status":     status,
				})
			}
		}
	}
}

// Delete - 删除 VPCEP Service
func (s *VpcepService) Delete(ctx context.Context, serviceId string) error {
	deleteReq := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
	}

	tflog.Debug(ctx, "Deleting vpcep-service", map[string]any{
		"service_id": serviceId,
	})

	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.DeleteEndpointService(deleteReq)
		if innerErr != nil {
			if isVpcepNotFoundError(innerErr) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "Vpcep-service deleted", map[string]any{
		"service_id": serviceId,
	})
	return nil
}

// AddPermissions - 添加白名单权限
func (s *VpcepService) AddPermissions(ctx context.Context, serviceId string, permissions []PermissionInput) error {
	addPermissions := make([]model.EpsAddPermissionRequest, len(permissions))
	for i := range permissions {
		addPermissions[i] = model.EpsAddPermissionRequest{
			Permission:  permissions[i].Permission,
			Description: "Allow access from configured permission",
		}
	}
	req := &model.BatchAddEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchAddEndpointServicePermissionsRequestBody{
			Permissions: addPermissions,
		},
	}

	tflog.Debug(ctx, "Adding vpcep-service permission", map[string]any{
		"service_id":  serviceId,
		"permissions": len(permissions),
	})

	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.BatchAddEndpointServicePermissions(req)
		if innerErr != nil {
			tflog.Warn(ctx, "BatchAddEndpointServicePermissions API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("batchAddEndpointServicePermissions API failed after retries: %w", err)
	}

	tflog.Info(ctx, "Vpcep-service permission added", map[string]any{
		"service_id":  serviceId,
		"permissions": len(permissions),
	})
	return nil
}

// ReconcilePermissions - 对比并同步权限列表
func (s *VpcepService) ReconcilePermissions(ctx context.Context, serviceId string, desired []PermissionInput) error {
	remote, err := s.GetPermissions(ctx, serviceId)
	if err != nil {
		return err
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, permission := range desired {
		desiredSet[permission.Permission] = struct{}{}
	}

	var addPermissions []PermissionInput
	for permission := range desiredSet {
		if _, ok := remote[permission]; !ok {
			addPermissions = append(addPermissions, PermissionInput{Permission: permission})
		}
	}
	if len(addPermissions) > 0 {
		if err := s.AddPermissions(ctx, serviceId, addPermissions); err != nil {
			return err
		}
	}

	var removePermissions []model.EpsRemovePermissionRequest
	for permission, id := range remote {
		if _, ok := desiredSet[permission]; ok {
			continue
		}
		if id == "" {
			return fmt.Errorf("vpcep-service permission %s has no id", permission)
		}
		removePermissions = append(removePermissions, model.EpsRemovePermissionRequest{Id: id})
	}
	if len(removePermissions) == 0 {
		return nil
	}

	removeReq := &model.BatchRemoveEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchRemoveEndpointServicePermissionsRequestBody{
			Permissions: removePermissions,
		},
	}
	err = retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.BatchRemoveEndpointServicePermissions(removeReq)
		if innerErr != nil {
			tflog.Warn(ctx, "BatchRemoveEndpointServicePermissions API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("batchRemoveEndpointServicePermissions API failed after retries: %w", err)
	}
	tflog.Info(ctx, "Vpcep-service permissions reconciled", map[string]any{
		"service_id": serviceId,
		"added":      len(addPermissions),
		"removed":    len(removePermissions),
	})
	return nil
}

// GetPermissions - 查询当前权限列表，返回 map[permission]id
func (s *VpcepService) GetPermissions(ctx context.Context, serviceId string) (map[string]string, error) {
	var getResp *model.ListServicePermissionsDetailsResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		getResp, innerErr = s.client.ListServicePermissionsDetails(&model.ListServicePermissionsDetailsRequest{
			VpcEndpointServiceId: serviceId,
		})
		return innerErr
	})
	if err != nil {
		return nil, fmt.Errorf("listServicePermissionsDetails API failed after retries: %w", err)
	}

	permissions := make(map[string]string)
	if getResp.Permissions == nil {
		return permissions, nil
	}
	for _, permission := range *getResp.Permissions {
		if permission.Permission == nil {
			continue
		}
		id := ""
		if permission.Id != nil {
			id = *permission.Id
		}
		permissions[*permission.Permission] = id
	}
	return permissions, nil
}

// UpdateConfig - 更新 VPCEP Service 配置，仅支持更新 port_id 与 ports
func (s *VpcepService) UpdateConfig(ctx context.Context, serviceId string, input VpcepServiceInput) error {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	ports := make([]model.PortList, len(input.Ports))
	for i := range input.Ports {
		ports[i] = model.PortList{
			ClientPort: &input.Ports[i].ClientPort,
			ServerPort: &input.Ports[i].ServerPort,
			Protocol:   &tcpProtocol,
		}
	}

	updateReq := &model.UpdateEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.UpdateEndpointServiceRequestBody{
			PortId: &input.PortId,
			Ports:  &ports,
		},
	}

	tflog.Debug(ctx, "Updating vpcep-service", map[string]any{
		"service_id": serviceId,
		"port_id":    input.PortId,
		"ports":      len(input.Ports),
	})

	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		_, innerErr := s.client.UpdateEndpointService(updateReq)
		if innerErr != nil {
			tflog.Warn(ctx, "UpdateEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("updateEndpointService API failed after retries: %w", err)
	}
	if err := s.waitForReady(ctx, serviceId); err != nil {
		return fmt.Errorf("wait for vpcep-service ready failed: %w", err)
	}
	return nil
}

// Get - 查询 VPCEP Service 详情，不存在时返回 nil, nil
func (s *VpcepService) Get(ctx context.Context, serviceId string) (*VpcepServiceOutput, error) {
	serviceNotFound := false
	var getResp *model.ListServiceDetailsResponse
	err := retryWithBackoff(ctx, maxRetryCount, retryBaseDelay, func() error {
		var innerErr error
		getResp, innerErr = s.client.ListServiceDetails(&model.ListServiceDetailsRequest{
			VpcEndpointServiceId: serviceId,
		})
		if isVpcepNotFoundError(innerErr) {
			serviceNotFound = true
			return nil
		}
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	if serviceNotFound {
		return nil, nil
	}

	output := &VpcepServiceOutput{ServiceId: serviceId}
	if getResp.Status != nil {
		output.Status = *getResp.Status
	}
	if getResp.ServerType != nil {
		output.ServerType = *getResp.ServerType
	}
	if getResp.VpcId != nil {
		output.VpcId = *getResp.VpcId
	}
	if getResp.PortId != nil {
		output.PortId = *getResp.PortId
	}
	if getResp.Ports != nil {
		output.Ports = extractTcpPortPairs(*getResp.Ports)
	}
	return output, nil
}

// extractTcpPortPairs 从远端端口列表中提取 TCP 端口对。
func extractTcpPortPairs(ports []model.PortList) []PortPair {
	tcpPorts := make([]PortPair, 0, len(ports))
	for _, port := range ports {
		if port.ClientPort == nil || port.ServerPort == nil || port.Protocol == nil ||
			port.Protocol.Value() != "TCP" {
			continue
		}
		tcpPorts = append(tcpPorts, PortPair{
			ClientPort: *port.ClientPort,
			ServerPort: *port.ServerPort,
		})
	}
	return tcpPorts
}

// getServerType - 将字符串转换为 API 调用所需类型
func getServerType(serverType string) model.CreateEndpointServiceRequestBodyServerType {
	switch serverType {
	case "VM":
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().VM
	default:
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().LB
	}
}
