/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"

	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

const resourceTypeName = "_net_connect_m1_to_m3"

const (
	vpcepClientPollingInterval = 5 * time.Second
	vpcepClientPollingTimeout  = 5 * time.Minute
)

type netConnectM1ToM3Resource struct {
	m1PlusVpcepClient *vpcep.VpcepClient
	m3VpcepClient     *vpcep.VpcepClient
}

type netConnectM1ToM3Model struct {
	M3VpcId             string                  `tfsdk:"m3_vpc_id"`
	M3ServerType        string                  `tfsdk:"m3_server_type"`
	M3PortId            string                  `tfsdk:"m3_port_id"`
	M3VpcepServicePorts []vpcepServicePortBlock `tfsdk:"m3_vpcep_service_ports"`
	M3VpcepServiceName  types.String            `tfsdk:"m3_vpcep_service_name"`
	M1PlusVpcId         string                  `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId      string                  `tfsdk:"m1_plus_subnet_id"`
	M1PlusDomainId      string                  `tfsdk:"m1_plus_domain_id"`
	DnsDomain           string                  `tfsdk:"dns_domain"`
	DnsDomainSuffix     string                  `tfsdk:"dns_domain_suffix"`
	VpcepServiceId      types.String            `tfsdk:"vpcep_service_id"`
	VpcepClientId       types.String            `tfsdk:"vpcep_client_id"`
	VpcepClientIp       types.String            `tfsdk:"vpcep_client_ip"`
}

type vpcepServicePortBlock struct {
	ClientPort int32 `tfsdk:"client_port"`
	ServerPort int32 `tfsdk:"server_port"`
}

func NewNetConnectM1ToM3Resource() resource.Resource {
	return &netConnectM1ToM3Resource{}
}

func (r *netConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + resourceTypeName
}

func (r *netConnectM1ToM3Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m3_vpc_id":      schema.StringAttribute{Required: true},
			"m3_server_type": schema.StringAttribute{Required: true},
			"m3_port_id":     schema.StringAttribute{Required: true},
			"m3_vpcep_service_ports": schema.ListNestedAttribute{Required: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"client_port": schema.Int32Attribute{Required: true},
					"server_port": schema.Int32Attribute{Required: true},
				}}},
			"m3_vpcep_service_name": schema.StringAttribute{Optional: true},
			"m1_plus_vpc_id":        schema.StringAttribute{Required: true},
			"m1_plus_subnet_id":     schema.StringAttribute{Required: true},
			"m1_plus_domain_id":     schema.StringAttribute{Required: true},
			"dns_domain":            schema.StringAttribute{Required: true},
			"dns_domain_suffix":     schema.StringAttribute{Required: true},
			"vpcep_service_id":      schema.StringAttribute{Computed: true},
			"vpcep_client_id":       schema.StringAttribute{Computed: true},
			"vpcep_client_ip":       schema.StringAttribute{Computed: true},
		},
	}
}

func (r *netConnectM1ToM3Resource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*clients)
	if !ok {
		resp.Diagnostics.AddError("configure error", "invalid provider data type")
		return
	}
	r.m1PlusVpcepClient = clients.m1PlusVpcepClient
	r.m3VpcepClient = clients.m3VpcepClient
}

func (r *netConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Create started", map[string]any{
		"m3_vpc_id":      plan.M3VpcId,
		"m3_port_id":     plan.M3PortId,
		"m1_plus_vpc_id": plan.M1PlusVpcId,
	})

	// Step 1 - 在 M3 侧创建 VPCEP-Service
	vpcepServiceId, err := r.createM3VpcepService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("create VPCEP service failed", err.Error())
		return
	}
	plan.VpcepServiceId = types.StringValue(vpcepServiceId)

	// Step 2 - 配置 VPCEP-Service 白名单
	if err := r.addVpcepServicePermission(ctx, vpcepServiceId, plan.M1PlusDomainId); err != nil {
		// 回滚：删除已创建的 VPCEP-Service
		if rollbackErr := r.deleteM3VpcepService(ctx, vpcepServiceId); rollbackErr != nil {
			tflog.Warn(ctx, "rollback: failed to delete VPCEP-Service", map[string]any{
				"service_id": vpcepServiceId,
				"error":      rollbackErr.Error(),
			})
		}
		resp.Diagnostics.AddError("add VPCEP service permission failed", err.Error())
		return
	}

	// Step 3 - 在 M1+ 侧创建 VPCEP-Client
	vpcepClientId, err := r.createM1PlusVpcepClient(ctx, &plan, vpcepServiceId)
	if err != nil {
		// 回滚：删除已创建的 VPCEP-Service
		if rollbackErr := r.deleteM3VpcepService(ctx, vpcepServiceId); rollbackErr != nil {
			tflog.Warn(ctx, "rollback: failed to delete VPCEP-Service", map[string]any{
				"service_id": vpcepServiceId,
				"error":      rollbackErr.Error(),
			})
		}
		resp.Diagnostics.AddError("create VPCEP client failed", err.Error())
		return
	}
	plan.VpcepClientId = types.StringValue(vpcepClientId)

	// Step 4 - 轮询等待 Client 状态就绪
	clientIp, err := r.waitForVpcepClientReady(ctx, vpcepClientId)
	if err != nil {
		// 回滚：删除 VPCEP-Client 和 Service
		r.deleteM1PlusVpcepClient(ctx, vpcepClientId)
		if rollbackErr := r.deleteM3VpcepService(ctx, vpcepServiceId); rollbackErr != nil {
			tflog.Warn(ctx, "rollback: failed to delete VPCEP-Service", map[string]any{
				"service_id": vpcepServiceId,
				"error":      rollbackErr.Error(),
			})
		}
		resp.Diagnostics.AddError("wait for VPCEP client ready failed", err.Error())
		return
	}
	plan.VpcepClientIp = types.StringValue(clientIp)

	// Step 5 - 调用内网 DNS API 创建解析记录（TODO）
	tflog.Info(ctx, "Step 5 - DNS record creation not implemented yet", map[string]any{
		"dns_domain":        plan.DnsDomain,
		"dns_domain_suffix": plan.DnsDomainSuffix,
		"client_ip":         clientIp,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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

func (r *netConnectM1ToM3Resource) createM3VpcepService(ctx context.Context, plan *netConnectM1ToM3Model) (string,
	error) {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	ports := make([]model.PortList, len(plan.M3VpcepServicePorts))
	for i := range plan.M3VpcepServicePorts {
		ports[i] = model.PortList{
			ClientPort: &plan.M3VpcepServicePorts[i].ClientPort,
			ServerPort: &plan.M3VpcepServicePorts[i].ServerPort,
			Protocol:   &tcpProtocol,
		}
	}

	// 当前固定创建单栈 IPv4、不启用审批的 VPCEP-Service
	ipVersion := model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4
	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:           plan.M3VpcId,
			PortId:          plan.M3PortId,
			ServerType:      getVpcepServerType(plan.M3ServerType),
			ApprovalEnabled: utils.BoolPtr(false),
			Ports:           ports,
			IpVersion:       &ipVersion,
		},
	}
	if !plan.M3VpcepServiceName.IsNull() {
		serviceName := plan.M3VpcepServiceName.ValueString()
		createReq.Body.ServiceName = &serviceName
	}

	requestJson, err := json.Marshal(createReq.Body)
	if err != nil {
		tflog.Warn(ctx, "Failed to marshal VPCEP-Service request", map[string]any{
			"error": err.Error(),
		})
	}
	tflog.Debug(ctx, "Creating VPCEP-Service", map[string]any{
		"request": string(requestJson),
	})

	createResp, err := r.m3VpcepClient.CreateEndpointService(createReq)
	if err != nil {
		return "", fmt.Errorf("CreateEndpointService API failed: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("CreateEndpointService response has no ID")
	}

	tflog.Info(ctx, "VPCEP-Service created", map[string]any{
		"service_id": *createResp.Id,
		"status":     *createResp.Status,
	})

	return *createResp.Id, nil
}

// createM1PlusVpcepClient 在 M1+ 侧创建 VPCEP-Client。
func (r *netConnectM1ToM3Resource) createM1PlusVpcepClient(ctx context.Context, plan *netConnectM1ToM3Model,
	serviceId string) (string, error) {
	createReq := &model.CreateEndpointRequest{
		Body: &model.CreateEndpointRequestBody{
			EndpointServiceId: serviceId,
			VpcId:             plan.M1PlusVpcId,
			SubnetId:          &plan.M1PlusSubnetId,
		},
	}

	tflog.Debug(ctx, "Creating VPCEP-Client", map[string]any{
		"endpoint_service_id": serviceId,
		"vpc_id":              plan.M1PlusVpcId,
		"subnet_id":           plan.M1PlusSubnetId,
	})

	createResp, err := r.m1PlusVpcepClient.CreateEndpoint(createReq)
	if err != nil {
		return "", fmt.Errorf("CreateEndpoint API failed: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("CreateEndpoint response has no ID")
	}

	tflog.Info(ctx, "VPCEP-Client created", map[string]any{
		"client_id": *createResp.Id,
		"status":    *createResp.Status,
	})

	return *createResp.Id, nil
}

// waitForVpcepClientReady 轮询等待 VPCEP-Client 状态变为 accepted 后返回 Client IP。
func (r *netConnectM1ToM3Resource) waitForVpcepClientReady(ctx context.Context,
	clientId string) (string, error) {
	timeout := time.After(vpcepClientPollingTimeout)
	ticker := time.NewTicker(vpcepClientPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for VPCEP-Client %s to be ready", clientId)
		case <-ticker.C:
			getReq := &model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: clientId,
			}

			getResp, err := r.m1PlusVpcepClient.ListEndpointInfoDetails(getReq)
			if err != nil {
				return "", fmt.Errorf("query VPCEP-Client status failed: %w", err)
			}

			if getResp.Status == nil {
				return "", fmt.Errorf("VPCEP-Client response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "VPCEP-Client status check", map[string]any{
				"client_id": clientId,
				"status":    status,
			})

			switch status {
			case "accepted":
				if getResp.Ip == nil {
					return "", fmt.Errorf("VPCEP-Client is accepted but has no IP")
				}
				tflog.Info(ctx, "VPCEP-Client is ready", map[string]any{
					"client_id": clientId,
					"ip":        *getResp.Ip,
				})
				return *getResp.Ip, nil
			case "failed", "rejected":
				return "", fmt.Errorf("VPCEP-Client %s status is %s", clientId, status)
			case "creating", "pendingAcceptance":
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "VPCEP-Client unknown status", map[string]any{
					"client_id": clientId,
					"status":    status,
				})
			}
		}
	}
}

func (r *netConnectM1ToM3Resource) deleteM1PlusVpcepClient(ctx context.Context, clientId string) {
	deleteReq := &model.DeleteEndpointRequest{
		VpcEndpointId: clientId,
	}

	tflog.Debug(ctx, "Deleting VPCEP-Client", map[string]any{
		"client_id": clientId,
	})

	_, err := r.m1PlusVpcepClient.DeleteEndpoint(deleteReq)
	if err != nil {
		if !utils.IsNotFoundError(err) {
			tflog.Warn(ctx, "Failed to delete VPCEP-Client", map[string]any{
				"client_id": clientId,
				"error":     err.Error(),
			})
			return
		}
		tflog.Info(ctx, "VPCEP-Client already deleted or not found", map[string]any{
			"client_id": clientId,
		})
	} else {
		tflog.Info(ctx, "VPCEP-Client deleted", map[string]any{
			"client_id": clientId,
		})
	}
}

func (r *netConnectM1ToM3Resource) deleteM3VpcepService(ctx context.Context, serviceId string) error {
	deleteReq := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
	}

	tflog.Debug(ctx, "Deleting VPCEP-Service", map[string]any{
		"service_id": serviceId,
	})

	_, err := r.m3VpcepClient.DeleteEndpointService(deleteReq)
	if err != nil {
		if !utils.IsNotFoundError(err) {
			tflog.Warn(ctx, "Failed to delete VPCEP-Service", map[string]any{
				"service_id": serviceId,
				"error":      err.Error(),
			})
			return err
		}
		tflog.Info(ctx, "VPCEP-Service already deleted or not found", map[string]any{
			"service_id": serviceId,
		})
	} else {
		tflog.Info(ctx, "VPCEP-Service deleted", map[string]any{
			"service_id": serviceId,
		})
	}
	return nil
}

// addVpcepServicePermission 为 VPCEP-Service 添加白名单权限。
func (r *netConnectM1ToM3Resource) addVpcepServicePermission(ctx context.Context, serviceId, domainId string) error {
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

	tflog.Debug(ctx, "Adding VPCEP-Service permission", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})

	_, err := r.m3VpcepClient.BatchAddEndpointServicePermissions(req)
	if err != nil {
		return fmt.Errorf("BatchAddEndpointServicePermissions API failed: %w", err)
	}

	tflog.Info(ctx, "VPCEP-Service permission added", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})
	return nil
}

// Read 逻辑：
// - VpcepServiceId 有值 → 查询 API 验证存在性，404 → null
// - VpcepServiceId 为 null → 直接信任 state
// - 全部子资源均为 null → RemoveResource
func (r *netConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Read started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 验证 VPCEP-Service 是否仍存在
	if !state.VpcepServiceId.IsNull() {
		getReq := &model.ListServiceDetailsRequest{VpcEndpointServiceId: state.VpcepServiceId.ValueString()}
		_, err := r.m3VpcepClient.ListServiceDetails(getReq)
		if err != nil {
			if utils.IsNotFoundError(err) {
				tflog.Info(ctx, "VPCEP-Service not found, marking as null", map[string]any{
					"service_id": state.VpcepServiceId.ValueString(),
				})
				state.VpcepServiceId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("query VPCEP service failed", err.Error())
				return
			}
		}
	}

	// TODO: 验证 VPCEP-Client 是否仍存在（当VpcepClientId有值时）

	// 全部子资源均不存在时，移除整个 resource
	allRemoved := state.VpcepServiceId.IsNull() && state.VpcepClientId.IsNull() && state.VpcepClientIp.IsNull()
	if allRemoved {
		tflog.Info(ctx, "All sub-resources not found, removing resource from state")
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *netConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Update called")
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Delete started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 删除顺序：DNS → VPCEP-Client → VPCEP-Service
	// Step 1 - 删除内网 DNS 解析记录（TODO）
	// Step 2 - 删除 M1+ 侧 VPCEP-Client（TODO）
	// Step 3 - 删除 M3 侧 VPCEP-Service
	if !state.VpcepServiceId.IsNull() {
		if err := r.deleteM3VpcepService(ctx, state.VpcepServiceId.ValueString()); err != nil {
			resp.Diagnostics.AddError("delete VPCEP service failed", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}
