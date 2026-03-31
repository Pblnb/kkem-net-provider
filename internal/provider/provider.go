package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// KkemProviderData M1/M3 网络打通 Provider 数据结构，Configure 阶段初始化。
type KkemProviderData struct {
	M1VpcepClient interface{} // TODO: *vpcep.VpcepClient M1+ 侧 VPCEP 客户端
	M3VpcepClient interface{} // TODO: *vpcep.VpcepClient M3 侧 VPCEP 客户端
	M3DnsClient   interface{} // TODO: *dns.DnsClient M3 侧华为云标准 DNS 客户端
	M1Ak          types.String
	M1Sk          types.String
	M1ProjectId   types.String
	M3Ak          types.String
	M3Sk          types.String
	M3ProjectId   types.String
	Region        types.String
	DnsApplicant  types.String
}

// KkemProvider 实现 terraform-plugin-framework 的 provider.Provider 接口。
type KkemProvider struct{}

// New 返回新的 KkemProvider 实例。
func New() provider.Provider {
	return &KkemProvider{}
}

// Metadata 返回 Provider 元信息。
func (p *KkemProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kkem"
	resp.Version = "0.1.0"
}

// Schema 定义 Provider 级配置字段。
func (p *KkemProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m1_ak": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "M1+ 侧 Access Key",
			},
			"m1_sk": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "M1+ 侧 Secret Key",
			},
			"m1_project_id": schema.StringAttribute{
				Optional:    true,
				Description: "M1+ 侧 Project ID",
			},
			"m3_ak": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "M3 侧 Access Key",
			},
			"m3_sk": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "M3 侧 Secret Key",
			},
			"m3_project_id": schema.StringAttribute{
				Optional:    true,
				Description: "M3 侧 Project ID",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "华为云 Region，如 cn-north-7",
			},
			"dns_applicant": schema.StringAttribute{
				Optional:    true,
				Description: "DNS 工单申请人工号",
			},
		},
	}
}

// Configure 读取配置字段并初始化 M1/M3 两套客户端。
func (p *KkemProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KkemProviderData

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "初始化 KkemProviderData", map[string]interface{}{
		"region":        data.Region.ValueString(),
		"dns_applicant": data.DnsApplicant.ValueString(),
	})

	// TODO: Step 1 - 使用 m1_ak/m1_sk/m1_project_id 构建 M1+ 侧 VPCEP 客户端
	// TODO: Step 2 - 使用 m3_ak/m3_sk/m3_project_id 构建 M3 侧 VPCEP 客户端
	// TODO: Step 3 - 使用 m3_ak/m3_sk/m3_project_id 构建 M3 侧华为云标准 DNS 客户端

	// 临时使用空接口占位，确保能编译通过
	data.M1VpcepClient = nil
	data.M3VpcepClient = nil
	data.M3DnsClient = nil

	resp.ResourceData = data
}

// DataSources 注册所有 DataSource（当前无）。
func (p *KkemProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

// Resources 注册所有 Resource。
func (p *KkemProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetConnectM1ToM3Resource,
		NewNetConnectM3ToM1Resource,
	}
}

// Ensure interfaces are satisfied
var _ provider.Provider = (*KkemProvider)(nil)
