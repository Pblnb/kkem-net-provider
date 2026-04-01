package provider

import (
	"context"
	"fmt"

	"github.com/chnsz/golangsdk"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// KkemProviderData M1/M3 网络打通 Provider 数据结构，Configure 阶段初始化。
type KkemProviderData struct {
	M1VpcepClient *golangsdk.ServiceClient // M1+ 侧 VPCEP 客户端
	M3VpcepClient *golangsdk.ServiceClient // M3 侧 VPCEP 客户端
	M3DnsClient   *golangsdk.ServiceClient  // M3 侧华为云标准 DNS 客户端 (TODO)
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

// newServiceClient 创建 golangsdk ServiceClient（用于 VPCEP/DNS）。
func newServiceClient(ak, sk, projectId, region, service string) (*golangsdk.ServiceClient, error) {
	// 创建 ProviderClient
	providerClient := &golangsdk.ProviderClient{
		IdentityEndpoint: "",
		ProjectID:        projectId,
		AKSKAuthOptions: golangsdk.AKSKAuthOptions{
			AccessKey:       ak,
			SecretKey:       sk,
			ProjectId:       projectId,
			Region:          region,
			SecurityToken:   "",
			WithUserCatalog: true,
		},
	}

	// 构建 endpoint
	endpoint := fmt.Sprintf("https://%s.%s.myhuaweicloud.com/", service, region)
	resourceBase := fmt.Sprintf("%sv1/%s/", endpoint, projectId)

	// 创建 ServiceClient
	serviceClient := &golangsdk.ServiceClient{
		ProviderClient: providerClient,
		Endpoint:      endpoint,
		ResourceBase:  resourceBase,
	}

	return serviceClient, nil
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
	m1VpcepClient, err := newServiceClient(
		data.M1Ak.ValueString(),
		data.M1Sk.ValueString(),
		data.M1ProjectId.ValueString(),
		region,
		"vpcep",
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M1+ VPCEP 客户端失败", err.Error())
		return
	}

	// 构建 M3 侧 VPCEP 客户端
	m3VpcepClient, err := newServiceClient(
		data.M3Ak.ValueString(),
		data.M3Sk.ValueString(),
		data.M3ProjectId.ValueString(),
		region,
		"vpcep",
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M3 VPCEP 客户端失败", err.Error())
		return
	}

	// 构建 M3 侧 DNS 客户端（TODO）
	m3DnsClient, err := newServiceClient(
		data.M3Ak.ValueString(),
		data.M3Sk.ValueString(),
		data.M3ProjectId.ValueString(),
		region,
		"dns",
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M3 DNS 客户端失败", err.Error())
		return
	}

	data.M1VpcepClient = m1VpcepClient
	data.M3VpcepClient = m3VpcepClient
	data.M3DnsClient = m3DnsClient

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
