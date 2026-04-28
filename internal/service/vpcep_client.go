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

	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

type VpcepClient interface {
	CreateEndpoint(req *model.CreateEndpointRequest) (*model.CreateEndpointResponse, error)
	DeleteEndpoint(request *model.DeleteEndpointRequest) (*model.DeleteEndpointResponse, error)
	ListEndpointInfoDetails(req *model.ListEndpointInfoDetailsRequest) (*model.ListEndpointInfoDetailsResponse, error)

	CreateEndpointService(req *model.CreateEndpointServiceRequest) (*model.CreateEndpointServiceResponse, error)
	DeleteEndpointService(req *model.DeleteEndpointServiceRequest) (*model.DeleteEndpointServiceResponse, error)
	BatchAddEndpointServicePermissions(request *model.BatchAddEndpointServicePermissionsRequest) (*model.BatchAddEndpointServicePermissionsResponse, error)
	ListServiceDetails(request *model.ListServiceDetailsRequest) (*model.ListServiceDetailsResponse, error)
}

type VpcEndpointInput struct {
	EndpointServiceId string
	VpcId             string
	SubnetId          string
}

// CreateVpcEndpoint 创建 vpc-endpoint
func CreateVpcEndpoint(
	ctx context.Context,
	client VpcepClient,
	input VpcEndpointInput,
) (string, error) {
	createReq := &model.CreateEndpointRequest{
		Body: &model.CreateEndpointRequestBody{
			EndpointServiceId: input.EndpointServiceId,
			VpcId:             input.VpcId,
			SubnetId:          &input.SubnetId,
		},
	}

	tflog.Debug(ctx, "Creating vpc-endpoint", map[string]any{
		"endpoint_service_id": input.EndpointServiceId,
		"vpc_id":              input.VpcId,
		"subnet_id":           input.SubnetId,
	})

	var createResp *model.CreateEndpointResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var err error
		createResp, err = client.CreateEndpoint(createReq)
		if err != nil {
			tflog.Warn(ctx, "CreateEndpoint API failed, retrying", map[string]any{
				"error": err.Error(),
			})
		}
		return err
	})
	if err != nil {
		return "", fmt.Errorf("createEndpoint API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createEndpoint response has no ID")
	}

	tflog.Info(ctx, "vpc-endpoint created", map[string]any{
		"endpoint_id": *createResp.Id,
		"status":      *createResp.Status,
	})

	return *createResp.Id, nil
}

// DeleteVpcEndpoint 删除 vpc-endpoint
func DeleteVpcEndpoint(
	ctx context.Context,
	client VpcepClient,
	clientId string,
) error {
	deleteReq := &model.DeleteEndpointRequest{
		VpcEndpointId: clientId,
	}

	tflog.Debug(ctx, "Deleting vpc-endpoint", map[string]any{
		"client_id": clientId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, err := client.DeleteEndpoint(deleteReq)
		if err != nil {
			if utils.IsVpcepNotFoundError(err) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpoint API failed, retrying", map[string]any{
				"error": err.Error(),
			})
		}
		return err
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "vpc-endpoint deleted", map[string]any{
		"client_id": clientId,
	})
	return nil
}

// WaitForVpcEndpointReady 轮询等待 vpc-endpoint 状态变为 accepted 后返回 endpoint IP。
func WaitForVpcEndpointReady(
	ctx context.Context,
	client VpcepClient,
	clientId string,
) (string, error) {
	timeout := time.After(PollingTimeout)
	ticker := time.NewTicker(PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for vpc-endpoint %s", clientId)
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for vpc-endpoint %s to be ready", clientId)
		case <-ticker.C:
			getReq := &model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: clientId,
			}

			getResp, err := client.ListEndpointInfoDetails(getReq)
			if err != nil {
				return "", fmt.Errorf("query vpc-endpoint status failed: %w", err)
			}

			if getResp.Status == nil {
				return "", fmt.Errorf("vpc-endpoint response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "vpc-endpoint status check", map[string]any{
				"client_id": clientId,
				"status":    status,
			})

			switch status {
			case "accepted":
				if getResp.Ip == nil {
					return "", fmt.Errorf("vpc-endpoint is accepted but has no IP")
				}
				tflog.Info(ctx, "vpc-endpoint is ready", map[string]any{
					"client_id": clientId,
					"ip":        *getResp.Ip,
				})
				return *getResp.Ip, nil
			case "failed", "rejected":
				return "", fmt.Errorf("vpc-endpoint %s status is %s", clientId, status)
			case "deleting":
				return "", fmt.Errorf("vpc-endpoint %s is being deleted", clientId)
			case "creating", "pendingAcceptance":
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "vpc-endpoint unknown status", map[string]any{
					"client_id": clientId,
					"status":    status,
				})
			}
		}
	}
}

type PortPair struct {
	ClientPort int32
	ServerPort int32
}

type VpcEndpointServiceInput struct {
	VpcId       string
	PortId      string
	ServerType  string
	Ports       []PortPair
	ServiceName *string
}

func CreateEndpointService(
	ctx context.Context,
	client VpcepClient,
	input VpcEndpointServiceInput,
) (string, error) {
	tcpProtocol := model.GetPortListProtocolEnum().TCP

	ports := make([]model.PortList, len(input.Ports))
	for i := range input.Ports {
		cp := input.Ports[i].ClientPort
		sp := input.Ports[i].ServerPort

		ports[i] = model.PortList{
			ClientPort: &cp,
			ServerPort: &sp,
			Protocol:   &tcpProtocol,
		}
	}

	ipVersion := model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4

	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:           input.VpcId,
			PortId:          input.PortId,
			ServerType:      getVpcepServerType(input.ServerType),
			ApprovalEnabled: utils.BoolPtr(false),
			Ports:           ports,
			IpVersion:       &ipVersion,
		},
	}

	if input.ServiceName != nil {
		createReq.Body.ServiceName = input.ServiceName
	}

	reqJson, err := json.Marshal(createReq.Body)

	if err != nil {
		return "", fmt.Errorf("failed to init credential with ak/sk: %w", err)
	}

	tflog.Debug(ctx, "Creating vpcep-service", map[string]any{
		"request": string(reqJson),
	})

	var createResp *model.CreateEndpointServiceResponse
	err = utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var err error
		createResp, err = client.CreateEndpointService(createReq)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("createEndpointService API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createEndpointService response has no ID")
	}

	return *createResp.Id, nil
}

// DeleteVpcEndpointService 删除 VpcEndpointService
func DeleteVpcEndpointService(ctx context.Context,
	client VpcepClient,
	serviceId string,
) error {
	deleteReq := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
	}

	tflog.Debug(ctx, "Deleting vpcep-service", map[string]any{
		"service_id": serviceId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, err := client.DeleteEndpointService(deleteReq)
		if err != nil {
			if utils.IsVpcepNotFoundError(err) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpointService API failed, retrying", map[string]any{
				"error": err.Error(),
			})
		}
		return err
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "Vpcep-service deleted", map[string]any{
		"service_id": serviceId,
	})
	return nil
}

// AddVpcepServicePermission 为 vpcep-service 添加白名单权限。
func AddVpcepServicePermission(ctx context.Context,
	client VpcepClient,
	serviceId string,
	domainId string,
) error {
	permission := fmt.Sprintf("iam:domain::%s", domainId)
	req := &model.BatchAddEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchAddEndpointServicePermissionsRequestBody{
			Permissions: []model.EpsAddPermissionRequest{
				{
					Permission:  permission,
					Description: "Allow access from M1+ domain",
				},
			},
		},
	}

	tflog.Debug(ctx, "Adding vpcep-service permission", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, err := client.BatchAddEndpointServicePermissions(req)
		if err != nil {
			tflog.Warn(ctx, "BatchAddEndpointServicePermissions API failed, retrying", map[string]any{
				"error": err.Error(),
			})
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("batchAddEndpointServicePermissions API failed after retries: %w", err)
	}

	tflog.Info(ctx, "Vpcep-service permission added", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})
	return nil
}

// WaitForVpcepServiceReady 轮询等待 vpcep-service 状态变为 available。
func WaitForVpcepServiceReady(ctx context.Context,
	client VpcepClient,
	serviceId string,
) error {
	timeout := time.After(PollingTimeout)
	ticker := time.NewTicker(PollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for vpcep-service %s to be ready", serviceId)
		case <-ticker.C:
			getReq := &model.ListServiceDetailsRequest{
				VpcEndpointServiceId: serviceId,
			}

			getResp, err := client.ListServiceDetails(getReq)
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
			case VpcepStatusAvailable:
				tflog.Info(ctx, "Vpcep-service is ready", map[string]any{
					"service_id": serviceId,
				})
				return nil
			case VpcepStatusFailed:
				return fmt.Errorf("vpcep-service %s status is failed", serviceId)
			case VpcepStatusCreating:
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "Vpcep-service unknown status", map[string]any{
					"service_id": serviceId,
					"status":     status,
				})
			}
		}
	}
}

// getVpcepServerType 将字符串转换为 API 调用所需类型。注意：目前仅支持 VM 和 LB 两种类型，默认返回 LB。
func getVpcepServerType(serverType string) model.CreateEndpointServiceRequestBodyServerType {
	switch serverType {
	case "VM":
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().VM
	default:
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().LB
	}
}
