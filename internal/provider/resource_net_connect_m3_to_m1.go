package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NetConnectM3ToM1Resource M3→M1 网络打通 Resource。
type NetConnectM3ToM1Resource struct{}

// NetConnectM3ToM1Model M3→M1 网络打通 Resource 的数据模型。
type NetConnectM3ToM1Model struct {
	// Required 字段
	M3VpcId             types.String `tfsdk:"m3_vpc_id"`
	M3SubnetId          types.String `tfsdk:"m3_subnet_id"`
	M1SniProxyServiceId types.String `tfsdk:"m1_sni_proxy_service_id"`
	ServiceDomain       types.String `tfsdk:"service_domain"`
	// Computed 字段
	VpcepClientId  types.String `tfsdk:"vpcep_client_id"`
	VpcepClientIp  types.String `tfsdk:"vpcep_client_ip"`
	DnsARecordId   types.String `tfsdk:"dns_a_record_id"`
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
	// TODO: 从 req.ProviderData 取出 KkemProviderData
	_ = req.ProviderData
}

// Create 执行 M3→M1 网络打通的完整创建流程：
// Step 1 - 在 M3 侧创建 VPCEP-Client，对接 M1 侧 SNI Proxy EP-Server
// Step 2 - 轮询等待 Client 状态就绪
// Step 3 - 调用华为云标准 DNS 服务创建 A 记录，将业务域名指向 VPCEP-Client IP
func (r *NetConnectM3ToM1Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// TODO: 实现 M3→M1 创建逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Delete 执行 M3→M1 网络打通的完整删除流程：
// Step 1 - 调用华为云标准 DNS 服务删除 A 记录
// Step 2 - 删除 M3 侧 VPCEP-Client
func (r *NetConnectM3ToM1Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// TODO: 实现 M3→M1 删除逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Read 读取当前 M3→M1 网络打通状态。
func (r *NetConnectM3ToM1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// TODO: 实现 M3→M1 Read 逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Update 更新 M3→M1 网络打通（当前不支持）。
func (r *NetConnectM3ToM1Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// TODO: 实现 M3→M1 Update 逻辑（若不支持则返回错误）
	_ = ctx
	_ = req
	_ = resp
}
