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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
)

type cloudCredentials struct {
	Ak        string `tfsdk:"ak"`
	Sk        string `tfsdk:"sk"`
	ProjectId string `tfsdk:"project_id"`
}

// kkemNetProviderModel Provider 数据结构，在 Configure 方法中被初始化。
type kkemNetProviderModel struct {
	M1Plus cloudCredentials `tfsdk:"m1_plus"`
	M3     cloudCredentials `tfsdk:"m3"`

	VpcepEndpoint string `tfsdk:"vpcep_endpoint"`
	DnsEndpoint   string `tfsdk:"dns_endpoint"`
}

type clients struct {
	m1PlusVpcepClient *vpcep.VpcepClient
	m3VpcepClient     *vpcep.VpcepClient
	m3DnsClient       interface{} // TODO
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
			"vpcep_endpoint": schema.StringAttribute{
				Required:    true,
				Description: "VPCEP 服务 Endpoint，如 https://vpcep.cn-north-7.myhuaweicloud.com",
			},
			"dns_endpoint": schema.StringAttribute{
				Required:    true,
				Description: "DNS 服务 Endpoint，如 https://dns.cn-north-7.myhuaweicloud.com",
			},
		},
		Blocks: map[string]schema.Block{
			"m1_plus": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"ak": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "M1+ Access Key",
					},
					"sk": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "M1+ Secret Key",
					},
					"project_id": schema.StringAttribute{
						Required:    true,
						Description: "M1+ Project ID",
					},
				},
			},
			"m3": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"ak": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "M3 Access Key",
					},
					"sk": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "M3 Secret Key",
					},
					"project_id": schema.StringAttribute{
						Required:    true,
						Description: "M3 Project ID",
					},
				},
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
		return nil, fmt.Errorf("failed to init vpcep client with endpoint %s: %w", endpoint, err)
	}

	return vpcep.NewVpcepClient(hcClient), nil
}

func (p *KkemProvider) buildVpcepClient(ctx context.Context,
	label, ak, sk, projectId, endpoint string) (*vpcep.VpcepClient, error) {
	client, err := newVpcepClient(ak, sk, projectId, endpoint)
	if err != nil {
		return nil, fmt.Errorf("create %s VPCEP client failed: %w", label, err)
	}
	tflog.Info(ctx, fmt.Sprintf("%s VPCEP client created", label), map[string]interface{}{
		"endpoint": endpoint,
	})
	return client, nil
}

// Configure 读取配置字段并初始化 M1+/M3 两套客户端。
func (p *KkemProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data kkemNetProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	m1PlusVpcepClient, err := p.buildVpcepClient(ctx, "M1+", data.M1Plus.Ak, data.M1Plus.Sk, data.M1Plus.ProjectId,
		data.VpcepEndpoint)
	if err != nil {
		resp.Diagnostics.AddError("create M1+ VPCEP client failed", err.Error())
		return
	}

	m3VpcepClient, err := p.buildVpcepClient(ctx, "M3", data.M3.Ak, data.M3.Sk, data.M3.ProjectId, data.VpcepEndpoint)
	if err != nil {
		resp.Diagnostics.AddError("create M3 VPCEP client failed", err.Error())
		return
	}

	clients := &clients{
		m1PlusVpcepClient: m1PlusVpcepClient,
		m3VpcepClient:     m3VpcepClient,
	}

	tflog.Info(ctx, "KkemProvider initialized", map[string]interface{}{
		"m1_plus_project_id": data.M1Plus.ProjectId,
		"m3_project_id":      data.M3.ProjectId,
		"vpcep_endpoint":     data.VpcepEndpoint,
		"dns_endpoint":       data.DnsEndpoint,
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
