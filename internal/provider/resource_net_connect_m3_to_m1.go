/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type NetConnectM3ToM1Resource struct{}

type netConnectM3ToM1Model struct {
	M3VpcId                 string `tfsdk:"m3_vpc_id"`
	M3SubnetId              string `tfsdk:"m3_subnet_id"`
	M1PlusSniProxyServiceId string `tfsdk:"m1_plus_sni_proxy_service_id"`
	ServiceDomain           string `tfsdk:"service_domain"`
	VpcepClientId           string `tfsdk:"vpcep_client_id"`
	VpcepClientIp           string `tfsdk:"vpcep_client_ip"`
	DnsARecordId            string `tfsdk:"dns_a_record_id"`
}

func NewNetConnectM3ToM1Resource() resource.Resource {
	return &NetConnectM3ToM1Resource{}
}

func (r *NetConnectM3ToM1Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m3_to_m1"
}

func (r *NetConnectM3ToM1Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m3_vpc_id":                    schema.StringAttribute{Required: true},
			"m3_subnet_id":                 schema.StringAttribute{Required: true},
			"m1_plus_sni_proxy_service_id": schema.StringAttribute{Required: true},
			"service_domain":               schema.StringAttribute{Required: true},
			"vpcep_client_id":              schema.StringAttribute{Computed: true},
			"vpcep_client_ip":              schema.StringAttribute{Computed: true},
			"dns_a_record_id":              schema.StringAttribute{Computed: true},
		},
	}
}

func (r *NetConnectM3ToM1Resource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
}

func (r *NetConnectM3ToM1Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m3_to_m1: Create called")
	var plan netConnectM3ToM1Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	plan.VpcepClientId = "demo_client_id"
	plan.VpcepClientIp = "10.0.0.100"
	plan.DnsARecordId = "demo_dns_record_id"
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetConnectM3ToM1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "kkem_net_connect_m3_to_m1: Read called")
	var state netConnectM3ToM1Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NetConnectM3ToM1Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "kkem_net_connect_m3_to_m1: Update called")
	var plan netConnectM3ToM1Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetConnectM3ToM1Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "kkem_net_connect_m3_to_m1: Delete called")
}
