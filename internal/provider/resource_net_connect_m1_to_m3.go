/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	M3VpcepServiceName  types.String            `tfsdk:"m3_vpcep_service_name"`
	M1PlusVpcId         string                  `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId      string                  `tfsdk:"m1_plus_subnet_id"`
	DnsDomain           string                  `tfsdk:"dns_domain"`
	DnsDomainSuffix     string                  `tfsdk:"dns_domain_suffix"`
	VpcepServiceId      types.String            `tfsdk:"vpcep_service_id"`
	VpcepClientId       types.String            `tfsdk:"vpcep_client_id"`
	VpcepClientIp       types.String            `tfsdk:"vpcep_client_ip"`
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
	// cmt:_net_connect_m1_to_m3 此处抽取一个常量吧，命名能说明其含义
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
					"client_port": schema.Int32Attribute{Required: true},
					"server_port": schema.Int32Attribute{Required: true},
					"protocol":    schema.StringAttribute{Required: true},
				}}},
			"m3_vpcep_service_name": schema.StringAttribute{Optional: true},
			"m1_plus_vpc_id":        schema.StringAttribute{Required: true},
			"m1_plus_subnet_id":     schema.StringAttribute{Required: true},
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
	r.m3VpcepClient = clients.m3VpcepClient
}

func (r *netConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	// cmt:此处日志是否需要放入特征信息，比如能知道是谁的M1_M3网络打通开始create了
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Create started")
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1 - 在 M3 侧创建 VPCEP-Service
	vpcepServiceId, err := r.createM3VpcepService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("create VPCEP service failed", err.Error())
		return
	}
	plan.VpcepServiceId = types.StringValue(vpcepServiceId)

	// Step 2 - 配置 VPCEP-Service 白名单（当前跳过，approval_enabled=false）
	// Step 3 - 在 M1+ 侧创建 VPCEP-Client（TODO）
	// Step 4 - 轮询等待 Client 状态就绪（TODO）
	// Step 5 - 调用内网 DNS API 创建解析记录（TODO）

	plan.VpcepClientId = types.StringNull()
	plan.VpcepClientIp = types.StringNull()
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

// cmt:粗暴点，直接代码里写死 TCP 吧，也不需要用户在 schema 里配置了这个参数了
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

func (r *netConnectM1ToM3Resource) createM3VpcepService(ctx context.Context, plan *netConnectM1ToM3Model) (string,
	error) {
	ports := make([]model.PortList, len(plan.M3VpcepServicePorts))
	for i := range plan.M3VpcepServicePorts {
		ports[i] = model.PortList{
			ClientPort: &plan.M3VpcepServicePorts[i].ClientPort,
			ServerPort: &plan.M3VpcepServicePorts[i].ServerPort,
			Protocol:   getPortProtocol(plan.M3VpcepServicePorts[i].Protocol),
		}
	}

	// VPCEP-Service 不启用连接审批
	approvalEnabled := false
	// 当前固定创建单栈 IPv4 的 VPCEP-Service
	ipVersion := model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4
	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:      plan.M3VpcId,
			PortId:     plan.M3PortId,
			ServerType: getVpcepServerType(plan.M3ServerType),
			// cmt: 可以用 ptr 工具包里的函数代替 approvalEnabled := false 声明后在这里引用么？
			ApprovalEnabled: &approvalEnabled,
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
		tflog.Warn(ctx, "Failed to marshal VPCEP-Service request", map[string]interface{}{
			"error": err.Error(),
		})
	}
	tflog.Debug(ctx, "Creating VPCEP-Service", map[string]interface{}{
		"request": string(requestJson),
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
			errMsg := err.Error()
			if strings.Contains(errMsg, "404") {
				tflog.Info(ctx, "VPCEP-Service not found, marking as null", map[string]interface{}{
					"service_id": state.VpcepServiceId.ValueString(),
				})
				state.VpcepServiceId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("query VPCEP service failed", errMsg)
				return
			}
		}
	}

	// TODO: 验证 VPCEP-Client 是否仍存在（当VpcepClientId有值时）

	// 全部子资源均不存在时，移除整个 resource
	allRemoved := state.VpcepServiceId.IsNull() && state.VpcepClientId.IsNull() && state.VpcepClientIp.IsNull()
	if allRemoved {
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
	// Step 3 - 删除 M3 侧 VPCEP-Service（幂等删除：404视为已删除）
	if !state.VpcepServiceId.IsNull() {
		deleteReq := &model.DeleteEndpointServiceRequest{
			VpcEndpointServiceId: state.VpcepServiceId.ValueString(),
		}

		tflog.Debug(ctx, "Deleting VPCEP-Service", map[string]interface{}{
			"service_id": state.VpcepServiceId.ValueString(),
		})

		_, err := r.m3VpcepClient.DeleteEndpointService(deleteReq)
		if err != nil {
			errMsg := err.Error()
			if !(strings.Contains(errMsg, "404") || strings.Contains(errMsg, "NotFound")) {
				resp.Diagnostics.AddError("delete VPCEP service failed", errMsg)
				return
			}
			tflog.Info(ctx, "VPCEP-Service already deleted or not found", map[string]interface{}{
				"service_id": state.VpcepServiceId.ValueString(),
			})
		} else {
			tflog.Info(ctx, "VPCEP-Service deleted", map[string]interface{}{
				"service_id": state.VpcepServiceId.ValueString(),
			})
		}
	}

	resp.State.RemoveResource(ctx)
}
