package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// NetConnectM3ToM1Resource M3→M1 网络打通 Resource。
type NetConnectM3ToM1Resource struct {
	M3VpcepClient interface{} // M3 侧 VPCEP 客户端（用于创建 VPCEP-Client）
	M3DnsClient   interface{} // M3 侧 DNS 客户端（用于创建 A 记录）
}

// NetConnectM3ToM1Model M3→M1 网络打通 Resource 的数据模型。
type NetConnectM3ToM1Model struct {
	// Required 字段
	M3VpcId             types.String `tfsdk:"m3_vpc_id"`
	M3SubnetId          types.String `tfsdk:"m3_subnet_id"`
	M1SniProxyServiceId types.String `tfsdk:"m1_sni_proxy_service_id"`
	ServiceDomain       types.String `tfsdk:"service_domain"`
	// Computed 字段
	VpcepClientId types.String `tfsdk:"vpcep_client_id"`
	VpcepClientIp types.String `tfsdk:"vpcep_client_ip"`
	DnsARecordId  types.String `tfsdk:"dns_a_record_id"`
}

// NewNetConnectM3ToM1Resource 创建 M3→M1 网络打通 Resource 实例。
func NewNetConnectM3ToM1Resource() resource.Resource {
	return &NetConnectM3ToM1Resource{}
}

// Metadata 返回 Resource 元信息。
func (r *NetConnectM3ToM1Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m3_to_m1"
}

// Schema 定义 M3→M1 网络打通 Resource 的 Schema。
func (r *NetConnectM3ToM1Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required 字段
			"m3_vpc_id": schema.StringAttribute{
				Required: true,
			},
			"m3_subnet_id": schema.StringAttribute{
				Required: true,
			},
			"m1_sni_proxy_service_id": schema.StringAttribute{
				Required: true,
			},
			"service_domain": schema.StringAttribute{
				Required: true,
			},
			// Computed 字段
			"vpcep_client_id": schema.StringAttribute{
				Computed: true,
			},
			"vpcep_client_ip": schema.StringAttribute{
				Computed: true,
			},
			"dns_a_record_id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure 从 req.ProviderData 取出 KkemProviderData。
func (r *NetConnectM3ToM1Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*KkemProviderData)
	if !ok {
		resp.Diagnostics.AddError("Provider data 类型错误", "expected *KkemProviderData")
		return
	}

	r.M3VpcepClient = data.M3VpcepClient
	r.M3DnsClient = data.M3DnsClient

	tflog.Debug(ctx, "NetConnectM3ToM1Resource Configure 完成", map[string]interface{}{
		"has_m3_vpcep_client": r.M3VpcepClient != nil,
		"has_m3_dns_client":   r.M3DnsClient != nil,
	})
}

// Create 执行 M3→M1 网络打通的完整创建流程：
// Step 1 - 在 M3 侧创建 VPCEP-Client，对接 M1 侧 SNI Proxy EP-Server（TODO）
// Step 2 - 轮询等待 Client 状态就绪（TODO）
// Step 3 - 调用华为云标准 DNS 服务创建 A 记录，将业务域名指向 VPCEP-Client IP（TODO）
func (r *NetConnectM3ToM1Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "[M3→M1] Create 方法入口", map[string]interface{}{
		"method":   "Create",
		"resource": "kkem_net_connect_m3_to_m1",
	})

	// 解析配置
	var plan NetConnectM3ToM1Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "[M3→M1] 解析配置失败", map[string]interface{}{
			"diagnostics": resp.Diagnostics.Errors(),
		})
		return
	}

	tflog.Debug(ctx, "[M3→M1] 创建请求参数", map[string]interface{}{
		"m3_vpc_id":               plan.M3VpcId.ValueString(),
		"m3_subnet_id":            plan.M3SubnetId.ValueString(),
		"m1_sni_proxy_service_id": plan.M1SniProxyServiceId.ValueString(),
		"service_domain":          plan.ServiceDomain.ValueString(),
	})

	// ========== Step 1 - 在 M3 侧创建 VPCEP-Client，对接 M1 侧 SNI Proxy EP-Server（TODO）==========
	tflog.Info(ctx, "[M3→M1] Step 1: 在 M3 侧创建 VPCEP-Client", map[string]interface{}{
		"step":                    1,
		"m3_vpc_id":               plan.M3VpcId.ValueString(),
		"m3_subnet_id":            plan.M3SubnetId.ValueString(),
		"m1_sni_proxy_service_id": plan.M1SniProxyServiceId.ValueString(),
	})
	// TODO: 实现 M3 侧 VPCEP-Client 创建逻辑
	// 1. 构建请求: model.CreateEndpointRequestBody{EndpointServiceId, VpcId, SubnetId, EnableDns}
	// 2. 调用 r.M3VpcepClient.CreateEndpoint(request)
	// 3. 返回 clientId, clientIp

	// ========== Step 2 - 轮询等待 Client 状态就绪（TODO）==========
	tflog.Info(ctx, "[M3→M1] Step 2: 轮询等待 VPCEP-Client 状态就绪", map[string]interface{}{
		"step": 2,
	})
	// TODO: 实现轮询等待逻辑
	// 调用 r.M3VpcepClient.ListEndpointInfoDetails(request)
	// 轮询间隔: 5s，最大重试: 60次
	// 终态判断: status == "accepted" || status == "pendingAcceptance"

	// ========== Step 3 - 调用华为云标准 DNS 服务创建 A 记录（TODO）==========
	tflog.Info(ctx, "[M3→M1] Step 3: 调用华为云标准 DNS 创建 A 记录", map[string]interface{}{
		"step":           3,
		"service_domain": plan.ServiceDomain.ValueString(),
	})
	// TODO: 实现华为云标准 DNS A 记录创建
	// 调用 r.M3DnsClient 创建 A 记录，将 service_domain 指向 clientIp

	// 设置状态（框架占位，模拟成功返回）
	tflog.Info(ctx, "[M3→M1] 设置资源状态", map[string]interface{}{
		"vpcep_client_id": "TODO_CLIENT_ID",
		"vpcep_client_ip": "TODO_CLIENT_IP",
		"dns_a_record_id": "TODO_DNS_RECORD_ID",
	})
	plan.VpcepClientId = types.StringValue("TODO_CLIENT_ID")
	plan.VpcepClientIp = types.StringValue("TODO_CLIENT_IP")
	plan.DnsARecordId = types.StringValue("TODO_DNS_RECORD_ID")

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	tflog.Info(ctx, "[M3→M1] Create 方法出口", map[string]interface{}{
		"method":          "Create",
		"resource":        "kkem_net_connect_m3_to_m1",
		"vpcep_client_id": plan.VpcepClientId.ValueString(),
	})
}

// Read 读取当前 M3→M1 网络打通状态。
func (r *NetConnectM3ToM1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "[M3→M1] Read 方法入口", map[string]interface{}{
		"method":   "Read",
		"resource": "kkem_net_connect_m3_to_m1",
	})

	// 解析状态
	var state NetConnectM3ToM1Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "[M3→M1] 读取资源状态", map[string]interface{}{
		"vpcep_client_id": state.VpcepClientId.ValueString(),
		"dns_a_record_id": state.DnsARecordId.ValueString(),
	})

	clientId := state.VpcepClientId.ValueString()
	dnsRecordId := state.DnsARecordId.ValueString()

	// ========== Step 1 - 查询 VPCEP-Client 状态（TODO）==========
	if clientId != "" && clientId != "TODO_CLIENT_ID" {
		tflog.Info(ctx, "[M3→M1] Step 1: 查询 VPCEP-Client 状态", map[string]interface{}{
			"step":      1,
			"client_id": clientId,
		})
		// TODO: 实现 VPCEP-Client 状态查询
		// 调用 r.M3VpcepClient.ListEndpointInfoDetails(request)
		// 如果客户端不存在: state.VpcepClientId = types.StringNull(); state.VpcepClientIp = types.StringNull()
	} else {
		tflog.Debug(ctx, "[M3→M1] VPCEP-Client 状态: TODO 占位符，跳过查询", map[string]interface{}{
			"client_id": clientId,
		})
	}

	// ========== Step 2 - 查询 DNS A 记录状态（TODO）==========
	if dnsRecordId != "" && dnsRecordId != "TODO_DNS_RECORD_ID" {
		tflog.Info(ctx, "[M3→M1] Step 2: 查询 DNS A 记录状态", map[string]interface{}{
			"step":          2,
			"dns_record_id": dnsRecordId,
		})
		// TODO: 实现 DNS A 记录状态查询
		// 如果记录不存在: state.DnsARecordId = types.StringNull()
	} else {
		tflog.Debug(ctx, "[M3→M1] DNS A 记录状态: TODO 占位符，跳过查询", map[string]interface{}{
			"dns_record_id": dnsRecordId,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	tflog.Info(ctx, "[M3→M1] Read 方法出口", map[string]interface{}{
		"method":          "Read",
		"resource":        "kkem_net_connect_m3_to_m1",
		"vpcep_client_id": state.VpcepClientId.ValueString(),
	})
}

// Update 更新 M3→M1 网络打通（当前不支持）。
func (r *NetConnectM3ToM1Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "[M3→M1] Update 方法入口", map[string]interface{}{
		"method":   "Update",
		"resource": "kkem_net_connect_m3_to_m1",
	})

	resp.Diagnostics.AddError(
		"更新操作不支持",
		"M3→M1 网络打通资源不支持更新操作，如需变更请删除后重新创建",
	)

	tflog.Info(ctx, "[M3→M1] Update 方法出口", map[string]interface{}{
		"method":    "Update",
		"resource":  "kkem_net_connect_m3_to_m1",
		"supported": false,
	})
}

// Delete 执行 M3→M1 网络打通的完整删除流程：
// Step 1 - 调用华为云标准 DNS 服务删除 A 记录（TODO）
// Step 2 - 删除 M3 侧 VPCEP-Client（TODO）
func (r *NetConnectM3ToM1Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "[M3→M1] Delete 方法入口", map[string]interface{}{
		"method":   "Delete",
		"resource": "kkem_net_connect_m3_to_m1",
	})

	// 解析状态
	var state NetConnectM3ToM1Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "[M3→M1] 删除资源参数", map[string]interface{}{
		"vpcep_client_id": state.VpcepClientId.ValueString(),
		"dns_a_record_id": state.DnsARecordId.ValueString(),
	})

	// ========== Step 1 - 调用华为云标准 DNS 服务删除 A 记录（TODO）==========
	dnsRecordId := state.DnsARecordId.ValueString()
	if dnsRecordId != "" && dnsRecordId != "TODO_DNS_RECORD_ID" {
		tflog.Info(ctx, "[M3→M1] Step 1: 删除 DNS A 记录", map[string]interface{}{
			"step":          1,
			"dns_record_id": dnsRecordId,
		})
		// TODO: 实现 DNS A 记录删除
		// 调用 r.M3DnsClient 删除 A 记录
	} else {
		tflog.Debug(ctx, "[M3→M1] DNS A 记录: TODO 占位符，跳过删除", map[string]interface{}{
			"dns_record_id": dnsRecordId,
		})
	}

	// ========== Step 2 - 删除 M3 侧 VPCEP-Client（TODO）==========
	clientId := state.VpcepClientId.ValueString()
	if clientId != "" && clientId != "TODO_CLIENT_ID" {
		tflog.Info(ctx, "[M3→M1] Step 2: 删除 M3 侧 VPCEP-Client", map[string]interface{}{
			"step":      2,
			"client_id": clientId,
		})
		// TODO: 实现 VPCEP-Client 删除
		// 调用 r.M3VpcepClient.DeleteEndpoint(request)
	} else {
		tflog.Debug(ctx, "[M3→M1] VPCEP-Client: TODO 占位符，跳过删除", map[string]interface{}{
			"client_id": clientId,
		})
	}

	tflog.Info(ctx, "[M3→M1] Delete 方法出口", map[string]interface{}{
		"method":   "Delete",
		"resource": "kkem_net_connect_m3_to_m1",
	})
}
