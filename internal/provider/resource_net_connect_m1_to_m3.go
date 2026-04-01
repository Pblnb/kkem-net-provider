package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
)

// NetConnectM1ToM3Resource M1→M3 网络打通 Resource。
type NetConnectM1ToM3Resource struct {
	VpcepClient *vpcep.VpcepClient
	DnsClient   interface{}
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

// stringToPortListProtocol 将字符串转换为 PortListProtocol 枚举。
func stringToPortListProtocol(s string) (*model.PortListProtocol, error) {
	var p model.PortListProtocol
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(jsonBytes, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// stringToServerType 将字符串转换为 CreateEndpointServiceRequestBodyServerType 枚举。
func stringToServerType(s string) (model.CreateEndpointServiceRequestBodyServerType, error) {
	var st model.CreateEndpointServiceRequestBodyServerType
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(jsonBytes, &st); err != nil {
		return st, err
	}
	return st, nil
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
// TODO: 实际应该从 ProviderData 获取 M1 客户端
func (r *NetConnectM1ToM3Resource) getM1VpcEpClient() *vpcep.VpcepClient {
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
	var ports []model.PortList
	for _, port := range plan.M3VpcepServerPorts {
		var clientPort, serverPort int32
		fmt.Sscanf(port.ClientPort.ValueString(), "%d", &clientPort)
		fmt.Sscanf(port.ServerPort.ValueString(), "%d", &serverPort)

		protocol, err := stringToPortListProtocol(port.Protocol.ValueString())
		if err != nil {
			return "", fmt.Errorf("转换协议类型失败: %w", err)
		}

		ports = append(ports, model.PortList{
			ClientPort: &clientPort,
			ServerPort: &serverPort,
			Protocol:   protocol,
		})
	}

	// approval_enabled=false 表示不需要审批
	approvalEnabled := false
	serverType, err := stringToServerType(plan.M3BackendType.ValueString())
	if err != nil {
		return "", fmt.Errorf("转换服务器类型失败: %w", err)
	}
	createOpts := &model.CreateEndpointServiceRequestBody{
		PortId:          plan.M3BackendId.ValueString(),
		VpcId:           plan.M3VpcId.ValueString(),
		ServerType:      serverType,
		ApprovalEnabled: &approvalEnabled,
		Ports:           ports,
	}

	tflog.Debug(ctx, "创建 VPCEP-Server 请求参数", map[string]interface{}{
		"vpc_id":      createOpts.VpcId,
		"port_id":     createOpts.PortId,
		"server_type": serverType.Value(),
	})

	request := &model.CreateEndpointServiceRequest{
		Body: createOpts,
	}

	response, err := r.VpcepClient.CreateEndpointService(request)
	if err != nil {
		return "", fmt.Errorf("创建 VPCEP-Server 失败: %w", err)
	}

	if response.Id == nil {
		return "", fmt.Errorf("创建 VPCEP-Server 成功但未返回 ID")
	}

	return *response.Id, nil
}

// createVpcEndpoint 在 M1+ 侧创建 VPCEP-Client。
func (r *NetConnectM1ToM3Resource) createVpcEndpoint(ctx context.Context, serverId string, plan *NetConnectM1ToM3Model) (string, string, error) {
	enableDNS := false
	subnetId := plan.M1SubnetId.ValueString()
	createOpts := &model.CreateEndpointRequestBody{
		EndpointServiceId: serverId,
		VpcId:              plan.M1VpcId.ValueString(),
		SubnetId:           &subnetId,
		EnableDns:          &enableDNS,
	}

	tflog.Debug(ctx, "创建 VPCEP-Client 请求参数", map[string]interface{}{
		"endpoint_service_id": createOpts.EndpointServiceId,
		"vpc_id":             createOpts.VpcId,
		"subnet_id":          createOpts.SubnetId,
	})

	request := &model.CreateEndpointRequest{
		Body: createOpts,
	}

	// TODO: 使用 M1 客户端
	response, err := r.VpcepClient.CreateEndpoint(request)
	if err != nil {
		return "", "", fmt.Errorf("创建 VPCEP-Client 失败: %w", err)
	}

	if response.Id == nil {
		return "", "", fmt.Errorf("创建 VPCEP-Client 成功但未返回 ID")
	}

	return *response.Id, "", nil
}

// waitForVpcEndpointReady 轮询等待 VPCEP-Client 状态就绪。
func (r *NetConnectM1ToM3Resource) waitForVpcEndpointReady(ctx context.Context, client *vpcep.VpcepClient, clientId string) error {
	const (
		pollInterval = 5 * time.Second
		maxRetries   = 60
	)

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 VPCEP-Client 就绪超时: %s", clientId)
		case <-time.After(pollInterval):
			request := &model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: clientId,
			}
			response, err := client.ListEndpointInfoDetails(request)
			if err != nil {
				tflog.Warn(ctx, "查询 VPCEP-Client 状态失败", map[string]interface{}{
					"client_id": clientId,
					"error":     err.Error(),
				})
				continue
			}

			tflog.Debug(ctx, "VPCEP-Client 当前状态", map[string]interface{}{
				"client_id": clientId,
				"status":    response.Status,
			})

			// 终态: accepted 或 pendingAcceptance（取决于是否需要审批）
			if response.Status != nil && (response.Status.Value() == "accepted" || response.Status.Value() == "pendingAcceptance") {
				return nil
			}
		}
	}

	return fmt.Errorf("等待 VPCEP-Client 就绪超时（已达最大重试次数）: %s", clientId)
}

// deleteVpcEndpointService 删除 VPCEP-Server。
func (r *NetConnectM1ToM3Resource) deleteVpcEndpointService(ctx context.Context, serverId string) error {
	request := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: serverId,
	}
	_, err := r.VpcepClient.DeleteEndpointService(request)
	if err != nil {
		return fmt.Errorf("删除 VPCEP-Server 失败: %w", err)
	}
	return nil
}

// deleteVpcEndpoint 删除 VPCEP-Client。
func (r *NetConnectM1ToM3Resource) deleteVpcEndpoint(ctx context.Context, clientId string) error {
	// TODO: 使用 M1 客户端
	request := &model.DeleteEndpointRequest{
		VpcEndpointId: clientId,
	}
	_, err := r.VpcepClient.DeleteEndpoint(request)
	if err != nil {
		return fmt.Errorf("删除 VPCEP-Client 失败: %w", err)
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
		request := &model.ListServiceDetailsRequest{
			VpcEndpointServiceId: serverId,
		}
		response, err := r.VpcepClient.ListServiceDetails(request)
		if err != nil {
			// 如果服务不存在，认为已被删除
			tflog.Warn(ctx, "VPCEP-Server 不存在，标记为已删除", map[string]interface{}{
				"server_id": serverId,
			})
			state.VpcepServerId = types.StringNull()
		} else {
			tflog.Debug(ctx, "VPCEP-Server 当前状态", map[string]interface{}{
				"server_id": serverId,
				"status":    response.Status,
			})
		}
	}

	// 查询 VPCEP-Client 状态
	if clientId != "" {
		request := &model.ListEndpointInfoDetailsRequest{
			VpcEndpointId: clientId,
		}
		response, err := r.VpcepClient.ListEndpointInfoDetails(request)
		if err != nil {
			// 如果客户端不存在，认为已被删除
			tflog.Warn(ctx, "VPCEP-Client 不存在，标记为已删除", map[string]interface{}{
				"client_id": clientId,
			})
			state.VpcepClientId = types.StringNull()
			state.VpcepClientIp = types.StringNull()
		} else {
			tflog.Debug(ctx, "VPCEP-Client 当前状态", map[string]interface{}{
				"client_id": clientId,
				"status":    response.Status,
				"ip":        response.Ip,
			})
			if response.Ip != nil {
				state.VpcepClientIp = types.StringValue(*response.Ip)
			}
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
