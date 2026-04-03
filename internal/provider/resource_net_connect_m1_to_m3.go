/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type NetConnectM1ToM3Resource struct{}

// NetConnectM1ToM3Model M1→M3 网络打通 Resource 的数据模型。
type NetConnectM1ToM3Model struct {
	M3VpcId            types.String     `tfsdk:"m3_vpc_id"`
	M3BackendType      types.String     `tfsdk:"m3_backend_type"`
	M3BackendId        types.String     `tfsdk:"m3_backend_id"`
	M3VpcepServerPorts []VpcepPortBlock `tfsdk:"m3_vpcep_server_ports"`
	M1PlusVpcId        types.String     `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId     types.String     `tfsdk:"m1_plus_subnet_id"`
	DnsDomain          types.String     `tfsdk:"dns_domain"`
	DnsDomainSuffix    types.String     `tfsdk:"dns_domain_suffix"`
	VpcepServerId      types.String     `tfsdk:"vpcep_server_id"`
	VpcepClientId      types.String     `tfsdk:"vpcep_client_id"`
	VpcepClientIp      types.String     `tfsdk:"vpcep_client_ip"`
}

type VpcepPortBlock struct {
	ClientPort types.String `tfsdk:"client_port"`
	ServerPort types.String `tfsdk:"server_port"`
	Protocol   types.String `tfsdk:"protocol"`
}

func NewNetConnectM1ToM3Resource() resource.Resource {
	return &NetConnectM1ToM3Resource{}
}

func (r *NetConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m1_to_m3"
}

func (r *NetConnectM1ToM3Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m3_vpc_id":       schema.StringAttribute{Required: true},
			"m3_backend_type": schema.StringAttribute{Required: true},
			"m3_backend_id":   schema.StringAttribute{Required: true},
			"m3_vpcep_server_ports": schema.ListNestedAttribute{Required: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"client_port": schema.StringAttribute{Required: true},
					"server_port": schema.StringAttribute{Required: true},
					"protocol":    schema.StringAttribute{Required: true},
				}}},
			"m1_plus_vpc_id":    schema.StringAttribute{Required: true},
			"m1_plus_subnet_id": schema.StringAttribute{Required: true},
			"dns_domain":        schema.StringAttribute{Required: true},
			"dns_domain_suffix": schema.StringAttribute{Required: true},
			"vpcep_server_id":   schema.StringAttribute{Computed: true},
			"vpcep_client_id":   schema.StringAttribute{Computed: true},
			"vpcep_client_ip":   schema.StringAttribute{Computed: true},
		},
	}
}

func (r *NetConnectM1ToM3Resource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
}

func (r *NetConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Create called")
	var plan NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	plan.VpcepServerId = types.StringValue("demo_server_id")
	plan.VpcepClientId = types.StringValue("demo_client_id")
	plan.VpcepClientIp = types.StringValue("10.0.0.100")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Read called")
	var state NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NetConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Update called")
	resp.Diagnostics.AddError("TODO：暂不支持更新", "TODO：M1→M3 资源不支持更新操作")
}

func (r *NetConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "kkem_net_connect_m1_to_m3: Delete called")
}
