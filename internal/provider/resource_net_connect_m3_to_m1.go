/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/manager"
)

const (
	dnsType           = "intranet_domain"
	vpcepEndpointType = "vpcep_endpoint"
	sniProxyType      = "sni_proxy"

	m3ToM1ResourceTypeName = "_net_connect_m3_to_m1"
)

type netConnectM3ToM1Resource struct {
	m3VpcepEndpointManager vpcepEndpointManager
	m3DnsManager           dnsManager
	m3SniProxyManager      sniProxyManager
}

// m3ToM1CreatedChildResource 用于记录每个成功创建的子资源,便于精确回滚
type m3ToM1CreatedChildResource struct {
	Type string
	ID   string
}

type netConnectM3ToM1ResourceModel struct {
	//vpc-endpoint相关
	M3VpcEndpointId       types.String `tfsdk:"m3_vpcep_id"`
	M3VpcID               types.String `tfsdk:"m3_vpc_id"`
	M3VpcEndpointIp       types.String `tfsdk:"m3_vpcep_ip"`
	M3VpcEndpointSubnetId types.String `tfsdk:"m3_vpcep_subnet_id"`
	SniVpcepServerId      types.String `tfsdk:"sni_vpcep_server_id"`
	//dns相关
	M3DnsDomainName    types.String `tfsdk:"m3_dns_domain_name"`
	M3DnsPrivateZoneId types.String `tfsdk:"m3_dns_privatezone_id"`
	//sni-proxy 相关
	SniProxyResourceId types.String `tfsdk:"sni_proxy_resource_id"`
	RegionCode         types.String `tfsdk:"region_code"`
	ServiceName        types.String `tfsdk:"service_name"`
	DomainAccount      types.String `tfsdk:"m3_iam_domain_account"`
}

func (r *netConnectM3ToM1Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// --- VPCEP Endpoint 相关 ---
			"m3_vpcep_id": schema.StringAttribute{Computed: true},
			"m3_vpc_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"m3_vpcep_ip": schema.StringAttribute{Computed: true},
			"m3_vpcep_subnet_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sni_vpcep_server_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			// --- DNS 相关 ---
			"m3_dns_domain_name":    schema.StringAttribute{Required: true},
			"m3_dns_privatezone_id": schema.StringAttribute{Computed: true},
			// --- SNI Proxy 相关 ---
			"sni_proxy_resource_id": schema.StringAttribute{Computed: true},
			"region_code": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"m3_iam_domain_account": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// NewNetConnectM3ToM1Resource creates and returns a new resource definition for M3-to-M1 network connectivity
func NewNetConnectM3ToM1Resource() resource.Resource {
	return &netConnectM3ToM1Resource{}
}

func (r *netConnectM3ToM1Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + m3ToM1ResourceTypeName
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
	r.m3VpcepEndpointManager = manager.NewVpcepEndpointManager(clients.m3VpcepClient)
	r.m3DnsManager = manager.NewDnsManager(clients.m3DnsClient)
	r.m3SniProxyManager = manager.NewSniProxyManager(clients.sniProxyClient)
}

func (r *netConnectM3ToM1Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {

	var plan netConnectM3ToM1ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created []m3ToM1CreatedChildResource

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

	// Step 1 - 创建 SNI Proxy
	tflog.Info(ctx, "Step 1: Accessing sni-proxy")

	sniProxyResourceId, err := r.m3SniProxyManager.AccessSniProxy(ctx, manager.AccessSniProxyInput{
		RegionCode:       plan.RegionCode.ValueString(),
		ServiceName:      plan.ServiceName.ValueString(),
		IamDomainAccount: []string{plan.DomainAccount.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("create sni-proxy failed", err.Error())
		return
	}

	created = append(created, m3ToM1CreatedChildResource{
		Type: sniProxyType,
		ID:   sniProxyResourceId,
	})

	tflog.Info(ctx, "Step 1 completed", map[string]any{
		"sni_proxy_resource_id": sniProxyResourceId,
	})

	// Step 2 - 创建 VPCEP Endpoint
	tflog.Info(ctx, "Step 2: Creating M3 vpc-endpoint")

	vpcepEndpointId, clientIp, err := r.m3VpcepEndpointManager.Create(ctx, manager.VpcEndpointInput{
		EndpointServiceId: plan.SniVpcepServerId.ValueString(),
		VpcId:             plan.M3VpcID.ValueString(),
		SubnetId:          plan.M3VpcEndpointSubnetId.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("create vpc-endpoint failed", err.Error())
		return
	}

	created = append(created, m3ToM1CreatedChildResource{
		Type: vpcepEndpointType,
		ID:   vpcepEndpointId,
	})

	tflog.Info(ctx, "Step 2 completed", map[string]any{
		"vpcep_endpoint_id": vpcepEndpointId,
		"ip":                clientIp,
	})

	// Step 3.1 - 创建 Intranet Domain
	if !plan.M3DnsDomainName.IsNull() && plan.M3DnsDomainName.ValueString() != "" {
		tflog.Info(ctx, "Step 3.1: Creating M3 intranet domain")

		domainID, err := r.m3DnsManager.CreatePrivateZone(ctx, manager.DnsZoneInput{
			DomainName: plan.M3DnsDomainName.ValueString(),
			RouterId:   plan.M3VpcID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("create M3 intranet domain failed", err.Error())
			return
		}

		created = append(created, m3ToM1CreatedChildResource{
			Type: dnsType,
			ID:   domainID,
		})

		tflog.Info(ctx, "Step 3.1 completed", map[string]any{
			"intranet_domain_id": domainID,
		})

		// Step 3.2 - 创建 Record Set
		tflog.Info(ctx, "Step 3.2: Creating M3 intranet domain record set")

		_, err = r.m3DnsManager.CreateRecordSet(ctx, manager.DnsRecordSetInput{
			ZoneId:  domainID,
			Name:    plan.M3DnsDomainName.ValueString(),
			Records: []string{clientIp},
		})
		if err != nil {
			resp.Diagnostics.AddError("create M3 intranet domain record set failed", err.Error())
			return
		}

		plan.M3DnsPrivateZoneId = types.StringValue(domainID)
	}

	plan.SniProxyResourceId = types.StringValue(sniProxyResourceId)
	plan.M3VpcEndpointId = types.StringValue(vpcepEndpointId)
	plan.M3VpcEndpointIp = types.StringValue(clientIp)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM3ToM1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m3_to_m1: Read called")
	var state netConnectM3ToM1ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.SniProxyResourceId.IsNull() {
		output, getAccessServiceResponse, err := r.m3SniProxyManager.GetSniProxy(ctx,
			state.SniProxyResourceId.ValueString())
		if getAccessServiceResponse != nil && sniproxyclient.IsNotExist(getAccessServiceResponse.Body.Code) {
			tflog.Info(ctx, "sni-proxy-server not found, marking as null", map[string]any{
				"ResourceId": state.SniProxyResourceId.ValueString(),
			})
			state.SniProxyResourceId = types.StringNull()
			state.ServiceName = types.StringNull()
			state.RegionCode = types.StringNull()
			state.DomainAccount = types.StringNull()
		} else if err != nil {
			resp.Diagnostics.AddError("Failed to get SNI proxy", err.Error())
			return
		} else {
			if output.ResourceId == "" {
				state.ServiceName = types.StringNull()
				state.RegionCode = types.StringNull()
				state.DomainAccount = types.StringNull()
			}
			state.SniProxyResourceId = types.StringValue(output.ResourceId)
		}
	}

	if !state.M3DnsPrivateZoneId.IsNull() {
		output, err := r.m3DnsManager.GetPrivateZone(ctx, state.M3DnsPrivateZoneId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("query intranet domain failed", err.Error())
			return
		}
		if output == nil {
			tflog.Info(ctx, "intranet domain not found, marking as null", map[string]any{
				"ZoneId": state.M3DnsPrivateZoneId.ValueString(),
			})
			state.M3DnsPrivateZoneId = types.StringNull()
			state.M3DnsDomainName = types.StringNull()
		}
	}

	if !state.M3VpcEndpointId.IsNull() {
		output, err := r.m3VpcepEndpointManager.Get(ctx, state.M3VpcEndpointId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("query vpc-endpoint failed", err.Error())
			return
		}
		if output == nil {
			tflog.Info(ctx, "vpc-endpoint not found, marking as null", map[string]any{
				"endpoint_id": state.M3VpcEndpointId.ValueString(),
			})
			state.M3VpcEndpointId = types.StringNull()
			state.M3VpcEndpointIp = types.StringNull()
			state.M3VpcID = types.StringNull()
			state.M3VpcEndpointSubnetId = types.StringNull()
			state.SniVpcepServerId = types.StringNull()
		}
	}

	allRemoved := state.M3VpcEndpointId.IsNull() && state.M3DnsPrivateZoneId.IsNull() && state.SniProxyResourceId.IsNull()
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

	var state netConnectM3ToM1ResourceModel
	var plan netConnectM3ToM1ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created []m3ToM1CreatedChildResource
	defer func() {
		if resp.Diagnostics.HasError() {
			tflog.Warn(ctx, "Update failed, starting rollback", map[string]any{
				"created_count": len(created),
			})

			if rollbackErrs := r.rollback(ctx, created); len(rollbackErrs) > 0 {
				details := make([]string, len(rollbackErrs))
				for i, err := range rollbackErrs {
					details[i] = err.Error()
				}
				resp.Diagnostics.AddWarning(
					"resource update failed and rollback encountered errors, manual cleanup may be required",
					fmt.Sprintf("resource creation failed, triggering rollback. errors occurred during rollback, please check and manually clean up residual resources:\n%s",
						strings.Join(details, "\n")),
				)
			}
		}
	}()

	// 仅处理 m3_dns_domain_name 变更，其他字段变化会触发 RequiresReplace，不会进入 Update
	if !plan.M3DnsDomainName.Equal(state.M3DnsDomainName) && !plan.M3DnsDomainName.IsNull() {
		newDomainID, err := r.m3DnsManager.CreatePrivateZone(ctx, manager.DnsZoneInput{
			DomainName: plan.M3DnsDomainName.ValueString(),
			RouterId:   plan.M3VpcID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("create new intranet domain failed", err.Error())
			return
		}

		created = append(created, m3ToM1CreatedChildResource{
			Type: dnsType,
			ID:   newDomainID,
		})

		tflog.Info(ctx, "New intranet domain created", map[string]any{
			"domain_id": newDomainID,
			"domain":    plan.M3DnsDomainName.ValueString(),
		})

		clientIp := state.M3VpcEndpointIp.ValueString()

		_, err = r.m3DnsManager.CreateRecordSet(ctx, manager.DnsRecordSetInput{
			ZoneId:  newDomainID,
			Name:    plan.M3DnsDomainName.ValueString(),
			Records: []string{clientIp},
		})
		if err != nil {
			resp.Diagnostics.AddError("create record set for new domain failed", err.Error())
			return
		}

		if err := r.m3DnsManager.DeletePrivateZone(ctx, state.M3DnsPrivateZoneId.ValueString()); err != nil {
			resp.Diagnostics.AddError(
				"Failed to delete old intranet domain",
				fmt.Sprintf("Old domain ID %s could not be deleted: %s", state.M3DnsPrivateZoneId.ValueString(),
					err.Error()),
			)
		}

		state.M3DnsPrivateZoneId = types.StringValue(newDomainID)
		state.M3DnsDomainName = plan.M3DnsDomainName
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}

func (r *netConnectM3ToM1Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m3_to_m1: Delete started")
	var state netConnectM3ToM1ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var deleteErr error
	if !state.M3DnsPrivateZoneId.IsNull() {
		if err := r.m3DnsManager.DeletePrivateZone(ctx, state.M3DnsPrivateZoneId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete intranet domain %s, the vpc endpoint and sni-proxy remain intact: %w",
				state.M3DnsDomainName.ValueString(), err)
		} else {
			state.M3DnsPrivateZoneId = types.StringNull()
			state.M3DnsDomainName = types.StringNull()
		}
	}

	if !state.M3VpcEndpointId.IsNull() && deleteErr == nil {
		if err := r.m3VpcepEndpointManager.Delete(ctx, state.M3VpcEndpointId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete vpc endpoint %s, the sni-proxy remains intact: %w",
				state.M3VpcEndpointId.ValueString(), err)
		} else {
			state.M3VpcEndpointId = types.StringNull()
			state.M3VpcEndpointIp = types.StringNull()
		}
	}

	if !state.SniProxyResourceId.IsNull() && deleteErr == nil {
		if err := r.m3SniProxyManager.DeleteSniProxy(ctx, state.SniProxyResourceId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete sni-proxy: %w", err)
		} else {
			state.SniProxyResourceId = types.StringNull()
		}
	}

	if deleteErr != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		resp.Diagnostics.AddError("delete m3-to-m1 network connection failed", deleteErr.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *netConnectM3ToM1Resource) rollback(ctx context.Context, created []m3ToM1CreatedChildResource) []error {
	var errs []error

	// 反向删除（后创建的先删）intranet_domain->vpcep-ednpoint->sni-proxy
	for i := len(created) - 1; i >= 0; i-- {
		cr := created[i]

		switch cr.Type {
		case dnsType:
			if err := r.m3DnsManager.DeletePrivateZone(ctx, cr.ID); err != nil {
				errs = append(errs,
					fmt.Errorf("delete %s %s failed: %w", dnsType, cr.ID, err))
			}

		case vpcepEndpointType:
			if err := r.m3VpcepEndpointManager.Delete(ctx, cr.ID); err != nil {
				errs = append(errs,
					fmt.Errorf("delete %s %s failed: %w", vpcepEndpointType, cr.ID, err))
			}

		case sniProxyType:
			if err := r.m3SniProxyManager.DeleteSniProxy(ctx, cr.ID); err != nil {
				errs = append(errs,
					fmt.Errorf("delete %s %s failed: %w", sniProxyType, cr.ID, err))
			}

		default:
			errs = append(errs, fmt.Errorf("unknown resource type: %s", cr.Type))
		}
	}

	return errs
}
