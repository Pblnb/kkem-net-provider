/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"fmt"
	"huawei.com/kkem/kkem-net-provider/internal/service"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"

	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

type netConnectM3ToM1Resource struct {
	clients *clients
}

// createdResource 用于记录每个成功创建的子资源,便于精确回滚
type createdResource struct {
	Type string
	ID   string
}

type netConnectM3ToM1Model struct {
	//vpc-endpoint相关
	M3VpcEndpointId       types.String `tfsdk:"m3_vpcep_endpoint_id"`
	M3VpcID               string       `tfsdk:"m3_vpc_id"`
	M3VpcEndpointIp       types.String `tfsdk:"m3_vpcep_endpoint_ip"`
	M3VpcEndpointSubnetId string       `tfsdk:"m3_vpcep_endpoint_subnet_id"`
	SniVpcepServerId      string       `tfsdk:"sni_vpcep_server_id"`
	//dns相关
	M3DnsDomain         types.String `tfsdk:"m3_dns_domain"`
	M3DnsZoneType       types.String `tfsdk:"dns_zone_type"`
	M3DnsRecordSetsType types.String `tfsdk:"m3_dns_record_sets_type"`
	M3DnsRecordS        []string     `tfsdk:"m3_dns_records"`
	M3DnsPrivateZoneId  types.String `tfsdk:"m3_dns_privatezone_id"`
	M3DnsRecordId       types.String `tfsdk:"m3_dns_recordid"`
	//sni-proxy 相关
	ResourceId    types.String `tfsdk:"sni_proxy_resource_id"`
	RegionCode    types.String `tfsdk:"region_code"`
	ServiceName   types.String `tfsdk:"service_name"`
	DomainAccount types.String `tfsdk:"m3_iam_domain_account"`
}

func (r *netConnectM3ToM1Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// --- VPCEP Endpoint 相关 ---
			"m3_vpcep_endpoint_id":        schema.StringAttribute{Computed: true},
			"m3_vpc_id":                   schema.StringAttribute{Required: true},
			"m3_vpcep_endpoint_ip":        schema.StringAttribute{Computed: true},
			"m3_vpcep_endpoint_subnet_id": schema.StringAttribute{Required: true},
			"sni_vpcep_server_id":         schema.StringAttribute{Required: true},
			// --- DNS 相关 ---
			"m3_dns_domain":           schema.StringAttribute{Optional: true},
			"dns_zone_type":           schema.StringAttribute{Required: true},
			"m3_dns_record_sets_type": schema.StringAttribute{Required: true},
			"m3_dns_records":          schema.ListAttribute{ElementType: types.StringType, Optional: true},
			"m3_dns_privatezone_id":   schema.StringAttribute{Optional: true},
			"m3_dns_recordid":         schema.StringAttribute{Optional: true},
			// --- SNI Proxy 相关 ---
			"sni_proxy_resource_id": schema.StringAttribute{Optional: true},
			"region_code":           schema.StringAttribute{Required: true},
			"service_name":          schema.StringAttribute{Required: true},
			"m3_iam_domain_account": schema.StringAttribute{Required: true},
		},
	}
}

func NewNetConnectM3ToM1Resource() resource.Resource {
	return &netConnectM3ToM1Resource{}
}

func (r *netConnectM3ToM1Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m3_to_m1"
}

func (r *netConnectM3ToM1Resource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*clients)
	if !ok {
		resp.Diagnostics.AddError("configure error", "invalid provider data")
		return
	}
	r.clients = clients
}

func (r *netConnectM3ToM1Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {

	var plan netConnectM3ToM1Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created []createdResource

	defer func() {
		if resp.Diagnostics.HasError() && len(created) > 0 {
			tflog.Warn(ctx, "Create failed, starting rollback", map[string]any{
				"created_count": len(created),
			})

			if rollbackErrs := r.rollback(ctx, created); len(rollbackErrs) > 0 {
				details := make([]string, len(rollbackErrs))
				for i, err := range rollbackErrs {
					details[i] = err.Error()
				}
				resp.Diagnostics.AddWarning(
					"resource creation failed and rollback encountered errors, manual cleanup may be required",
					fmt.Sprintf("resource creation failed, triggering rollback. errors occurred during rollback, please check and manually clean up residual resources:\n%s",
						strings.Join(details, "\n")),
				)
			}
		}
	}()

	// Step 1 - 创建 SNI Proxy（TODO）

	// Step 2.1 - 创建 VPCEP Endpoint
	tflog.Info(ctx, "Step 2.1: Creating M3 vpc-endpoint")

	vpcepEndpointId, err := service.CreateVpcEndpoint(ctx, r.clients.m3VpcepClient, service.VpcEndpointInput{
		EndpointServiceId: plan.SniVpcepServerId,
		VpcId:             plan.M3VpcID,
		SubnetId:          plan.M3VpcEndpointSubnetId,
	})
	if err != nil {
		resp.Diagnostics.AddError("create vpc-endpoint failed", err.Error())
		return
	}

	created = append(created, createdResource{
		Type: "vpcep_endpoint",
		ID:   vpcepEndpointId,
	})

	tflog.Info(ctx, "Step 2.1 completed", map[string]any{
		"vpcep_endpoint_id": vpcepEndpointId,
	})

	// Step 2.2 - 等待 VPCEP Endpoint Ready
	tflog.Info(ctx, "Step 2.2: Waiting for vpc-endpoint ready")

	clientIp, err := service.WaitForVpcEndpointReady(ctx, r.clients.m3VpcepClient, vpcepEndpointId)
	if err != nil {
		resp.Diagnostics.AddError("wait for vpc-endpoint ready failed", err.Error())
		return
	}

	tflog.Info(ctx, "Step 2.2 completed", map[string]any{
		"vpcep_endpoint_id": vpcepEndpointId,
		"ip":                clientIp,
	})

	// Step 3 - 创建 Intranet Domain（TODO）

	plan.M3VpcEndpointId = types.StringValue(vpcepEndpointId)
	plan.M3VpcEndpointIp = types.StringValue(clientIp)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM3ToM1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m3_to_m1: Read called")
	var state netConnectM3ToM1Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 验证 Intranet Domain 是否存在,更新相关state字段（TODO）

	getReq := &model.ListEndpointInfoDetailsRequest{
		VpcEndpointId: state.M3VpcEndpointId.ValueString(),
	}
	_, err := r.clients.m3VpcepClient.ListEndpointInfoDetails(getReq)
	if err != nil {
		if utils.IsVpcepNotFoundError(err) {
			tflog.Info(ctx, "vpc-endpoint not found, marking as null", map[string]any{
				"endpoint_id": state.M3VpcEndpointId.ValueString(),
			})
			state.M3VpcEndpointId = types.StringNull()
		} else {
			resp.Diagnostics.AddError("query vpc-endpoint failed", err.Error())
			return
		}
	}

	// 验证 sni-proxy 接入状态,更新相关state字段（TODO）

	// 全部子资源均不存在时,移除整个 resource  补充sni-proxy,Intranet Domain资源是否存在的判断（TODO)
	allRemoved := state.M3VpcEndpointId.IsNull()
	if allRemoved {
		tflog.Info(ctx, "All sub-resources not found, removing resource from state")
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *netConnectM3ToM1Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m3_to_m1: Update called")
	var plan netConnectM3ToM1Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	// 补齐相关update逻辑（TODO）

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM3ToM1Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	var state netConnectM3ToM1Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 验证 Intranet Domain 是否存在（TODO）

	if !state.M3VpcEndpointId.IsNull() {
		if err := service.DeleteVpcEndpoint(ctx, r.clients.m3VpcepClient, state.M3VpcEndpointId.ValueString()); err != nil {
			resp.Diagnostics.AddError("delete vpc-endpoint failed", err.Error())
			return
		}
	}

	// 验证 sni-proxy 是否接出（TODO）

	resp.State.RemoveResource(ctx)

	tflog.Info(ctx, "KKEM_net_connect_m3_to_m1: Delete called")
}

func (r *netConnectM3ToM1Resource) rollback(ctx context.Context, created []createdResource) []error {
	var errs []error

	// 反向删除（后创建的先删）domain->vpcep-ednpoint->sni-proxy
	for i := len(created) - 1; i >= 0; i-- {
		cr := created[i]

		switch cr.Type {
		case "domain":
			// TODO: delete domain

		case "vpcep_endpoint":
			if err := service.DeleteVpcEndpoint(ctx, r.clients.m3VpcepClient, cr.ID); err != nil {
				errs = append(errs,
					fmt.Errorf("delete vpc-endpoint %s failed: %w", cr.ID, err))
			}

		case "sni_proxy":
			// TODO: delete sni proxy
		}
	}

	return errs
}
