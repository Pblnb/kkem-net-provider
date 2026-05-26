/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	tfvalidator "github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"huawei.com/kkem/kkem-net-provider/internal/service"
)

const m1ToM3ResourceTypeName = "_net_connect_m1_to_m3"

var lbmDnsRecordValueAttrTypes = map[string]attr.Type{
	"record_type":  types.StringType,
	"record_value": types.StringType,
}

var lbmDnsRecordValueObjectType = types.ObjectType{
	AttrTypes: lbmDnsRecordValueAttrTypes,
}

type netConnectM1ToM3Resource struct {
	m1PlusVpcepService m1ToM3VpcepEndpointService
	m3VpcepService     m1ToM3VpcepService
	lbmDnsService      m1ToM3LbmDnsService
}

type m1ToM3VpcepEndpointService interface {
	Create(ctx context.Context, input service.VpcEndpointInput) (string, string, error)
	Delete(ctx context.Context, endpointId string) error
	Get(ctx context.Context, endpointId string) (*service.VpcepEndpointOutput, error)
}

type m1ToM3VpcepService interface {
	Create(ctx context.Context, input service.VpcepServiceInput) (string, error)
	Delete(ctx context.Context, serviceId string) error
	Get(ctx context.Context, serviceId string) (*service.VpcepServiceOutput, error)
	AddPermissions(ctx context.Context, serviceId string, permissions []service.PermissionInput) error
	GetPermissions(ctx context.Context, serviceId string) (map[string]string, error)
	UpdateConfig(ctx context.Context, serviceId string, input service.VpcepServiceInput) error
	ReconcilePermissions(ctx context.Context, serviceId string, desired []service.PermissionInput) error
}

type m1ToM3LbmDnsService interface {
	CreateIntranetDnsDomain(ctx context.Context, input service.CreateLbmDnsInput) (*service.CreateLbmDnsOutput, error)
	DeleteIntranetDnsDomain(ctx context.Context, recordId string) error
	UpdateRecordValue(ctx context.Context, recordId, endpointIp string) error
	GetLbmDnsDetail(ctx context.Context, recordId string) (*service.LbmDnsDetailOutput, error)
}

type netConnectM1ToM3Model struct {
	// M3 参数
	M3VpcId                   string                        `tfsdk:"m3_vpc_id"`
	M3ServerType              string                        `tfsdk:"m3_server_type"`
	M3PortId                  string                        `tfsdk:"m3_port_id"`
	M3VpcepServicePorts       []vpcepServicePortBlock       `tfsdk:"m3_vpcep_service_ports"`
	M3VpcepServicePermissions []vpcepServicePermissionBlock `tfsdk:"m3_vpcep_service_permissions"`

	// M1+ 参数
	M1PlusVpcId    string `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId string `tfsdk:"m1_plus_subnet_id"`

	// LBM DNS 参数
	DnsDomain         string `tfsdk:"dns_domain"`
	DnsDomainSuffix   string `tfsdk:"dns_domain_suffix"`
	LbmDnsServiceName string `tfsdk:"lbm_dns_service_name"`
	RegionCode        string `tfsdk:"region_code"`

	// 计算参数
	VpcepServiceId         types.String `tfsdk:"vpcep_service_id"`
	VpcepEndpointId        types.String `tfsdk:"vpcep_endpoint_id"`
	VpcepEndpointIp        types.String `tfsdk:"vpcep_endpoint_ip"`
	VpcepEndpointServiceId types.String `tfsdk:"vpcep_endpoint_service_id"`
	LbmDnsRecordId         types.String `tfsdk:"lbm_dns_record_id"`
	LbmDnsRecordValues     types.List   `tfsdk:"lbm_dns_record_values"`
}

type vpcepServicePortBlock struct {
	ClientPort int32 `tfsdk:"client_port"`
	ServerPort int32 `tfsdk:"server_port"`
}

type vpcepServicePermissionBlock struct {
	Permission string `tfsdk:"permission"`
}

type lbmDnsRecordValueBlock struct {
	RecordType  string `tfsdk:"record_type"`
	RecordValue string `tfsdk:"record_value"`
}

type m1ToM3StaleResources struct {
	dnsRecordId string
	endpointId  string
}

func NewNetConnectM1ToM3Resource() resource.Resource {
	return &netConnectM1ToM3Resource{}
}

// requiredM1ToM3StringAttribute 用于普通必填字符串字段，仅校验非空，不影响资源替换策略。
func requiredM1ToM3StringAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:   true,
		Validators: []tfvalidator.String{stringvalidator.LengthAtLeast(1)},
	}
}

// requiredM1ToM3RootStringAttribute 用于决定 M1→M3 根资源身份的必填字符串字段，变更时需要替换资源。
func requiredM1ToM3RootStringAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:   true,
		Validators: []tfvalidator.String{stringvalidator.LengthAtLeast(1)},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

func requiredM1ToM3PortAttribute() schema.Int32Attribute {
	return schema.Int32Attribute{
		Required:   true,
		Validators: []tfvalidator.Int32{int32validator.Between(1, 65535)},
	}
}

func (r *netConnectM1ToM3Resource) Metadata(_ context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + m1ToM3ResourceTypeName
}

func (r *netConnectM1ToM3Resource) Schema(_ context.Context, _ resource.SchemaRequest,
	resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// M3 参数
			"m3_vpc_id":      requiredM1ToM3RootStringAttribute(),
			"m3_server_type": requiredM1ToM3RootStringAttribute(),
			"m3_port_id":     requiredM1ToM3StringAttribute(),
			"m3_vpcep_service_ports": schema.ListNestedAttribute{
				Required:   true,
				Validators: []tfvalidator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client_port": requiredM1ToM3PortAttribute(),
						"server_port": requiredM1ToM3PortAttribute(),
					},
				},
			},
			"m3_vpcep_service_permissions": schema.ListNestedAttribute{
				Required:   true,
				Validators: []tfvalidator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"permission": requiredM1ToM3StringAttribute(),
					},
				},
			},

			// M1+ 参数
			"m1_plus_vpc_id":    requiredM1ToM3StringAttribute(),
			"m1_plus_subnet_id": requiredM1ToM3StringAttribute(),

			// LBM DNS 参数
			"dns_domain":           requiredM1ToM3StringAttribute(),
			"dns_domain_suffix":    requiredM1ToM3StringAttribute(),
			"lbm_dns_service_name": requiredM1ToM3StringAttribute(),
			"region_code":          requiredM1ToM3StringAttribute(),

			// 计算参数
			"vpcep_service_id":          schema.StringAttribute{Computed: true},
			"vpcep_endpoint_id":         schema.StringAttribute{Computed: true},
			"vpcep_endpoint_ip":         schema.StringAttribute{Computed: true},
			"vpcep_endpoint_service_id": schema.StringAttribute{Computed: true},
			"lbm_dns_record_id":         schema.StringAttribute{Computed: true},
			"lbm_dns_record_values": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"record_type":  schema.StringAttribute{Computed: true},
						"record_value": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (r *netConnectM1ToM3Resource) Configure(_ context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*clients)
	if !ok {
		resp.Diagnostics.AddError("configure error", "invalid provider data type")
		return
	}
	r.m1PlusVpcepService = service.NewVpcepEndpointService(clients.m1PlusVpcepClient)
	r.m3VpcepService = service.NewVpcepService(clients.m3VpcepClient)
	r.lbmDnsService = service.NewLbmDnsService(clients.lbmDnsClient)
}

func (r *netConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse) {
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Create started", map[string]any{
		"m3_vpc_id":      plan.M3VpcId,
		"m3_port_id":     plan.M3PortId,
		"m1_plus_vpc_id": plan.M1PlusVpcId,
	})

	success := false
	defer r.rollbackCreateOnFailure(ctx, &plan, &success, resp)

	vpcepServiceId, err := r.createAndWaitVpcepService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("create vpcep-service failed", err.Error())
		return
	}

	vpcepEndpointId, endpointIp, err := r.createAndWaitVpcepEndpoint(ctx, &plan, vpcepServiceId)
	if err != nil {
		resp.Diagnostics.AddError("create vpcep-endpoint failed", err.Error())
		return
	}

	dnsRecordId, err := r.createAndWaitLbmDnsRecord(ctx, &plan, endpointIp)
	if err != nil {
		resp.Diagnostics.AddError("create lbm-dns record failed", err.Error())
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}

	success = true
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Create completed", map[string]any{
		"service_id":    vpcepServiceId,
		"endpoint_id":   vpcepEndpointId,
		"endpoint_ip":   endpointIp,
		"dns_record_id": dnsRecordId,
	})
	normalizeM1ToM3ListState(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM1ToM3Resource) createAndWaitVpcepService(ctx context.Context,
	plan *netConnectM1ToM3Model) (vpcepServiceId string, err error) {
	vpcepServiceId, err = r.m3VpcepService.Create(ctx, service.VpcepServiceInput{
		VpcId:      plan.M3VpcId,
		PortId:     plan.M3PortId,
		ServerType: plan.M3ServerType,
		Ports:      convertPorts(plan.M3VpcepServicePorts),
	})
	if err != nil {
		return "", fmt.Errorf("create vpcep-service failed: %w", err)
	}
	plan.VpcepServiceId = types.StringValue(vpcepServiceId)
	tflog.Info(ctx, "Step 1 completed: vpcep-service created and ready", map[string]any{
		"service_id": vpcepServiceId,
	})

	permissions := convertPermissions(plan.M3VpcepServicePermissions)
	if err := r.m3VpcepService.AddPermissions(ctx, vpcepServiceId, permissions); err != nil {
		return "", fmt.Errorf("add vpcep-service permission failed: %w", err)
	}
	tflog.Info(ctx, "Step 2 completed: vpcep-service permission added", map[string]any{
		"service_id":  vpcepServiceId,
		"permissions": len(plan.M3VpcepServicePermissions),
	})
	return vpcepServiceId, nil
}

func (r *netConnectM1ToM3Resource) createAndWaitVpcepEndpoint(ctx context.Context, plan *netConnectM1ToM3Model,
	vpcepServiceId string) (vpcepEndpointId string, endpointIp string, err error) {
	vpcepEndpointId, endpointIp, err = r.m1PlusVpcepService.Create(ctx, service.VpcEndpointInput{
		EndpointServiceId: vpcepServiceId,
		VpcId:             plan.M1PlusVpcId,
		SubnetId:          plan.M1PlusSubnetId,
	})
	if err != nil {
		return "", "", fmt.Errorf("create vpcep-endpoint failed: %w", err)
	}
	plan.VpcepEndpointId = types.StringValue(vpcepEndpointId)
	plan.VpcepEndpointServiceId = types.StringValue(vpcepServiceId)
	plan.VpcepEndpointIp = types.StringValue(endpointIp)
	tflog.Info(ctx, "Step 3 completed: vpcep-endpoint created and ready", map[string]any{
		"endpoint_id": vpcepEndpointId,
		"ip":          endpointIp,
	})
	return vpcepEndpointId, endpointIp, nil
}

func (r *netConnectM1ToM3Resource) createAndWaitLbmDnsRecord(ctx context.Context, plan *netConnectM1ToM3Model,
	endpointIp string) (dnsRecordId string, err error) {
	dnsOutput, err := r.lbmDnsService.CreateIntranetDnsDomain(ctx, service.CreateLbmDnsInput{
		RegionCode:   plan.RegionCode,
		ServiceName:  plan.LbmDnsServiceName,
		HostRecord:   plan.DnsDomain,
		DomainSuffix: plan.DnsDomainSuffix,
		EndpointIp:   endpointIp,
	})
	if err != nil {
		return "", fmt.Errorf("create lbm-dns record failed: %w", err)
	}
	plan.LbmDnsRecordId = types.StringValue(dnsOutput.RecordId)
	if err := setLbmDnsRecordValues(plan, endpointIp); err != nil {
		return "", err
	}
	tflog.Info(ctx, "Step 4 completed: lbm-dns record created", map[string]any{
		"dns_record_id": dnsOutput.RecordId,
	})
	return dnsOutput.RecordId, nil
}

func (r *netConnectM1ToM3Resource) rollbackCreateOnFailure(ctx context.Context, plan *netConnectM1ToM3Model,
	success *bool, resp *resource.CreateResponse) {
	if *success {
		return
	}
	tflog.Info(ctx, "Create failed, executing rollback", map[string]any{})
	if rollbackErrs := r.rollbackCreate(ctx, plan); len(rollbackErrs) > 0 {
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

func setLbmDnsRecordValues(plan *netConnectM1ToM3Model, endpointIp string) error {
	lbmDnsRecordValues, recordValueDiags := buildLbmDnsRecordValues([]lbmDnsRecordValueBlock{
		{
			RecordType:  "A",
			RecordValue: endpointIp,
		},
	})
	if recordValueDiags.HasError() {
		return fmt.Errorf("build lbm-dns record values failed: %s", recordValueDiags.Errors()[0].Detail())
	}
	plan.LbmDnsRecordValues = lbmDnsRecordValues
	return nil
}

func (r *netConnectM1ToM3Resource) rollbackCreate(ctx context.Context, plan *netConnectM1ToM3Model) []error {
	var errs []error
	if !plan.VpcepEndpointId.IsNull() {
		if err := r.m1PlusVpcepService.Delete(ctx, plan.VpcepEndpointId.ValueString()); err != nil {
			errs = append(errs,
				fmt.Errorf("delete vpcep-endpoint %s failed: %w", plan.VpcepEndpointId.ValueString(), err))
		}
	}
	if !plan.VpcepServiceId.IsNull() {
		if err := r.m3VpcepService.Delete(ctx, plan.VpcepServiceId.ValueString()); err != nil {
			errs = append(errs,
				fmt.Errorf("delete vpcep-service %s failed: %w", plan.VpcepServiceId.ValueString(), err))
		}
	}
	return errs
}

// Read 逻辑（Empty-State Repair 策略）：
// - 查询所有子资源的存在性并回填远端真实属性
// - 若某个子资源返回 404，则将其 Computed 字段置为 null，并清空对应输入字段
// - 若所有子资源均不存在，则调用 RemoveResource
func (r *netConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Read started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.refreshVpcepServiceState(ctx, &state); err != nil {
		resp.Diagnostics.AddError("query vpcep-service failed", err.Error())
		return
	}

	if err := r.refreshVpcepEndpointState(ctx, &state); err != nil {
		resp.Diagnostics.AddError("query vpcep-endpoint failed", err.Error())
		return
	}

	resp.Diagnostics.Append(r.refreshLbmDnsState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if m1ToM3AllChildIdentitiesMissing(state) {
		tflog.Info(ctx, "All sub-resources not found, removing resource from state")
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *netConnectM1ToM3Resource) refreshVpcepServiceState(ctx context.Context,
	state *netConnectM1ToM3Model) error {
	if state.VpcepServiceId.IsNull() {
		return nil
	}

	output, err := r.m3VpcepService.Get(ctx, state.VpcepServiceId.ValueString())
	if err != nil {
		return err
	}
	if output == nil {
		tflog.Info(ctx, "Vpcep-service not found, marking as empty state", map[string]any{
			"service_id": state.VpcepServiceId.ValueString(),
		})
		state.VpcepServiceId = types.StringNull()
		clearM1ToM3ServiceInputState(state)
		return nil
	}

	state.VpcepServiceId = types.StringValue(output.ServiceId)
	if output.VpcId != "" {
		state.M3VpcId = output.VpcId
	}
	if output.PortId != "" {
		state.M3PortId = output.PortId
	}
	if output.ServerType != "" {
		state.M3ServerType = output.ServerType
	}
	if len(output.Ports) > 0 {
		state.M3VpcepServicePorts = normalizePortPairs(output.Ports)
	}

	if err := r.syncVpcepServicePermissionState(ctx, state, output.ServiceId); err != nil {
		return fmt.Errorf("query vpcep-service permission failed: %w", err)
	}
	return nil
}

func (r *netConnectM1ToM3Resource) refreshVpcepEndpointState(ctx context.Context,
	state *netConnectM1ToM3Model) error {
	if state.VpcepEndpointId.IsNull() {
		return nil
	}

	output, err := r.m1PlusVpcepService.Get(ctx, state.VpcepEndpointId.ValueString())
	if err != nil {
		return err
	}
	if output == nil {
		tflog.Info(ctx, "Vpcep-endpoint not found, marking as empty state", map[string]any{
			"endpoint_id": state.VpcepEndpointId.ValueString(),
		})
		state.VpcepEndpointId = types.StringNull()
		state.VpcepEndpointIp = types.StringNull()
		state.VpcepEndpointServiceId = types.StringNull()
		clearM1ToM3EndpointInputState(state)
		return nil
	}

	state.VpcepEndpointId = types.StringValue(output.EndpointId)
	if output.Ip != "" {
		state.VpcepEndpointIp = types.StringValue(output.Ip)
	}
	if output.VpcId != "" {
		state.M1PlusVpcId = output.VpcId
	}
	if output.SubnetId != "" {
		state.M1PlusSubnetId = output.SubnetId
	}
	if output.ServiceId != "" {
		state.VpcepEndpointServiceId = types.StringValue(output.ServiceId)
	}
	return nil
}

func (r *netConnectM1ToM3Resource) refreshLbmDnsState(ctx context.Context,
	state *netConnectM1ToM3Model) diag.Diagnostics {
	if state.LbmDnsRecordId.IsNull() {
		return nil
	}

	dnsDetail, err := r.lbmDnsService.GetLbmDnsDetail(ctx, state.LbmDnsRecordId.ValueString())
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("query lbm-dns record failed", err.Error())
		return diags
	}
	if dnsDetail == nil {
		tflog.Info(ctx, "lbm-dns record not found, marking as empty state", map[string]any{
			"dns_record_id": state.LbmDnsRecordId.ValueString(),
		})
		state.LbmDnsRecordId = types.StringNull()
		state.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
		clearM1ToM3DnsInputState(state)
		return nil
	}

	state.LbmDnsRecordId = types.StringValue(dnsDetail.RecordId)
	state.RegionCode = dnsDetail.RegionCode
	state.LbmDnsServiceName = dnsDetail.ServiceName
	state.DnsDomain = dnsDetail.HostRecord
	state.DnsDomainSuffix = dnsDetail.DomainSuffix

	recordValues := make([]lbmDnsRecordValueBlock, len(dnsDetail.RecordValues))
	for i, rv := range dnsDetail.RecordValues {
		recordValues[i] = lbmDnsRecordValueBlock{
			RecordType:  rv.RecordType,
			RecordValue: rv.RecordValue,
		}
	}
	values, diags := buildLbmDnsRecordValues(normalizeLbmDnsRecordValueBlocks(recordValues))
	state.LbmDnsRecordValues = values
	return diags
}

// syncVpcepServicePermissionState 查询并同步 VPCEP-Service 的权限列表。
func (r *netConnectM1ToM3Resource) syncVpcepServicePermissionState(ctx context.Context, state *netConnectM1ToM3Model,
	serviceId string) error {
	remote, err := r.m3VpcepService.GetPermissions(ctx, serviceId)
	if err != nil {
		return err
	}
	permissions := make([]vpcepServicePermissionBlock, 0, len(remote))
	for permission := range remote {
		permissions = append(permissions, vpcepServicePermissionBlock{Permission: permission})
	}
	state.M3VpcepServicePermissions = normalizeVpcepServicePermissionBlocks(permissions)
	tflog.Debug(ctx, "Vpcep-service permissions synced", map[string]any{
		"service_id":  serviceId,
		"permissions": len(state.M3VpcepServicePermissions),
	})
	return nil
}

func normalizeM1ToM3ListState(state *netConnectM1ToM3Model) {
	state.M3VpcepServicePorts = normalizeVpcepServicePortBlocks(state.M3VpcepServicePorts)
	state.M3VpcepServicePermissions = normalizeVpcepServicePermissionBlocks(state.M3VpcepServicePermissions)
}

// normalizePortPairs 将 service 层端口对转换为 Resource state 端口块，并按 client_port、server_port 排序。
func normalizePortPairs(pairs []service.PortPair) []vpcepServicePortBlock {
	blocks := make([]vpcepServicePortBlock, len(pairs))
	for i, p := range pairs {
		blocks[i] = vpcepServicePortBlock{ClientPort: p.ClientPort, ServerPort: p.ServerPort}
	}
	return normalizeVpcepServicePortBlocks(blocks)
}

func normalizeVpcepServicePortBlocks(ports []vpcepServicePortBlock) []vpcepServicePortBlock {
	normalizedPorts := append([]vpcepServicePortBlock(nil), ports...)
	sort.Slice(normalizedPorts, func(i, j int) bool {
		if normalizedPorts[i].ClientPort == normalizedPorts[j].ClientPort {
			return normalizedPorts[i].ServerPort < normalizedPorts[j].ServerPort
		}
		return normalizedPorts[i].ClientPort < normalizedPorts[j].ClientPort
	})
	return normalizedPorts
}

// buildLbmDnsRecordValues 将 DNS 记录值块排序后转换为 Terraform List state。
func buildLbmDnsRecordValues(values []lbmDnsRecordValueBlock) (types.List, diag.Diagnostics) {
	normalizedValues := normalizeLbmDnsRecordValueBlocks(values)

	elements := make([]attr.Value, 0, len(normalizedValues))
	var diagnostics diag.Diagnostics
	for _, value := range normalizedValues {
		objectValue, diags := types.ObjectValue(lbmDnsRecordValueAttrTypes, map[string]attr.Value{
			"record_type":  types.StringValue(value.RecordType),
			"record_value": types.StringValue(value.RecordValue),
		})
		diagnostics.Append(diags...)
		if diagnostics.HasError() {
			return types.ListUnknown(lbmDnsRecordValueObjectType), diagnostics
		}
		elements = append(elements, objectValue)
	}

	listValue, diags := types.ListValue(lbmDnsRecordValueObjectType, elements)
	diagnostics.Append(diags...)

	if diagnostics.HasError() {
		return types.ListUnknown(lbmDnsRecordValueObjectType), diagnostics
	}

	return listValue, diagnostics
}

func normalizeLbmDnsRecordValueBlocks(values []lbmDnsRecordValueBlock) []lbmDnsRecordValueBlock {
	normalizedValues := append([]lbmDnsRecordValueBlock(nil), values...)
	sort.Slice(normalizedValues, func(i, j int) bool {
		if normalizedValues[i].RecordType == normalizedValues[j].RecordType {
			return normalizedValues[i].RecordValue < normalizedValues[j].RecordValue
		}
		return normalizedValues[i].RecordType < normalizedValues[j].RecordType
	})
	return normalizedValues
}

func normalizeVpcepServicePermissionBlocks(permissions []vpcepServicePermissionBlock) []vpcepServicePermissionBlock {
	normalizedPermissions := append([]vpcepServicePermissionBlock(nil), permissions...)
	sort.Slice(normalizedPermissions, func(i, j int) bool {
		return normalizedPermissions[i].Permission < normalizedPermissions[j].Permission
	})
	return normalizedPermissions
}

func (r *netConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Update called")
	var plan netConnectM1ToM3Model
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	preserveKnownComputedFields(&plan, state)

	var stale m1ToM3StaleResources

	serviceReplaced, err := r.reconcileM1ToM3Service(ctx, state, &plan)
	if err != nil {
		resp.Diagnostics.AddError("reconcile vpcep-service failed", err.Error())
		return
	}
	if !setM1ToM3UpdateState(ctx, resp, &plan) {
		return
	}

	if err := r.reconcileM1ToM3Endpoint(ctx, state, &plan, serviceReplaced, &stale); err != nil {
		resp.Diagnostics.AddError("reconcile vpcep-endpoint failed", err.Error())
		return
	}
	if !setM1ToM3UpdateState(ctx, resp, &plan) {
		return
	}

	diags := r.reconcileM1ToM3Dns(ctx, state, &plan, &stale)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !setM1ToM3UpdateState(ctx, resp, &plan) {
		return
	}

	r.cleanupStaleM1ToM3Resources(ctx, stale, resp)
}

func setM1ToM3UpdateState(ctx context.Context, resp *resource.UpdateResponse, plan *netConnectM1ToM3Model) bool {
	normalizeM1ToM3ListState(plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	return !resp.Diagnostics.HasError()
}

func (r *netConnectM1ToM3Resource) reconcileM1ToM3Service(ctx context.Context, state netConnectM1ToM3Model,
	plan *netConnectM1ToM3Model) (bool, error) {
	if state.VpcepServiceId.IsNull() {
		return false, fmt.Errorf("vpcep-service is missing; Terraform replacement is required")
	}
	if serviceRequiresReplacement(state, *plan) {
		return false, fmt.Errorf("vpcep-service replacement should be handled by Terraform resource replacement")
	}
	if serviceRequiresInPlaceUpdate(state, *plan) {
		if err := r.updateExistingM1ToM3Service(ctx, state, plan); err != nil {
			return false, err
		}
		plan.VpcepServiceId = state.VpcepServiceId
	}
	return false, nil
}

func (r *netConnectM1ToM3Resource) updateExistingM1ToM3Service(ctx context.Context, state netConnectM1ToM3Model,
	plan *netConnectM1ToM3Model) error {
	serviceId := state.VpcepServiceId.ValueString()
	if servicePortConfigChanged(state, *plan) {
		if err := r.m3VpcepService.UpdateConfig(ctx, serviceId, service.VpcepServiceInput{
			VpcId:      plan.M3VpcId,
			PortId:     plan.M3PortId,
			ServerType: plan.M3ServerType,
			Ports:      convertPorts(plan.M3VpcepServicePorts),
		}); err != nil {
			return err
		}
	}
	if servicePermissionsChanged(state, *plan) {
		return r.m3VpcepService.ReconcilePermissions(ctx, serviceId, convertPermissions(plan.M3VpcepServicePermissions))
	}
	return nil
}

func (r *netConnectM1ToM3Resource) reconcileM1ToM3Endpoint(ctx context.Context, state netConnectM1ToM3Model,
	plan *netConnectM1ToM3Model, serviceReplaced bool, stale *m1ToM3StaleResources) error {
	endpointReplace := shouldReplaceEndpoint(state, *plan, serviceReplaced)
	if !state.VpcepEndpointId.IsNull() && !endpointReplace {
		return nil
	}

	oldEndpointId := ""
	if endpointReplace {
		oldEndpointId = state.VpcepEndpointId.ValueString()
	}
	endpointId, endpointIp, err := r.createAndWaitVpcepEndpoint(ctx, plan, plan.VpcepServiceId.ValueString())
	if err != nil {
		return err
	}
	plan.VpcepEndpointId = types.StringValue(endpointId)
	plan.VpcepEndpointIp = types.StringValue(endpointIp)
	plan.VpcepEndpointServiceId = plan.VpcepServiceId
	if oldEndpointId != "" {
		stale.endpointId = oldEndpointId
	}
	return nil
}

func (r *netConnectM1ToM3Resource) reconcileM1ToM3Dns(ctx context.Context, state netConnectM1ToM3Model,
	plan *netConnectM1ToM3Model, stale *m1ToM3StaleResources) diag.Diagnostics {
	var diags diag.Diagnostics
	dnsIdentityChanged := !state.LbmDnsRecordId.IsNull() && dnsIdentityChanged(state, *plan)
	dnsValuesChanged, valueDiags := lbmDnsRecordValueNeedsUpdate(ctx, state.LbmDnsRecordValues, plan.VpcepEndpointIp)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return diags
	}

	switch {
	case state.LbmDnsRecordId.IsNull():
		if _, err := r.createAndWaitLbmDnsRecord(ctx, plan, plan.VpcepEndpointIp.ValueString()); err != nil {
			diags.AddError("create lbm-dns record failed", err.Error())
		}
	case dnsIdentityChanged:
		oldRecordId := state.LbmDnsRecordId.ValueString()
		if _, err := r.createAndWaitLbmDnsRecord(ctx, plan, plan.VpcepEndpointIp.ValueString()); err != nil {
			diags.AddError("replace lbm-dns record failed", err.Error())
			return diags
		}
		stale.dnsRecordId = oldRecordId
	case dnsValuesChanged:
		if err := r.lbmDnsService.UpdateRecordValue(ctx, state.LbmDnsRecordId.ValueString(),
			plan.VpcepEndpointIp.ValueString()); err != nil {
			diags.AddError("update lbm-dns record failed", err.Error())
			return diags
		}
		plan.LbmDnsRecordId = state.LbmDnsRecordId
		if err := setLbmDnsRecordValues(plan, plan.VpcepEndpointIp.ValueString()); err != nil {
			diags.AddError("sync lbm-dns record values failed", err.Error())
		}
	}
	return diags
}

func preserveKnownComputedFields(plan *netConnectM1ToM3Model, state netConnectM1ToM3Model) {
	if plan.VpcepServiceId.IsUnknown() {
		plan.VpcepServiceId = state.VpcepServiceId
	}
	if plan.VpcepEndpointId.IsUnknown() {
		plan.VpcepEndpointId = state.VpcepEndpointId
	}
	if plan.VpcepEndpointIp.IsUnknown() {
		plan.VpcepEndpointIp = state.VpcepEndpointIp
	}
	if plan.VpcepEndpointServiceId.IsUnknown() {
		plan.VpcepEndpointServiceId = state.VpcepEndpointServiceId
	}
	if plan.LbmDnsRecordId.IsUnknown() {
		plan.LbmDnsRecordId = state.LbmDnsRecordId
	}
	if plan.LbmDnsRecordValues.IsUnknown() {
		plan.LbmDnsRecordValues = state.LbmDnsRecordValues
	}
}

func clearM1ToM3ServiceInputState(state *netConnectM1ToM3Model) {
	state.M3VpcId = ""
	state.M3ServerType = ""
	state.M3PortId = ""
	state.M3VpcepServicePorts = []vpcepServicePortBlock{}
	state.M3VpcepServicePermissions = []vpcepServicePermissionBlock{}
}

func clearM1ToM3EndpointInputState(state *netConnectM1ToM3Model) {
	state.M1PlusVpcId = ""
	state.M1PlusSubnetId = ""
}

func clearM1ToM3DnsInputState(state *netConnectM1ToM3Model) {
	state.DnsDomain = ""
	state.DnsDomainSuffix = ""
	state.LbmDnsServiceName = ""
	state.RegionCode = ""
}

func m1ToM3AllChildIdentitiesMissing(state netConnectM1ToM3Model) bool {
	return state.VpcepServiceId.IsNull() && state.VpcepEndpointId.IsNull() && state.LbmDnsRecordId.IsNull()
}

func serviceRequiresReplacement(state, plan netConnectM1ToM3Model) bool {
	return state.M3VpcId != plan.M3VpcId || state.M3ServerType != plan.M3ServerType
}

func serviceRequiresInPlaceUpdate(state, plan netConnectM1ToM3Model) bool {
	return servicePortConfigChanged(state, plan) || servicePermissionsChanged(state, plan)
}

func servicePortConfigChanged(state, plan netConnectM1ToM3Model) bool {
	return state.M3PortId != plan.M3PortId ||
		!reflect.DeepEqual(normalizeVpcepServicePortBlocks(state.M3VpcepServicePorts),
			normalizeVpcepServicePortBlocks(plan.M3VpcepServicePorts))
}

func servicePermissionsChanged(state, plan netConnectM1ToM3Model) bool {
	return !reflect.DeepEqual(normalizeVpcepServicePermissionBlocks(state.M3VpcepServicePermissions),
		normalizeVpcepServicePermissionBlocks(plan.M3VpcepServicePermissions))
}

func shouldReplaceEndpoint(state, plan netConnectM1ToM3Model, serviceReplaced bool) bool {
	if state.VpcepEndpointId.IsNull() {
		return false
	}
	if serviceReplaced {
		return true
	}
	if state.M1PlusVpcId != plan.M1PlusVpcId || state.M1PlusSubnetId != plan.M1PlusSubnetId {
		return true
	}
	if state.VpcepEndpointServiceId.IsNull() || plan.VpcepServiceId.IsNull() || plan.VpcepServiceId.IsUnknown() {
		return true
	}
	return state.VpcepEndpointServiceId.ValueString() != plan.VpcepServiceId.ValueString()
}

func dnsIdentityChanged(state, plan netConnectM1ToM3Model) bool {
	return state.RegionCode != plan.RegionCode ||
		state.LbmDnsServiceName != plan.LbmDnsServiceName ||
		state.DnsDomain != plan.DnsDomain ||
		state.DnsDomainSuffix != plan.DnsDomainSuffix
}

func lbmDnsRecordValueNeedsUpdate(ctx context.Context, values types.List,
	endpointIp types.String) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if endpointIp.IsNull() || endpointIp.IsUnknown() {
		return true, diags
	}
	currentValue, found, valueDiags := lbmDnsRecordAValue(ctx, values)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return false, diags
	}
	return !found || currentValue != endpointIp.ValueString(), diags
}

func lbmDnsRecordAValue(ctx context.Context, values types.List) (recordValue string, hasARecord bool,
	diags diag.Diagnostics) {
	if values.IsNull() || values.IsUnknown() {
		return "", false, diags
	}
	var blocks []lbmDnsRecordValueBlock
	diags.Append(values.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() {
		return "", false, diags
	}
	for _, block := range blocks {
		if block.RecordType == "A" {
			return block.RecordValue, true, diags
		}
	}
	return "", false, diags
}

func (r *netConnectM1ToM3Resource) cleanupStaleM1ToM3Resources(ctx context.Context, stale m1ToM3StaleResources,
	resp *resource.UpdateResponse) {
	var warnings []string
	if stale.dnsRecordId != "" {
		if err := r.lbmDnsService.DeleteIntranetDnsDomain(ctx, stale.dnsRecordId); err != nil {
			warnings = append(warnings, fmt.Sprintf("delete stale lbm-dns record %s failed: %s", stale.dnsRecordId,
				err.Error()))
		}
	}
	if stale.endpointId != "" {
		if err := r.m1PlusVpcepService.Delete(ctx, stale.endpointId); err != nil {
			warnings = append(warnings, fmt.Sprintf("delete stale vpcep-endpoint %s failed: %s", stale.endpointId,
				err.Error()))
		}
	}
	if len(warnings) > 0 {
		resp.Diagnostics.AddWarning("stale resources cleanup failed", strings.Join(warnings, "\n"))
	}
}

func (r *netConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Delete started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var deleteErr error
	if !state.LbmDnsRecordId.IsNull() {
		if err := r.lbmDnsService.DeleteIntranetDnsDomain(ctx, state.LbmDnsRecordId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete lbm-dns record %s, the vpcep endpoint and service remain intact: %w",
				state.LbmDnsRecordId.ValueString(), err)
		} else {
			state.LbmDnsRecordId = types.StringNull()
			state.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
		}
	}

	if !state.VpcepEndpointId.IsNull() && deleteErr == nil {
		if err := r.m1PlusVpcepService.Delete(ctx, state.VpcepEndpointId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete vpcep-endpoint %s, the vpcep service remains intact: %w",
				state.VpcepEndpointId.ValueString(), err)
		} else {
			state.VpcepEndpointId = types.StringNull()
			state.VpcepEndpointIp = types.StringNull()
			state.VpcepEndpointServiceId = types.StringNull()
		}
	}

	if !state.VpcepServiceId.IsNull() && deleteErr == nil {
		if err := r.m3VpcepService.Delete(ctx, state.VpcepServiceId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("delete vpcep-service %s failed: %w", state.VpcepServiceId.ValueString(), err)
		} else {
			state.VpcepServiceId = types.StringNull()
		}
	}

	if deleteErr != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		resp.Diagnostics.AddError("delete m1-to-m3 network connection failed", deleteErr.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

// convertPorts 将端口块转换为服务层的 PortPair
func convertPorts(ports []vpcepServicePortBlock) []service.PortPair {
	result := make([]service.PortPair, len(ports))
	for i, p := range ports {
		result[i] = service.PortPair{ClientPort: p.ClientPort, ServerPort: p.ServerPort}
	}
	return result
}

// convertPermissions 将权限块转换为服务层的 PermissionInput
func convertPermissions(perms []vpcepServicePermissionBlock) []service.PermissionInput {
	result := make([]service.PermissionInput, len(perms))
	for i, p := range perms {
		result[i] = service.PermissionInput{Permission: p.Permission}
	}
	return result
}
