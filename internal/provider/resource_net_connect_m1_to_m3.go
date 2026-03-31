package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NetConnectM1ToM3Resource M1→M3 网络打通 Resource。
type NetConnectM1ToM3Resource struct{}

// NetConnectM1ToM3Model M1→M3 网络打通 Resource 的数据模型。
type NetConnectM1ToM3Model struct {
	// Required 字段
	M3VpcId               types.String       `tfsdk:"m3_vpc_id"`
	M3BackendType         types.String       `tfsdk:"m3_backend_type"`
	M3BackendId           types.String       `tfsdk:"m3_backend_id"`
	M3VpcepServerPorts    []VpcepPortBlock   `tfsdk:"m3_vpcep_server_ports"`
	M1VpcId               types.String       `tfsdk:"m1_vpc_id"`
	M1SubnetId            types.String       `tfsdk:"m1_subnet_id"`
	DnsApplicant          types.String       `tfsdk:"dns_applicant"`
	DnsDomain             types.String       `tfsdk:"dns_domain"`
	DnsDomainSuffix       types.String       `tfsdk:"dns_domain_suffix"`
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
	// TODO: 从 req.ProviderData 取出 KkemProviderData
	_ = req.ProviderData
}

// Create 执行 M1→M3 网络打通的完整创建流程：
// Step 1 - 在 M3 侧创建 VPCEP-Server
// Step 2 - 配置 VPCEP-Server 白名单（当前跳过，approval_enabled=false）
// Step 3 - 在 M1+ 侧创建 VPCEP-Client
// Step 4 - 轮询等待 Client 状态就绪
// Step 5 - 调用内网 DNS API 创建解析记录
func (r *NetConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// TODO: 实现 M1→M3 创建逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Delete 执行 M1→M3 网络打通的完整删除流程：
// Step 1 - 调用内网 DNS API 删除解析记录
// Step 2 - 删除 M1+ 侧 VPCEP-Client
// Step 3 - 删除 M3 侧 VPCEP-Server
func (r *NetConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// TODO: 实现 M1→M3 删除逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Read 读取当前 M1→M3 网络打通状态。
func (r *NetConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// TODO: 实现 M1→M3 Read 逻辑
	_ = ctx
	_ = req
	_ = resp
}

// Update 更新 M1→M3 网络打通（当前不支持）。
func (r *NetConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// TODO: 实现 M1→M3 Update 逻辑（若不支持则返回错误）
	_ = ctx
	_ = req
	_ = resp
}
