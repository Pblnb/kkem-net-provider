/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
)

type netConnectM1ToM3Resource struct {
	m3VpcepClient *vpcep.VpcepClient
}

type netConnectM1ToM3Model struct {
	M3VpcId             string                  `tfsdk:"m3_vpc_id"`
	M3ServerType        string                  `tfsdk:"m3_server_type"`
	M3PortId            string                  `tfsdk:"m3_port_id"`
	M3VpcepServicePorts []vpcepServicePortBlock `tfsdk:"m3_vpcep_service_ports"`
	M1PlusVpcId         string                  `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId      string                  `tfsdk:"m1_plus_subnet_id"`
	DnsDomain           string                  `tfsdk:"dns_domain"`
	DnsDomainSuffix     string                  `tfsdk:"dns_domain_suffix"`
	VpcepServiceId      *string                 `tfsdk:"vpcep_service_id"`
	VpcepClientId       *string                 `tfsdk:"vpcep_client_id"`
	VpcepClientIp       *string                 `tfsdk:"vpcep_client_ip"`
}

type vpcepServicePortBlock struct {
	ClientPort int32  `tfsdk:"client_port"`
	ServerPort int32  `tfsdk:"server_port"`
	Protocol   string `tfsdk:"protocol"`
}

func NewNetConnectM1ToM3Resource() resource.Resource {
	return &netConnectM1ToM3Resource{}
}

func (r *netConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m1_to_m3"
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
					"client_port": schema.Int64Attribute{Required: true},
					"server_port": schema.Int64Attribute{Required: true},
					"protocol":    schema.StringAttribute{Required: true},
				}}},
			"m1_plus_vpc_id":    schema.StringAttribute{Required: true},
			"m1_plus_subnet_id": schema.StringAttribute{Required: true},
			"dns_domain":        schema.StringAttribute{Required: true},
			"dns_domain_suffix": schema.StringAttribute{Required: true},
			"vpcep_service_id":  schema.StringAttribute{Computed: true},
			"vpcep_client_id":   schema.StringAttribute{Computed: true},
			"vpcep_client_ip":   schema.StringAttribute{Computed: true},
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
	r.m3VpcepClient = clients.m3VpcepClient
}

func (r *netConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Create started")
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1 - 在 M3 侧创建 VPCEP-Service
	vpcepServiceId, err := r.createVpcepService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("create VPCEP service failed", err.Error())
		return
	}
	plan.VpcepServiceId = &vpcepServiceId

	// Step 2 - 配置 VPCEP-Service 白名单（当前跳过，approval_enabled=false）
	// Step 3 - 在 M1+ 侧创建 VPCEP-Client（TODO）
	// Step 4 - 轮询等待 Client 状态就绪（TODO）
	// Step 5 - 调用内网 DNS API 创建解析记录（TODO）

	emptyStr := ""
	plan.VpcepClientId = &emptyStr
	plan.VpcepClientIp = &emptyStr
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

// getPortProtocol 将协议字符串转换为 API 调用所需类型。注意：目前仅支持 TCP，默认也返回 TCP。
func getPortProtocol(protocol string) *model.PortListProtocol {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	switch protocol {
	case "TCP":
		return &tcpProtocol
	default:
		return &tcpProtocol
	}
}

func (r *netConnectM1ToM3Resource) createVpcepService(ctx context.Context, plan *netConnectM1ToM3Model) (string,
	error) {
	ports := make([]model.PortList, len(plan.M3VpcepServicePorts))
	for i := range plan.M3VpcepServicePorts {
		ports[i] = model.PortList{
			ClientPort: &plan.M3VpcepServicePorts[i].ClientPort,
			ServerPort: &plan.M3VpcepServicePorts[i].ServerPort,
			Protocol:   getPortProtocol(plan.M3VpcepServicePorts[i].Protocol),
		}
	}

	approvalEnabled := false
	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:           plan.M3VpcId,
			PortId:          plan.M3PortId,
			ServerType:      getVpcepServerType(plan.M3ServerType),
			ApprovalEnabled: &approvalEnabled,
			Ports:           ports,
		},
	}

	tflog.Debug(ctx, "Creating VPCEP-Service", map[string]interface{}{
		"vpc_id":      plan.M3VpcId,
		"port_id":     plan.M3PortId,
		"server_type": plan.M3ServerType,
		"ports_count": len(ports),
	})

	createResp, err := r.m3VpcepClient.CreateEndpointService(createReq)
	if err != nil {
		return "", fmt.Errorf("CreateEndpointService API failed: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("CreateEndpointService response has no ID")
	}

	tflog.Info(ctx, "VPCEP-Service created", map[string]interface{}{
		"service_id": *createResp.Id,
		"status":     *createResp.Status,
	})

	return *createResp.Id, nil
}

func (r *netConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Read started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.VpcepServiceId == nil {
		return
	}

	// 查询 VPCEP-Service 状态
	getReq := &model.ListServiceDetailsRequest{VpcEndpointServiceId: *state.VpcepServiceId}

	getResp, err := r.m3VpcepClient.ListServiceDetails(getReq)
	if err != nil {
		resp.Diagnostics.AddError("query VPCEP service failed", err.Error())
		return
	}

	tflog.Debug(ctx, "VPCEP-Service status", map[string]interface{}{
		"service_id": *state.VpcepServiceId,
		"status":     *getResp.Status,
	})

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

	if state.VpcepServiceId == nil {
		return
	}

	// 删除顺序：DNS → VPCEP-Client → VPCEP-Service
	// Step 1 - 删除内网 DNS 解析记录（TODO）

	// Step 2 - 删除 M1+ 侧 VPCEP-Client（TODO）

	// Step 3 - 删除 M3 侧 VPCEP-Service
	deleteReq := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: *state.VpcepServiceId,
	}

	tflog.Debug(ctx, "Deleting VPCEP-Service", map[string]interface{}{
		"service_id": *state.VpcepServiceId,
	})

	_, err := r.m3VpcepClient.DeleteEndpointService(deleteReq)
	if err != nil {
		resp.Diagnostics.AddError("delete VPCEP service failed", err.Error())
		return
	}

	tflog.Info(ctx, "VPCEP-Service deleted", map[string]interface{}{
		"service_id": *state.VpcepServiceId,
	})
}
