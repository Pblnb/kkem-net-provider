package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
)

// KkemProviderData M1/M3 网络打通 Provider 数据结构，Configure 阶段初始化。
type KkemProviderData struct {
	M1VpcepClient *vpcep.VpcepClient // M1+ 侧 VPCEP 客户端
	M3VpcepClient *vpcep.VpcepClient // M3 侧 VPCEP 客户端
	M3DnsClient   interface{}         // M3 侧华为云标准 DNS 客户端 (TODO)
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

// newVpcepClient 创建 VPCEP 客户端（使用 huaweicloud-sdk-go-v3）。
func newVpcepClient(ak, sk, projectId, regionId string) (*vpcep.VpcepClient, error) {
	// 构建 AK/SK 鉴权信息
	credentials := basic.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		WithProjectId(projectId).
		Build()

	// 构建 Region
	reg := region.NewRegion(regionId, "")

	// 构建 HTTP Client
	hcClient := core.NewHcHttpClientBuilder().
		WithCredential(credentials).
		WithRegion(reg).
		WithEndpoint(fmt.Sprintf("https://vpcep.%s.myhuaweicloud.com", regionId)).
		Build()

	return vpcep.NewVpcepClient(hcClient), nil
}

// Configure 读取配置字段并初始化 M1/M3 两套客户端。
func (p *KkemProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KkemProviderData

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	region := data.Region.ValueString()

	// 构建 M1+ 侧 VPCEP 客户端
	m1VpcepClient, err := newVpcepClient(
		data.M1Ak.ValueString(),
		data.M1Sk.ValueString(),
		data.M1ProjectId.ValueString(),
		region,
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M1+ VPCEP 客户端失败", err.Error())
		return
	}

	// 构建 M3 侧 VPCEP 客户端
	m3VpcepClient, err := newVpcepClient(
		data.M3Ak.ValueString(),
		data.M3Sk.ValueString(),
		data.M3ProjectId.ValueString(),
		region,
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M3 VPCEP 客户端失败", err.Error())
		return
	}

	data.M1VpcepClient = m1VpcepClient
	data.M3VpcepClient = m3VpcepClient
	// TODO: M3DnsClient 暂未实现

	tflog.Debug(ctx, "KkemProviderData 初始化完成", map[string]interface{}{
		"region":        region,
		"dns_applicant": data.DnsApplicant.ValueString(),
	})

	resp.ResourceData = &data
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
