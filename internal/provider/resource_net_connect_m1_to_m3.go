/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	vpcep "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"

	"huawei.com/kkem/kkem-net-provider/internal/lbmdnsclient"
	"huawei.com/kkem/kkem-net-provider/internal/utils"
)

const resourceTypeName = "_net_connect_m1_to_m3"

const (
	vpcepEndpointPollingInterval = 5 * time.Second
	vpcepEndpointPollingTimeout  = 5 * time.Minute
	vpcepServicePollingInterval  = 5 * time.Second
	vpcepServicePollingTimeout   = 5 * time.Minute

	lbmDnsPollingInterval = 3 * time.Second
	lbmDnsPollingTimeout  = 2 * time.Minute
)

type netConnectM1ToM3Resource struct {
	m1PlusVpcepClient *vpcep.VpcepClient
	m3VpcepClient     *vpcep.VpcepClient
	m3LbmDnsClient    *lbmdnsclient.Client
}

type netConnectM1ToM3Model struct {
	M3VpcId             string                  `tfsdk:"m3_vpc_id"`
	M3ServerType        string                  `tfsdk:"m3_server_type"`
	M3PortId            string                  `tfsdk:"m3_port_id"`
	M3VpcepServicePorts []vpcepServicePortBlock `tfsdk:"m3_vpcep_service_ports"`
	M3VpcepServiceName  types.String            `tfsdk:"m3_vpcep_service_name"`
	M1PlusVpcId         string                  `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId      string                  `tfsdk:"m1_plus_subnet_id"`
	M1PlusDomainId      string                  `tfsdk:"m1_plus_domain_id"`
	DnsDomain           string                  `tfsdk:"dns_domain"`
	DnsDomainSuffix     string                  `tfsdk:"dns_domain_suffix"`
	LbmDnsServiceName   string                  `tfsdk:"lbm_dns_service_name"`
	RegionCode          string                  `tfsdk:"region_code"`
	VpcepServiceId      types.String            `tfsdk:"vpcep_service_id"`
	VpcepEndpointId     types.String            `tfsdk:"vpcep_endpoint_id"`
	VpcepEndpointIp     types.String            `tfsdk:"vpcep_endpoint_ip"`
	LbmDnsRecordId      types.String            `tfsdk:"lbm_dns_record_id"`
}

type vpcepServicePortBlock struct {
	ClientPort int32 `tfsdk:"client_port"`
	ServerPort int32 `tfsdk:"server_port"`
}

func NewNetConnectM1ToM3Resource() resource.Resource {
	return &netConnectM1ToM3Resource{}
}

func (r *netConnectM1ToM3Resource) Metadata(ctx context.Context, req resource.MetadataRequest,
	resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + resourceTypeName
}

func (r *netConnectM1ToM3Resource) Schema(ctx context.Context, req resource.SchemaRequest,
	resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"m3_vpc_id":      schema.StringAttribute{Required: true},
			"m3_server_type": schema.StringAttribute{Required: true},
			"m3_port_id":     schema.StringAttribute{Required: true},
			"m3_vpcep_service_ports": schema.ListNestedAttribute{Required: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"client_port": schema.Int32Attribute{Required: true},
					"server_port": schema.Int32Attribute{Required: true},
				}}},
			"m3_vpcep_service_name": schema.StringAttribute{Optional: true},
			"m1_plus_vpc_id":        schema.StringAttribute{Required: true},
			"m1_plus_subnet_id":     schema.StringAttribute{Required: true},
			"m1_plus_domain_id":     schema.StringAttribute{Required: true},
			"dns_domain":            schema.StringAttribute{Required: true},
			"dns_domain_suffix":     schema.StringAttribute{Required: true},
			"lbm_dns_service_name":  schema.StringAttribute{Required: true},
			"region_code":           schema.StringAttribute{Required: true},
			"vpcep_service_id":      schema.StringAttribute{Computed: true},
			"vpcep_endpoint_id":     schema.StringAttribute{Computed: true},
			"vpcep_endpoint_ip":     schema.StringAttribute{Computed: true},
			"lbm_dns_record_id":     schema.StringAttribute{Computed: true},
		},
	}
}

func (r *netConnectM1ToM3Resource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*clients)
	if !ok {
		resp.Diagnostics.AddError("configure error", "invalid provider data type")
		return
	}
	r.m1PlusVpcepClient = clients.m1PlusVpcepClient
	r.m3VpcepClient = clients.m3VpcepClient
	r.m3LbmDnsClient = clients.m3LbmDnsClient
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

	// success 标志：只有整个 Create 流程完全成功才设为 true，用于控制回滚
	success := false
	defer func() {
		if !success {
			tflog.Info(ctx, "Create failed, executing rollback", map[string]any{})
			if rollbackErrs := r.rollbackCreate(ctx, &plan); len(rollbackErrs) > 0 {
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

	// Step 1.1 - 在 M3 侧创建 vpcep-service
	vpcepServiceId, err := r.createM3VpcepService(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("create vpcep-service failed", err.Error())
		return
	}
	plan.VpcepServiceId = types.StringValue(vpcepServiceId)
	tflog.Info(ctx, "Step 1.1 completed: vpcep-service created", map[string]any{
		"service_id": vpcepServiceId,
	})

	// Step 1.2 - 轮询等待 vpcep-service 状态变为 available
	if err := r.waitForVpcepServiceReady(ctx, vpcepServiceId); err != nil {
		resp.Diagnostics.AddError("wait for vpcep-service ready failed", err.Error())
		return
	}
	tflog.Info(ctx, "Step 1.2 completed: vpcep-service is ready", map[string]any{
		"service_id": vpcepServiceId,
	})

	// Step 2 - 配置 vpcep-service 白名单
	if err := r.addVpcepServicePermission(ctx, vpcepServiceId, plan.M1PlusDomainId); err != nil {
		resp.Diagnostics.AddError("add vpcep-service permission failed", err.Error())
		return
	}
	tflog.Info(ctx, "Step 2 completed: vpcep-service permission added", map[string]any{
		"service_id": vpcepServiceId,
		"domain_id":  plan.M1PlusDomainId,
	})

	// Step 3.1 - 在 M1+ 侧创建 vpcep-endpoint
	vpcepEndpointId, err := r.createM1PlusVpcepEndpoint(ctx, &plan, vpcepServiceId)
	if err != nil {
		resp.Diagnostics.AddError("create vpcep-endpoint failed", err.Error())
		return
	}
	plan.VpcepEndpointId = types.StringValue(vpcepEndpointId)
	tflog.Info(ctx, "Step 3.1 completed: vpcep-endpoint created", map[string]any{
		"client_id": vpcepEndpointId,
	})

	// Step 3.2 - 轮询等待 Client 状态就绪
	clientIp, err := r.waitForVpcepEndpointReady(ctx, vpcepEndpointId)
	if err != nil {
		resp.Diagnostics.AddError("wait for vpcep-endpoint ready failed", err.Error())
		return
	}
	plan.VpcepEndpointIp = types.StringValue(clientIp)
	tflog.Info(ctx, "Step 3.2 completed: vpcep-endpoint is ready", map[string]any{
		"client_id": vpcepEndpointId,
		"ip":        clientIp,
	})

	// Step 4.1 - 创建 lbm-dns 解析记录
	taskId, err := r.createLbmDnsRecord(ctx, &plan, clientIp)
	if err != nil {
		resp.Diagnostics.AddError("create lbm-dns record failed", err.Error())
		return
	}

	// Step 4.2 - 轮询等待 lbm-dns 记录就绪
	dnsRecordId, err := r.waitForLbmDnsRecordReady(ctx, taskId)
	if err != nil {
		resp.Diagnostics.AddError("wait for lbm-dns record ready failed", err.Error())
		return
	}
	plan.LbmDnsRecordId = types.StringValue(dnsRecordId)
	tflog.Info(ctx, "Step 4.2 completed: lbm-dns record created", map[string]any{
		"dns_record_id": dnsRecordId,
	})

	// 全部流程成功完成
	success = true
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Create completed", map[string]any{
		"service_id":    vpcepServiceId,
		"client_id":     vpcepEndpointId,
		"client_ip":     clientIp,
		"dns_record_id": dnsRecordId,
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// getVpcepServerType 将字符串转换为 API 调用所需类型。注意：目前仅支持 VM 和 LB 两种类型，默认返回 LB。
func getVpcepServerType(serverType string) model.CreateEndpointServiceRequestBodyServerType {
	switch serverType {
	case "VM":
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().VM
	default:
		return model.GetCreateEndpointServiceRequestBodyServerTypeEnum().LB
	}
}

func (r *netConnectM1ToM3Resource) createM3VpcepService(ctx context.Context, plan *netConnectM1ToM3Model) (string,
	error) {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	ports := make([]model.PortList, len(plan.M3VpcepServicePorts))
	for i := range plan.M3VpcepServicePorts {
		ports[i] = model.PortList{
			ClientPort: &plan.M3VpcepServicePorts[i].ClientPort,
			ServerPort: &plan.M3VpcepServicePorts[i].ServerPort,
			Protocol:   &tcpProtocol,
		}
	}

	// 当前固定创建单栈 IPv4、不启用审批的 vpcep-service
	ipVersion := model.GetCreateEndpointServiceRequestBodyIpVersionEnum().IPV4
	createReq := &model.CreateEndpointServiceRequest{
		Body: &model.CreateEndpointServiceRequestBody{
			VpcId:           plan.M3VpcId,
			PortId:          plan.M3PortId,
			ServerType:      getVpcepServerType(plan.M3ServerType),
			ApprovalEnabled: utils.BoolPtr(false),
			Ports:           ports,
			IpVersion:       &ipVersion,
		},
	}
	if !plan.M3VpcepServiceName.IsNull() {
		serviceName := plan.M3VpcepServiceName.ValueString()
		createReq.Body.ServiceName = &serviceName
	}

	requestJson, err := json.Marshal(createReq.Body)
	if err != nil {
		tflog.Warn(ctx, "Failed to marshal vpcep-service request", map[string]any{
			"error": err.Error(),
		})
	}
	tflog.Debug(ctx, "Creating vpcep-service", map[string]any{
		"request": string(requestJson),
	})

	var createResp *model.CreateEndpointServiceResponse
	err = utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		createResp, innerErr = r.m3VpcepClient.CreateEndpointService(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("createEndpointService API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createEndpointService response has no ID")
	}

	tflog.Info(ctx, "Vpcep-service created", map[string]any{
		"service_id": *createResp.Id,
		"status":     *createResp.Status,
	})

	return *createResp.Id, nil
}

// createM1PlusVpcepEndpoint 在 M1+ 侧创建 vpcep-endpoint。
func (r *netConnectM1ToM3Resource) createM1PlusVpcepEndpoint(ctx context.Context, plan *netConnectM1ToM3Model,
	serviceId string) (string, error) {
	createReq := &model.CreateEndpointRequest{
		Body: &model.CreateEndpointRequestBody{
			EndpointServiceId: serviceId,
			VpcId:             plan.M1PlusVpcId,
			SubnetId:          &plan.M1PlusSubnetId,
		},
	}

	tflog.Debug(ctx, "Creating vpcep-endpoint", map[string]any{
		"endpoint_service_id": serviceId,
		"vpc_id":              plan.M1PlusVpcId,
		"subnet_id":           plan.M1PlusSubnetId,
	})

	var createResp *model.CreateEndpointResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		createResp, innerErr = r.m1PlusVpcepClient.CreateEndpoint(createReq)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateEndpoint API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return "", fmt.Errorf("createEndpoint API failed after retries: %w", err)
	}

	if createResp.Id == nil {
		return "", fmt.Errorf("createEndpoint response has no ID")
	}

	tflog.Info(ctx, "Vpcep-endpoint created", map[string]any{
		"client_id": *createResp.Id,
		"status":    *createResp.Status,
	})

	return *createResp.Id, nil
}

// waitForVpcepEndpointReady 轮询等待 vpcep-endpoint 状态变为 accepted 后返回 endpoint IP。
func (r *netConnectM1ToM3Resource) waitForVpcepEndpointReady(ctx context.Context,
	clientId string) (string, error) {
	timeout := time.After(vpcepEndpointPollingTimeout)
	ticker := time.NewTicker(vpcepEndpointPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for vpcep-endpoint %s", clientId)
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for vpcep-endpoint %s to be ready", clientId)
		case <-ticker.C:
			getReq := &model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: clientId,
			}

			getResp, err := r.m1PlusVpcepClient.ListEndpointInfoDetails(getReq)
			// cmt: 这里如果由于网络错误导致 error，那么直接会让 Create 逻辑失败。wait 方法是不是统一发请求的时候使用重试机制，若重试后仍然失败，则直接 return error
			if err != nil {
				return "", fmt.Errorf("query vpcep-endpoint status failed: %w", err)
			}

			if getResp.Status == nil {
				return "", fmt.Errorf("vpcep-endpoint response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "Vpcep-endpoint status check", map[string]any{
				"client_id": clientId,
				"status":    status,
			})

			switch status {
			case "accepted":
				if getResp.Ip == nil {
					return "", fmt.Errorf("vpcep-endpoint is accepted but has no IP")
				}
				tflog.Info(ctx, "Vpcep-endpoint is ready", map[string]any{
					"client_id": clientId,
					"ip":        *getResp.Ip,
				})
				return *getResp.Ip, nil
			case "failed", "rejected":
				return "", fmt.Errorf("vpcep-endpoint %s status is %s", clientId, status)
			case "deleting":
				return "", fmt.Errorf("vpcep-endpoint %s is being deleted", clientId)
			case "creating", "pendingAcceptance":
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "Vpcep-endpoint unknown status", map[string]any{
					"client_id": clientId,
					"status":    status,
				})
			}
		}
	}
}

// waitForLbmDnsRecordReady 轮询等待 lbm-dns 记录创建完成，返回 DNS 记录 ID。
func (r *netConnectM1ToM3Resource) waitForLbmDnsRecordReady(ctx context.Context, taskId string) (string, error) {
	timeout := time.After(lbmDnsPollingTimeout)
	ticker := time.NewTicker(lbmDnsPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for DNS record: %s", ctx.Err())
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for DNS record creation task: %s", taskId)
		case <-ticker.C:
			resp, err := r.m3LbmDnsClient.GetIntranetDnsDomainTaskStatus(ctx, taskId)
			// cmt: 这里如果由于网络错误导致 error，那么直接会让 Create 逻辑失败。wait 方法是不是统一发请求的时候使用重试机制，若重试后仍然失败，则直接 return error
			if err != nil {
				tflog.Warn(ctx, "query lbm-dns task status failed (network error), retrying",
					map[string]any{"task_id": taskId, "error": err.Error()})
				continue
			}
			if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
				tflog.Warn(ctx, "query lbm-dns task status failed (http error), retrying",
					map[string]any{"task_id": taskId, "http_status": resp.HTTPStatusCode})
				continue
			}
			if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
				return "", fmt.Errorf("query task status failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
					resp.Body.Code,
					resp.Body.ErrMsg)
			}

			status := resp.Body.Data.Status
			tflog.Debug(ctx, "DNS record creation task status check",
				map[string]any{"task_id": taskId, "status": status})

			switch status {
			case lbmdnsclient.TaskStatusSuccess:
				if resp.Body.Data.ResourceId == "" {
					return "", fmt.Errorf("task completed but no resource_id returned")
				}
				return resp.Body.Data.ResourceId, nil
			case lbmdnsclient.TaskStatusFailed:
				return "", fmt.Errorf("dns record creation task failed: %s", resp.Body.Data.Message)
			default:
				continue
			}
		}
	}
}

func (r *netConnectM1ToM3Resource) deleteM1PlusVpcepEndpoint(ctx context.Context, clientId string) error {
	deleteReq := &model.DeleteEndpointRequest{
		VpcEndpointId: clientId,
	}

	tflog.Debug(ctx, "Deleting vpcep-endpoint", map[string]any{
		"client_id": clientId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m1PlusVpcepClient.DeleteEndpoint(deleteReq)
		if innerErr != nil {
			if utils.IsNotFoundError(innerErr) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpoint API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "Vpcep-endpoint deleted", map[string]any{
		"client_id": clientId,
	})
	return nil
}

func (r *netConnectM1ToM3Resource) deleteM3VpcepService(ctx context.Context, serviceId string) error {
	deleteReq := &model.DeleteEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
	}

	tflog.Debug(ctx, "Deleting vpcep-service", map[string]any{
		"service_id": serviceId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m3VpcepClient.DeleteEndpointService(deleteReq)
		if innerErr != nil {
			if utils.IsNotFoundError(innerErr) {
				return nil
			}
			tflog.Warn(ctx, "DeleteEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	tflog.Info(ctx, "Vpcep-service deleted", map[string]any{
		"service_id": serviceId,
	})
	return nil
}

// createLbmDnsRecord 创建 lbm-dns 记录，返回 taskId。
func (r *netConnectM1ToM3Resource) createLbmDnsRecord(ctx context.Context, plan *netConnectM1ToM3Model,
	clientIp string) (string, error) {
	if r.m3LbmDnsClient == nil {
		return "", fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Creating lbm-dns record", map[string]any{
		"region_code":   plan.RegionCode,
		"service_name":  plan.LbmDnsServiceName,
		"host_record":   plan.DnsDomain,
		"domain_suffix": plan.DnsDomainSuffix,
		"client_ip":     clientIp,
	})

	var taskId string
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		resp, innerErr := r.m3LbmDnsClient.CreateIntranetDnsDomain(ctx, plan.RegionCode, plan.LbmDnsServiceName,
			plan.DnsDomain, plan.DnsDomainSuffix, clientIp)
		if innerErr != nil {
			tflog.Warn(ctx, "CreateIntranetDnsDomain API failed, retrying", map[string]any{"error": innerErr.Error()})
			return innerErr
		}
		if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
			return fmt.Errorf("create DNS record failed: httpStatusCode=%d", resp.HTTPStatusCode)
		}
		if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
			return fmt.Errorf("create DNS record failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
				resp.Body.Code,
				resp.Body.ErrMsg)
		}
		if resp.Body.TaskId == "" {
			return fmt.Errorf("create DNS record response has no task_id")
		}
		taskId = resp.Body.TaskId
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create DNS record failed after retries: %w", err)
	}

	tflog.Info(ctx, "lbm-dns record creation task started", map[string]any{"task_id": taskId})
	return taskId, nil
}

// rollbackCreate 根据 plan 中已创建的资源执行回滚清理。
// 按照依赖逆序删除：Client → Service
func (r *netConnectM1ToM3Resource) rollbackCreate(ctx context.Context, plan *netConnectM1ToM3Model) []error {
	var errs []error
	// 删除 vpcep-endpoint（如果已创建）
	if !plan.VpcepEndpointId.IsNull() {
		if err := r.deleteM1PlusVpcepEndpoint(ctx, plan.VpcepEndpointId.ValueString()); err != nil {
			errs = append(errs,
				fmt.Errorf("delete vpcep-endpoint %s failed: %w", plan.VpcepEndpointId.ValueString(), err))
		}
	}

	// 删除 vpcep-service（如果已创建）
	if !plan.VpcepServiceId.IsNull() {
		if err := r.deleteM3VpcepService(ctx, plan.VpcepServiceId.ValueString()); err != nil {
			errs = append(errs,
				fmt.Errorf("delete vpcep-service %s failed: %w", plan.VpcepServiceId.ValueString(), err))
		}
	}
	return errs
}

// waitForVpcepServiceReady 轮询等待 vpcep-service 状态变为 available。
func (r *netConnectM1ToM3Resource) waitForVpcepServiceReady(ctx context.Context, serviceId string) error {
	timeout := time.After(vpcepServicePollingTimeout)
	ticker := time.NewTicker(vpcepServicePollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for vpcep-service %s to be ready", serviceId)
		case <-ticker.C:
			getReq := &model.ListServiceDetailsRequest{
				VpcEndpointServiceId: serviceId,
			}

			getResp, err := r.m3VpcepClient.ListServiceDetails(getReq)
			// cmt: 这里如果由于网络错误导致 error，那么直接会让 Create 逻辑失败。wait 方法是不是统一发请求的时候使用重试机制，若重试后仍然失败，则直接 return error
			if err != nil {
				return fmt.Errorf("query vpcep-service status failed: %w", err)
			}

			if getResp.Status == nil {
				return fmt.Errorf("vpcep-service response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "Vpcep-service status check", map[string]any{
				"service_id": serviceId,
				"status":     status,
			})

			switch status {
			case "available":
				tflog.Info(ctx, "Vpcep-service is ready", map[string]any{
					"service_id": serviceId,
				})
				return nil
			case "failed":
				return fmt.Errorf("vpcep-service %s status is failed", serviceId)
			case "creating":
				// 继续轮询
				continue
			default:
				// 未知状态，继续轮询
				tflog.Warn(ctx, "Vpcep-service unknown status", map[string]any{
					"service_id": serviceId,
					"status":     status,
				})
			}
		}
	}
}

// addVpcepServicePermission 为 vpcep-service 添加白名单权限。
func (r *netConnectM1ToM3Resource) addVpcepServicePermission(ctx context.Context, serviceId, domainId string) error {
	permission := fmt.Sprintf("iam:domain::%s", domainId)

	req := &model.BatchAddEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchAddEndpointServicePermissionsRequestBody{
			Permissions: []model.EpsAddPermissionRequest{
				{
					Permission:  permission,
					Description: "Allow access from M1+ domain",
				},
			},
		},
	}

	tflog.Debug(ctx, "Adding vpcep-service permission", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m3VpcepClient.BatchAddEndpointServicePermissions(req)
		if innerErr != nil {
			tflog.Warn(ctx, "BatchAddEndpointServicePermissions API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("batchAddEndpointServicePermissions API failed after retries: %w", err)
	}

	tflog.Info(ctx, "Vpcep-service permission added", map[string]any{
		"service_id": serviceId,
		"domain_id":  domainId,
	})
	return nil
}

// Read 逻辑：
// - VpcepServiceId 有值 → 查询 API 验证存在性，404 → null
// - VpcepServiceId 为 null → 直接信任 state
// - 全部子资源均为 null → RemoveResource
func (r *netConnectM1ToM3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Read started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 验证 vpcep-service 是否仍存在
	if !state.VpcepServiceId.IsNull() {
		getReq := &model.ListServiceDetailsRequest{VpcEndpointServiceId: state.VpcepServiceId.ValueString()}
		_, err := r.m3VpcepClient.ListServiceDetails(getReq)
		if err != nil {
			if utils.IsNotFoundError(err) {
				tflog.Info(ctx, "Vpcep-service not found, marking as null", map[string]any{
					"service_id": state.VpcepServiceId.ValueString(),
				})
				state.VpcepServiceId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("query vpcep-service failed", err.Error())
				return
			}
		}
	}

	// TODO: 验证 vpcep-endpoint 是否仍存在（当VpcepEndpointId有值时）
	// TODO: 验证 lbm-dns 记录是否仍存在（当LbmDnsRecordId有值时）

	// 全部子资源均不存在时，移除整个 resource
	allRemoved := state.VpcepServiceId.IsNull() && state.VpcepEndpointId.IsNull() &&
		state.VpcepEndpointIp.IsNull() && state.LbmDnsRecordId.IsNull()
	if allRemoved {
		tflog.Info(ctx, "All sub-resources not found, removing resource from state")
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *netConnectM1ToM3Resource) Update(ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Update called")
	var plan netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *netConnectM1ToM3Resource) Delete(ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse) {
	tflog.Info(ctx, "KKEM_net_connect_m1_to_m3: Delete started")
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 删除顺序：DNS → vpcep-endpoint → vpcep-service
	// Step 1 - 删除 lbm-dns 解析记录（TODO）
	// Step 2 - 删除 M1+ 侧 vpcep-endpoint（TODO）
	// Step 3 - 删除 M3 侧 vpcep-service
	if !state.VpcepServiceId.IsNull() {
		if err := r.deleteM3VpcepService(ctx, state.VpcepServiceId.ValueString()); err != nil {
			resp.Diagnostics.AddError("delete vpcep-service failed", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}
