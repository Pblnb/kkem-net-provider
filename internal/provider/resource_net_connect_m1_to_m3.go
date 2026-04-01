package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/chnsz/golangsdk"
	"github.com/chnsz/golangsdk/openstack/vpcep/v1/endpoints"
	"github.com/chnsz/golangsdk/openstack/vpcep/v1/services"
)

// NetConnectM1ToM3Resource M1→M3 网络打通 Resource。
type NetConnectM1ToM3Resource struct {
	VpcepClient *golangsdk.ServiceClient
	DnsClient   *golangsdk.ServiceClient
}

// NetConnectM1ToM3Model M1→M3 网络打通 Resource 的数据模型。
type NetConnectM1ToM3Model struct {
	// Required 字段
	M3VpcId            types.String       `tfsdk:"m3_vpc_id"`
	M3BackendType      types.String       `tfsdk:"m3_backend_type"`
	M3BackendId        types.String       `tfsdk:"m3_backend_id"`
	M3VpcepServerPorts []VpcepPortBlock   `tfsdk:"m3_vpcep_server_ports"`
	M1VpcId            types.String       `tfsdk:"m1_vpc_id"`
	M1SubnetId         types.String       `tfsdk:"m1_subnet_id"`
	DnsApplicant       types.String       `tfsdk:"dns_applicant"`
	DnsDomain          types.String       `tfsdk:"dns_domain"`
	DnsDomainSuffix    types.String       `tfsdk:"dns_domain_suffix"`
	// Computed 字段
	VpcepServerId types.String `tfsdk:"vpcep_server_id"`
	VpcepClientId types.String `tfsdk:"vpcep_client_id"`
	VpcepClientIp types.String `tfsdk:"vpcep_client_ip"`
}

// VpcepPortBlock 端口映射嵌套块。
type VpcepPortBlock struct {
	ClientPort types.String `tfsdk:"client_port"`
	ServerPort types.String `tfsdk:"server_port"`
	Protocol   types.String `tfsdk:"protocol"`
}

// NewNetConnectM1ToM3Resource 创建 M1→M3 网络打通 Resource 实例。
func NewNetConnectM1ToM3Resource() resource.Resource {
	return &NetConnectM1ToM3Resource{}
}

// Metadata 返回 Resource 元信息。
func (r *NetConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_net_connect_m1_to_m3"
}

// Schema 定义 M1→M3 网络打通 Resource 的 Schema。
func (r *NetConnectM1ToM3Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required 字段
			"m3_vpc_id": schema.StringAttribute{
				Required: true,
			},
			"m3_backend_type": schema.StringAttribute{
				Required: true,
			},
			"m3_backend_id": schema.StringAttribute{
				Required: true,
			},
			"m3_vpcep_server_ports": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client_port": schema.StringAttribute{
							Required: true,
						},
						"server_port": schema.StringAttribute{
							Required: true,
						},
						"protocol": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"m1_vpc_id": schema.StringAttribute{
				Required: true,
			},
			"m1_subnet_id": schema.StringAttribute{
				Required: true,
			},
			"dns_applicant": schema.StringAttribute{
				Required: true,
			},
			"dns_domain": schema.StringAttribute{
				Required: true,
			},
			"dns_domain_suffix": schema.StringAttribute{
				Required: true,
			},
			// Computed 字段
			"vpcep_server_id": schema.StringAttribute{
				Computed: true,
			},
			"vpcep_client_id": schema.StringAttribute{
				Computed: true,
			},
			"vpcep_client_ip": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure 从 req.ProviderData 取出 KkemProviderData。
func (r *NetConnectM1ToM3Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*KkemProviderData)
	if !ok {
		resp.Diagnostics.AddError("Provider data 类型错误", "expected *KkemProviderData")
		return
	}

	r.VpcepClient = data.M3VpcepClient // 默认使用 M3 客户端
	r.DnsClient = data.M3DnsClient
}

// getM1VpcEpClient 获取 M1+ 侧 VPCEP 客户端。
// 注意：当前 Resource 使用 M3 的 ProviderData，M1 客户端需要单独获取。
// 这里暂时使用 M3VpcepClient 代替，后续需要修改 Configure 逻辑传入完整的 ProviderData。
func (r *NetConnectM1ToM3Resource) getM1VpcEpClient() *golangsdk.ServiceClient {
	// TODO: 实际应该从 ProviderData 获取 M1 客户端
	// 目前临时返回 M3 客户端，仅用于编译通过
	return r.VpcepClient
}

// Create 执行 M1→M3 网络打通的完整创建流程：
// Step 1 - 在 M3 侧创建 VPCEP-Server
// Step 2 - 配置 VPCEP-Server 白名单（当前跳过，approval_enabled=false）
// Step 3 - 在 M1+ 侧创建 VPCEP-Client
// Step 4 - 轮询等待 Client 状态就绪
// Step 5 - 调用内网 DNS API 创建解析记录（TODO）
func (r *NetConnectM1ToM3Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// 解析配置
	var plan NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "开始创建 M1→M3 网络打通", map[string]interface{}{
		"m3_vpc_id":      plan.M3VpcId.ValueString(),
		"m3_backend_id":  plan.M3BackendId.ValueString(),
		"m1_vpc_id":      plan.M1VpcId.ValueString(),
		"dns_applicant":   plan.DnsApplicant.ValueString(),
		"dns_domain":      plan.DnsDomain.ValueString(),
	})

	// ========== Step 1 - 在 M3 侧创建 VPCEP-Server ==========
	tflog.Info(ctx, "Step 1: 在 M3 侧创建 VPCEP-Server")
	serverId, err := r.createVpcEndpointService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("创建 VPCEP-Server 失败", err.Error())
		return
	}
	tflog.Info(ctx, "VPCEP-Server 创建成功", map[string]interface{}{
		"server_id": serverId,
	})

	// ========== Step 3 - 在 M1+ 侧创建 VPCEP-Client ==========
	tflog.Info(ctx, "Step 3: 在 M1+ 侧创建 VPCEP-Client")
	clientId, clientIp, err := r.createVpcEndpoint(ctx, serverId, &plan)
	if err != nil {
		// 创建失败时清理已创建的 Server
		tflog.Warn(ctx, "创建 VPCEP-Client 失败，删除 VPCEP-Server", map[string]interface{}{
			"server_id": serverId,
		})
		_ = r.deleteVpcEndpointService(ctx, serverId)
		resp.Diagnostics.AddError("创建 VPCEP-Client 失败", err.Error())
		return
	}
	tflog.Info(ctx, "VPCEP-Client 创建成功", map[string]interface{}{
		"client_id": clientId,
		"client_ip": clientIp,
	})

	// ========== Step 4 - 轮询等待 Client 状态就绪 ==========
	tflog.Info(ctx, "Step 4: 轮询等待 VPCEP-Client 状态就绪")
	m1Client := r.getM1VpcEpClient()
	if err := r.waitForVpcEndpointReady(ctx, m1Client, clientId); err != nil {
		resp.Diagnostics.AddError("等待 VPCEP-Client 就绪超时", err.Error())
		return
	}
	tflog.Info(ctx, "VPCEP-Client 状态就绪")

	// ========== Step 5 - 调用内网 DNS API 创建解析记录（TODO）==========
	tflog.Info(ctx, "Step 5: 调用内网 DNS API 创建解析记录（TODO）")
	// TODO: 实现内网 DNS API 调用
	// dnsApi.CreateRecord(dnsApplicant, dnsDomain, dnsDomainSuffix, clientIp)

	// 设置状态
	plan.VpcepServerId = types.StringValue(serverId)
	plan.VpcepClientId = types.StringValue(clientId)
	plan.VpcepClientIp = types.StringValue(clientIp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// createVpcEndpointService 在 M3 侧创建 VPCEP-Server。
func (r *NetConnectM1ToM3Resource) createVpcEndpointService(ctx context.Context, plan *NetConnectM1ToM3Model) (string, error) {
	// 构建端口映射
	var ports []services.PortOpts
	for _, port := range plan.M3VpcepServerPorts {
		ports = append(ports, services.PortOpts{
			Protocol:   port.Protocol.ValueString(),
			ClientPort: 0, // 由系统自动分配
			ServerPort: 0, // 由系统自动分配
		})
	}

	// approval_enabled=false 表示不需要审批
	approvalEnabled := false
	createOpts := services.CreateOpts{
		VpcID:      plan.M3VpcId.ValueString(),
		PortID:     plan.M3BackendId.ValueString(),
		ServerType: plan.M3BackendType.ValueString(),
		Ports:      ports,
		Approval:   &approvalEnabled,
	}

	tflog.Debug(ctx, "创建 VPCEP-Server 请求参数", map[string]interface{}{
		"vpc_id":      createOpts.VpcID,
		"port_id":     createOpts.PortID,
		"server_type": createOpts.ServerType,
	})

	result := services.Create(r.VpcepClient, createOpts)
	if result.Err != nil {
		return "", fmt.Errorf("创建 VPCEP-Server 失败: %s", result.Err.Error())
	}

	// 从结果中提取 Service 对象
	service, err := result.Extract()
	if err != nil {
		return "", fmt.Errorf("解析 VPCEP-Server 创建结果失败: %s", err.Error())
	}

	return service.ID, nil
}

// createVpcEndpoint 在 M1+ 侧创建 VPCEP-Client。
func (r *NetConnectM1ToM3Resource) createVpcEndpoint(ctx context.Context, serverId string, plan *NetConnectM1ToM3Model) (string, string, error) {
	enableDNS := false
	createOpts := endpoints.CreateOpts{
		ServiceID: serverId,
		VpcID:     plan.M1VpcId.ValueString(),
		SubnetID:  plan.M1SubnetId.ValueString(),
		EnableDNS: &enableDNS,
	}

	tflog.Debug(ctx, "创建 VPCEP-Client 请求参数", map[string]interface{}{
		"service_id": createOpts.ServiceID,
		"vpc_id":     createOpts.VpcID,
		"subnet_id":  createOpts.SubnetID,
	})

	// TODO: 使用 M1 客户端而不是 M3 客户端
	result := endpoints.Create(r.VpcepClient, createOpts)
	if result.Err != nil {
		return "", "", fmt.Errorf("创建 VPCEP-Client 失败: %s", result.Err.Error())
	}

	// 从结果中提取 Endpoint 对象
	endpoint, err := result.Extract()
	if err != nil {
		return "", "", fmt.Errorf("解析 VPCEP-Client 创建结果失败: %s", err.Error())
	}

	return endpoint.ID, endpoint.IPAddr, nil
}

// waitForVpcEndpointReady 轮询等待 VPCEP-Client 状态就绪。
func (r *NetConnectM1ToM3Resource) waitForVpcEndpointReady(ctx context.Context, client *golangsdk.ServiceClient, clientId string) error {
	const (
		pollInterval = 5 * time.Second
		maxRetries   = 60
	)

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 VPCEP-Client 就绪超时: %s", clientId)
		case <-time.After(pollInterval):
			endpoint, err := endpoints.Get(client, clientId).Extract()
			if err != nil {
				if _, ok := err.(golangsdk.ErrDefault404); ok {
					return fmt.Errorf("VPCEP-Client 不存在: %s", clientId)
				}
				tflog.Warn(ctx, "查询 VPCEP-Client 状态失败", map[string]interface{}{
					"client_id": clientId,
					"error":     err.Error(),
				})
				continue
			}

			tflog.Debug(ctx, "VPCEP-Client 当前状态", map[string]interface{}{
				"client_id": clientId,
				"status":    endpoint.Status,
			})

			// 终态: accepted 或 pendingAcceptance（取决于是否需要审批）
			if endpoint.Status == "accepted" || endpoint.Status == "pendingAcceptance" {
				return nil
			}
		}
	}

	return fmt.Errorf("等待 VPCEP-Client 就绪超时（已达最大重试次数）: %s", clientId)
}

// deleteVpcEndpointService 删除 VPCEP-Server。
func (r *NetConnectM1ToM3Resource) deleteVpcEndpointService(ctx context.Context, serverId string) error {
	result := services.Delete(r.VpcepClient, serverId)
	if result.Err != nil {
		return fmt.Errorf("删除 VPCEP-Server 失败: %s", result.Err.Error())
	}
	return nil
}

// deleteVpcEndpoint 删除 VPCEP-Client。
func (r *NetConnectM1ToM3Resource) deleteVpcEndpoint(ctx context.Context, clientId string) error {
	// TODO: 使用 M1 客户端
	result := endpoints.Delete(r.VpcepClient, clientId)
	if result.Err != nil {
		return fmt.Errorf("删除 VPCEP-Client 失败: %s", result.Err.Error())
	}
	return nil
}

// Delete 执行 M1→M3 网络打通的完整删除流程：
// Step 1 - 调用内网 DNS API 删除解析记录（TODO）
// Step 2 - 删除 M1+ 侧 VPCEP-Client
// Step 3 - 删除 M3 侧 VPCEP-Server
func (r *NetConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// 解析状态
	var state NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverId := state.VpcepServerId.ValueString()
	clientId := state.VpcepClientId.ValueString()

	tflog.Debug(ctx, "开始删除 M1→M3 网络打通", map[string]interface{}{
		"vpcep_server_id": serverId,
		"vpcep_client_id": clientId,
	})

	// ========== Step 1 - 调用内网 DNS API 删除解析记录（TODO）==========
	tflog.Info(ctx, "Step 1: 调用内网 DNS API 删除解析记录（TODO）")
	// TODO: 实现内网 DNS API 调用
	// dnsApi.DeleteRecord(dnsApplicant, dnsDomain, dnsDomainSuffix)

	// ========== Step 2 - 删除 M1+ 侧 VPCEP-Client ==========
	if clientId != "" {
		tflog.Info(ctx, "Step 2: 删除 M1+ 侧 VPCEP-Client")
		if err := r.deleteVpcEndpoint(ctx, clientId); err != nil {
			resp.Diagnostics.AddError("删除 VPCEP-Client 失败", err.Error())
			return
		}
		tflog.Info(ctx, "VPCEP-Client 删除成功")
	}

	// ========== Step 3 - 删除 M3 侧 VPCEP-Server ==========
	if serverId != "" {
		tflog.Info(ctx, "Step 3: 删除 M3 侧 VPCEP-Server")
		if err := r.deleteVpcEndpointService(ctx, serverId); err != nil {
			resp.Diagnostics.AddError("删除 VPCEP-Server 失败", err.Error())
			return
		}
		tflog.Info(ctx, "VPCEP-Server 删除成功")
	}
}

// Read 读取当前 M1→M3 网络打通状态。
func (r *NetConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// 解析状态
	var state NetConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverId := state.VpcepServerId.ValueString()
	clientId := state.VpcepClientId.ValueString()

	tflog.Debug(ctx, "读取 M1→M3 网络打通状态", map[string]interface{}{
		"vpcep_server_id": serverId,
		"vpcep_client_id": clientId,
	})

	// 查询 VPCEP-Server 状态
	if serverId != "" {
		server, err := services.Get(r.VpcepClient, serverId).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				tflog.Warn(ctx, "VPCEP-Server 不存在，标记为已删除")
				state.VpcepServerId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("查询 VPCEP-Server 状态失败", err.Error())
				return
			}
		} else {
			tflog.Debug(ctx, "VPCEP-Server 当前状态", map[string]interface{}{
				"server_id": serverId,
				"status":    server.Status,
			})
		}
	}

	// 查询 VPCEP-Client 状态
	if clientId != "" {
		// TODO: 使用 M1 客户端
		endpoint, err := endpoints.Get(r.VpcepClient, clientId).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				tflog.Warn(ctx, "VPCEP-Client 不存在，标记为已删除")
				state.VpcepClientId = types.StringNull()
				state.VpcepClientIp = types.StringNull()
			} else {
				resp.Diagnostics.AddError("查询 VPCEP-Client 状态失败", err.Error())
				return
			}
		} else {
			tflog.Debug(ctx, "VPCEP-Client 当前状态", map[string]interface{}{
				"client_id": clientId,
				"status":    endpoint.Status,
				"ip":        endpoint.IPAddr,
			})
			state.VpcepClientIp = types.StringValue(endpoint.IPAddr)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update 更新 M1→M3 网络打通（当前不支持）。
func (r *NetConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"更新操作不支持",
		"M1→M3 网络打通资源不支持更新操作，如需变更请删除后重新创建",
	)
}
