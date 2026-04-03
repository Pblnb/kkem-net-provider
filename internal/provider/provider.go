/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

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
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
)

// KkemProviderModel Provider 数据结构，在 Configure 方法中被初始化。
type KkemProviderModel struct {
	M1PlusAk        types.String `tfsdk:"m1_plus_ak"`
	M1PlusSk        types.String `tfsdk:"m1_plus_sk"`
	M1PlusProjectId types.String `tfsdk:"m1_plus_project_id"`

	M3Ak        types.String `tfsdk:"m3_ak"`
	M3Sk        types.String `tfsdk:"m3_sk"`
	M3ProjectId types.String `tfsdk:"m3_project_id"`

	VpcepEndpoint types.String `tfsdk:"vpcep_endpoint"`
	DnsEndpoint   types.String `tfsdk:"dns_endpoint"`
}

type Clients struct {
	M1PlusVpcepClient *vpcep.VpcepClient
	M3VpcepClient     *vpcep.VpcepClient
	M3DnsClient       interface{} // TODO
}

type KkemProvider struct {
	version string
}

func NewKKEMProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KkemProvider{
			version: version,
		}
	}
}

func (p *KkemProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kkem"
	resp.Version = p.version
}

func (p *KkemProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m1_plus_ak": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "M1+ Access Key",
			},
			"m1_plus_sk": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "M1+ Secret Key",
			},
			"m1_plus_project_id": schema.StringAttribute{
				Required:    true,
				Description: "M1+ Project ID",
			},
			"m3_ak": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "M3 Access Key",
			},
			"m3_sk": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "M3 Secret Key",
			},
			"m3_project_id": schema.StringAttribute{
				Required:    true,
				Description: "M3 Project ID",
			},
			"vpcep_endpoint": schema.StringAttribute{
				Required:    true,
				Description: "VPCEP 服务 Endpoint，如 https://vpcep.cn-north-7.myhuaweicloud.com",
			},
			"dns_endpoint": schema.StringAttribute{
				Required:    true,
				Description: "DNS 服务 Endpoint，如 https://dns.cn-north-7.myhuaweicloud.com",
			},
		},
	}
}

func newVpcepClient(ak, sk, projectId, endpoint string) (*vpcep.VpcepClient, error) {
	credentials, err := basic.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		WithProjectId(projectId).
		SafeBuild()

	if err != nil {
		return nil, fmt.Errorf("failed to init credential with ak/sk: %w", err)
	}

	hcClient, err := core.NewHcHttpClientBuilder().
		WithCredential(credentials).
		WithEndpoint(endpoint).
		SafeBuild()

	if err != nil {
		return nil, fmt.Errorf("failed to init client with endpoint %s: %w", endpoint, err)
	}

	return vpcep.NewVpcepClient(hcClient), nil
}

// Configure 读取配置字段并初始化 M1+/M3 两套客户端。
func (p *KkemProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KkemProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 构建 M1+ 侧 VPCEP 客户端
	tflog.Info(ctx, "开始初始化 M1+ VPCEP 客户端", map[string]interface{}{
		"endpoint": data.VpcepEndpoint.ValueString(),
	})
	m1PlusVpcepClient, err := newVpcepClient(
		data.M1PlusAk.ValueString(),
		data.M1PlusSk.ValueString(),
		data.M1PlusProjectId.ValueString(),
		data.VpcepEndpoint.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M1+ VPCEP 客户端失败", err.Error())
		return
	}
	tflog.Info(ctx, "M1+ VPCEP 客户端初始化成功", map[string]interface{}{
		"endpoint": data.VpcepEndpoint.ValueString(),
	})

	// 构建 M3 侧 VPCEP 客户端
	tflog.Info(ctx, "开始初始化 M3 VPCEP 客户端", map[string]interface{}{
		"endpoint": data.VpcepEndpoint.ValueString(),
	})
	m3VpcepClient, err := newVpcepClient(
		data.M3Ak.ValueString(),
		data.M3Sk.ValueString(),
		data.M3ProjectId.ValueString(),
		data.VpcepEndpoint.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("创建 M3 VPCEP 客户端失败", err.Error())
		return
	}
	tflog.Info(ctx, "M3 VPCEP 客户端初始化成功", map[string]interface{}{
		"endpoint": data.VpcepEndpoint.ValueString(),
	})

	clients := &Clients{
		M1PlusVpcepClient: m1PlusVpcepClient,
		M3VpcepClient:     m3VpcepClient,
	}

	tflog.Info(ctx, "KkemProvider 初始化完成", map[string]interface{}{
		"m1_plus_project_id": data.M1PlusProjectId.ValueString(),
		"m3_project_id":      data.M3ProjectId.ValueString(),
		"vpcep_endpoint":     data.VpcepEndpoint.ValueString(),
		"dns_endpoint":       data.DnsEndpoint.ValueString(),
	})

	resp.ResourceData = clients
	resp.DataSourceData = clients
}

func (p *KkemProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func (p *KkemProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetConnectM1ToM3Resource,
		NewNetConnectM3ToM1Resource,
	}
}

// 确保 KkemProvider 实现 terraform-plugin-framework 的 provider.Provider 接口。
var _ provider.Provider = (*KkemProvider)(nil)
