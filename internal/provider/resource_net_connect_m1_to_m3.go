package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// NetConnectM1ToM3Resource M1→M3 网络打通 Resource。
type NetConnectM1ToM3Resource struct {
	M3VpcepClient interface{} // M3 侧 VPCEP 客户端（用于创建 VPCEP-Server）
	M1VpcepClient interface{} // M1+ 侧 VPCEP 客户端（用于创建 VPCEP-Client）
	DnsClient     interface{}
}

// NetConnectM1ToM3Model M1→M3 网络打通 Resource 的数据模型。
type NetConnectM1ToM3Model struct {
	// Required 字段
	M3VpcId            types.String     `tfsdk:"m3_vpc_id"`
	M3BackendType      types.String     `tfsdk:"m3_backend_type"`
	M3BackendId        types.String     `tfsdk:"m3_backend_id"`
	M3VpcepServerPorts []VpcepPortBlock `tfsdk:"m3_vpcep_server_ports"`
	M1VpcId            types.String     `tfsdk:"m1_vpc_id"`
	M1SubnetId         types.String     `tfsdk:"m1_subnet_id"`
	DnsApplicant       types.String     `tfsdk:"dns_applicant"`
	DnsDomain          types.String     `tfsdk:"dns_domain"`
	DnsDomainSuffix    types.String     `tfsdk:"dns_domain_suffix"`
	// Computed 字段
	VpcepServerId types.String `tfsdk:"vpcep_server_id"`
	VpcepClientId types.String `tfsdk:"vpcep_client_id"`
	VpcepClientIp types.String `tfsdk:"vpcep_client_ip"`
}

// VpcepPortBlock 端口映射嵌套块。
type VpcepPortBlock struct {
	ClientPort types.String `tfsdk:"client_port"`
	ServerPort types.String `tfsdk:"server_port"`
	Protocol   types.String `tfsdk:"protocol"`
}

// NewNetConnectM1ToM3Resource 创建 M1→M3 网络打通 Resource 实例。
func NewNetConnectM1ToM3Resource() resource.Resource {
	return &NetConnectM1ToM3Resource{}
}

// Metadata 返回 Resource 元信息。
func (r *NetConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m1_to_m3"
}

// Schema 定义 M1→M3 网络打通 Resource 的 Schema。
func (r *NetConnectM1ToM3Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required 字段
			"m3_vpc_id": schema.StringAttribute{
				Required: true,
			},
			"m3_backend_type": schema.StringAttribute{
				Required: true,
			},
			"m3_backend_id": schema.StringAttribute{
				Required: true,
			},
			"m3_vpcep_server_ports": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client_port": schema.StringAttribute{
							Required: true,
						},
						"server_port": schema.StringAttribute{
							Required: true,
						},
						"protocol": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"m1_vpc_id": schema.StringAttribute{
				Required: true,
			},
			"m1_subnet_id": schema.StringAttribute{
				Required: true,
			},
			"dns_applicant": schema.StringAttribute{
				Required: true,
			},
			"dns_domain": schema.StringAttribute{
				Required: true,
			},
			"dns_domain_suffix": schema.StringAttribute{
				Required: true,
			},
			// Computed 字段
			"vpcep_server_id": schema.StringAttribute{
				Computed: true,
			},
			"vpcep_client_id": schema.StringAttribute{
				Computed: true,
			},
			"vpcep_client_ip": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure 从 req.ProviderData 取出 KkemProviderData。
func (r *NetConnectM1ToM3Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*KkemProviderData)
	if !ok {
		resp.Diagnostics.AddError("Provider data 类型错误", "expected *KkemProviderData")
		return
	}

	r.M3VpcepClient = data.M3VpcepClient
	r.M1VpcepClient = data.M1VpcepClient
	r.DnsClient = data.M3DnsClient

	tflog.Debug(ctx, "NetConnectM1ToM3Resource Configure 完成", map[string]interface{}{
		"has_m3_client":  r.M3VpcepClient != nil,
		"has_m1_client":  r.M1VpcepClient != nil,
		"has_dns_client": r.DnsClient != nil,
	})
}

// Create 执行 M1→M3 网络打通的完整创建流程：
// Step 1 - 在 M3 侧创建 VPCEP-Server（TODO）
// Step 2 - 配置 VPCEP-Server 白名单（当前跳过，approval_enabled=false）（TODO）
// Step 3 - 在 M1+ 侧创建 VPCEP-Client（TODO）
// Step 4 - 轮询等待 Client 状态就绪（TODO）
// Step 5 - 调用内网 DNS API 创建解析记录（TODO）
func (r *NetConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "[M1→M3] Create 方法入口", map[string]interface{}{
		"method":   "Create",
		"resource": "kkem_net_connect_m1_to_m3",
	})

	// 解析配置
	var plan NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "[M1→M3] 解析配置失败", map[string]interface{}{
			"diagnostics": resp.Diagnostics.Errors(),
		})
		return
	}

	tflog.Debug(ctx, "[M1→M3] 创建请求参数", map[string]interface{}{
		"m3_vpc_id":             plan.M3VpcId.ValueString(),
		"m3_backend_type":       plan.M3BackendType.ValueString(),
		"m3_backend_id":         plan.M3BackendId.ValueString(),
		"m3_vpcep_server_ports": plan.M3VpcepServerPorts,
		"m1_vpc_id":             plan.M1VpcId.ValueString(),
		"m1_subnet_id":          plan.M1SubnetId.ValueString(),
		"dns_applicant":         plan.DnsApplicant.ValueString(),
		"dns_domain":            plan.DnsDomain.ValueString(),
		"dns_domain_suffix":     plan.DnsDomainSuffix.ValueString(),
	})

	// ========== Step 1 - 在 M3 侧创建 VPCEP-Server（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 1: 在 M3 侧创建 VPCEP-Server", map[string]interface{}{
		"step":          1,
		"m3_vpc_id":     plan.M3VpcId.ValueString(),
		"m3_backend_id": plan.M3BackendId.ValueString(),
	})
	// TODO: 实现 M3 侧 VPCEP-Server 创建逻辑
	// 1. 构建端口映射: model.PortList{ClientPort, ServerPort, Protocol}
	// 2. 调用 r.M3VpcepClient.CreateEndpointService(request)
	// 3. 返回 serverId

	// ========== Step 2 - 配置 VPCEP-Server 白名单（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 2: 配置 VPCEP-Server 白名单", map[string]interface{}{
		"step": 2,
		"note": "当前跳过，approval_enabled=false",
	})
	// TODO: 如果需要审批，配置白名单逻辑
	// 调用 r.M3VpcepClient.AddOrRemoveServicePermissions(request)

	// ========== Step 3 - 在 M1+ 侧创建 VPCEP-Client（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 3: 在 M1+ 侧创建 VPCEP-Client", map[string]interface{}{
		"step":         3,
		"m1_vpc_id":    plan.M1VpcId.ValueString(),
		"m1_subnet_id": plan.M1SubnetId.ValueString(),
	})
	// TODO: 实现 M1+ 侧 VPCEP-Client 创建逻辑
	// 1. 构建请求: model.CreateEndpointRequestBody{EndpointServiceId, VpcId, SubnetId, EnableDns}
	// 2. 调用 r.M1VpcepClient.CreateEndpoint(request)
	// 3. 返回 clientId, clientIp

	// ========== Step 4 - 轮询等待 Client 状态就绪（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 4: 轮询等待 VPCEP-Client 状态就绪", map[string]interface{}{
		"step": 4,
	})
	// TODO: 实现轮询等待逻辑
	// 调用 r.M1VpcepClient.ListEndpointInfoDetails(request)
	// 轮询间隔: 5s，最大重试: 60次
	// 终态判断: status == "accepted" || status == "pendingAcceptance"

	// ========== Step 5 - 调用内网 DNS API 创建解析记录（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 5: 调用内网 DNS API 创建解析记录", map[string]interface{}{
		"step":              5,
		"dns_applicant":     plan.DnsApplicant.ValueString(),
		"dns_domain":        plan.DnsDomain.ValueString(),
		"dns_domain_suffix": plan.DnsDomainSuffix.ValueString(),
		"note":              "TODO: DNS 内网 API 暂不实现",
	})
	// TODO: 实现内网 DNS API 调用
	// dnsApi.CreateRecord(dnsApplicant, dnsDomain, dnsDomainSuffix, clientIp)

	// 设置状态（框架占位，模拟成功返回）
	tflog.Info(ctx, "[M1→M3] 设置资源状态", map[string]interface{}{
		"vpcep_server_id": "TODO_SERVER_ID",
		"vpcep_client_id": "TODO_CLIENT_ID",
		"vpcep_client_ip": "TODO_CLIENT_IP",
	})
	plan.VpcepServerId = types.StringValue("TODO_SERVER_ID")
	plan.VpcepClientId = types.StringValue("TODO_CLIENT_ID")
	plan.VpcepClientIp = types.StringValue("TODO_CLIENT_IP")

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	tflog.Info(ctx, "[M1→M3] Create 方法出口", map[string]interface{}{
		"method":          "Create",
		"resource":        "kkem_net_connect_m1_to_m3",
		"vpcep_server_id": plan.VpcepServerId.ValueString(),
		"vpcep_client_id": plan.VpcepClientId.ValueString(),
	})
}

// Read 读取当前 M1→M3 网络打通状态。
func (r *NetConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "[M1→M3] Read 方法入口", map[string]interface{}{
		"method":   "Read",
		"resource": "kkem_net_connect_m1_to_m3",
	})

	// 解析状态
	var state NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "[M1→M3] 读取资源状态", map[string]interface{}{
		"vpcep_server_id": state.VpcepServerId.ValueString(),
		"vpcep_client_id": state.VpcepClientId.ValueString(),
	})

	serverId := state.VpcepServerId.ValueString()
	clientId := state.VpcepClientId.ValueString()

	// ========== Step 1 - 查询 VPCEP-Server 状态（TODO）==========
	if serverId != "" && serverId != "TODO_SERVER_ID" {
		tflog.Info(ctx, "[M1→M3] Step 1: 查询 VPCEP-Server 状态", map[string]interface{}{
			"step":      1,
			"server_id": serverId,
		})
		// TODO: 实现 VPCEP-Server 状态查询
		// 调用 r.M3VpcepClient.ListServiceDetails(request)
		// 如果服务不存在: state.VpcepServerId = types.StringNull()
	} else {
		tflog.Debug(ctx, "[M1→M3] VPCEP-Server 状态: TODO 占位符，跳过查询", map[string]interface{}{
			"server_id": serverId,
		})
	}

	// ========== Step 2 - 查询 VPCEP-Client 状态（TODO）==========
	if clientId != "" && clientId != "TODO_CLIENT_ID" {
		tflog.Info(ctx, "[M1→M3] Step 2: 查询 VPCEP-Client 状态", map[string]interface{}{
			"step":      2,
			"client_id": clientId,
		})
		// TODO: 实现 VPCEP-Client 状态查询
		// 调用 r.M1VpcepClient.ListEndpointInfoDetails(request)
		// 如果客户端不存在: state.VpcepClientId = types.StringNull(); state.VpcepClientIp = types.StringNull()
	} else {
		tflog.Debug(ctx, "[M1→M3] VPCEP-Client 状态: TODO 占位符，跳过查询", map[string]interface{}{
			"client_id": clientId,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	tflog.Info(ctx, "[M1→M3] Read 方法出口", map[string]interface{}{
		"method":          "Read",
		"resource":        "kkem_net_connect_m1_to_m3",
		"vpcep_server_id": state.VpcepServerId.ValueString(),
		"vpcep_client_id": state.VpcepClientId.ValueString(),
	})
}

// Update 更新 M1→M3 网络打通（当前不支持）。
func (r *NetConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "[M1→M3] Update 方法入口", map[string]interface{}{
		"method":   "Update",
		"resource": "kkem_net_connect_m1_to_m3",
	})

	resp.Diagnostics.AddError(
		"更新操作不支持",
		"M1→M3 网络打通资源不支持更新操作，如需变更请删除后重新创建",
	)

	tflog.Info(ctx, "[M1→M3] Update 方法出口", map[string]interface{}{
		"method":    "Update",
		"resource":  "kkem_net_connect_m1_to_m3",
		"supported": false,
	})
}

// Delete 执行 M1→M3 网络打通的完整删除流程：
// Step 1 - 调用内网 DNS API 删除解析记录（TODO）
// Step 2 - 删除 M1+ 侧 VPCEP-Client（TODO）
// Step 3 - 删除 M3 侧 VPCEP-Server（TODO）
func (r *NetConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "[M1→M3] Delete 方法入口", map[string]interface{}{
		"method":   "Delete",
		"resource": "kkem_net_connect_m1_to_m3",
	})

	// 解析状态
	var state NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "[M1→M3] 删除资源参数", map[string]interface{}{
		"vpcep_server_id": state.VpcepServerId.ValueString(),
		"vpcep_client_id": state.VpcepClientId.ValueString(),
	})

	// ========== Step 1 - 调用内网 DNS API 删除解析记录（TODO）==========
	tflog.Info(ctx, "[M1→M3] Step 1: 调用内网 DNS API 删除解析记录", map[string]interface{}{
		"step": 1,
	})
	// TODO: 实现内网 DNS API 删除调用
	// dnsApi.DeleteRecord(dnsApplicant, dnsDomain, dnsDomainSuffix)

	// ========== Step 2 - 删除 M1+ 侧 VPCEP-Client（TODO）==========
	clientId := state.VpcepClientId.ValueString()
	if clientId != "" && clientId != "TODO_CLIENT_ID" {
		tflog.Info(ctx, "[M1→M3] Step 2: 删除 M1+ 侧 VPCEP-Client", map[string]interface{}{
			"step":      2,
			"client_id": clientId,
		})
		// TODO: 实现 VPCEP-Client 删除
		// 调用 r.M1VpcepClient.DeleteEndpoint(request)
	} else {
		tflog.Debug(ctx, "[M1→M3] VPCEP-Client: TODO 占位符，跳过删除", map[string]interface{}{
			"client_id": clientId,
		})
	}

	// ========== Step 3 - 删除 M3 侧 VPCEP-Server（TODO）==========
	serverId := state.VpcepServerId.ValueString()
	if serverId != "" && serverId != "TODO_SERVER_ID" {
		tflog.Info(ctx, "[M1→M3] Step 3: 删除 M3 侧 VPCEP-Server", map[string]interface{}{
			"step":      3,
			"server_id": serverId,
		})
		// TODO: 实现 VPCEP-Server 删除
		// 调用 r.M3VpcepClient.DeleteEndpointService(request)
	} else {
		tflog.Debug(ctx, "[M1→M3] VPCEP-Server: TODO 占位符，跳过删除", map[string]interface{}{
			"server_id": serverId,
		})
	}

	tflog.Info(ctx, "[M1→M3] Delete 方法出口", map[string]interface{}{
		"method":   "Delete",
		"resource": "kkem_net_connect_m1_to_m3",
	})
}
