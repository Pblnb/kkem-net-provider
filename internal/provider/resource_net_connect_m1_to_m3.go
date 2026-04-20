/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

var lbmDnsRecordValueAttrTypes = map[string]attr.Type{
	"record_type":  types.StringType,
	"record_value": types.StringType,
}

var lbmDnsRecordValueObjectType = types.ObjectType{
	AttrTypes: lbmDnsRecordValueAttrTypes,
}

const (
	vpcepEndpointPollingInterval = 5 * time.Second
	vpcepEndpointPollingTimeout  = 5 * time.Minute
	vpcepServicePollingInterval  = 5 * time.Second
	vpcepServicePollingTimeout   = 5 * time.Minute

	lbmDnsPollingInterval = 3 * time.Second
	lbmDnsPollingTimeout  = 2 * time.Minute

	pollingErrTolerance = 3
)

type netConnectM1ToM3Resource struct {
	m1PlusVpcepClient *vpcep.VpcepClient
	m3VpcepClient     *vpcep.VpcepClient
	m3LbmDnsClient    *lbmdnsclient.Client
}

type netConnectM1ToM3Model struct {
	M3VpcId                   string                        `tfsdk:"m3_vpc_id"`
	M3ServerType              string                        `tfsdk:"m3_server_type"`
	M3PortId                  string                        `tfsdk:"m3_port_id"`
	M3VpcepServicePorts       []vpcepServicePortBlock       `tfsdk:"m3_vpcep_service_ports"`
	M3VpcepServicePermissions []vpcepServicePermissionBlock `tfsdk:"m3_vpcep_service_permissions"`
	M1PlusVpcId               string                        `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId            string                        `tfsdk:"m1_plus_subnet_id"`
	DnsDomain                 string                        `tfsdk:"dns_domain"`
	DnsDomainSuffix           string                        `tfsdk:"dns_domain_suffix"`
	LbmDnsServiceName         string                        `tfsdk:"lbm_dns_service_name"`
	RegionCode                string                        `tfsdk:"region_code"`
	VpcepServiceId            types.String                  `tfsdk:"vpcep_service_id"`
	VpcepEndpointId           types.String                  `tfsdk:"vpcep_endpoint_id"`
	LbmDnsRecordId            types.String                  `tfsdk:"lbm_dns_record_id"`
	LbmDnsRecordValues        types.List                    `tfsdk:"lbm_dns_record_values"`
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

			"m3_vpcep_service_ports": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client_port": schema.Int32Attribute{Required: true},
						"server_port": schema.Int32Attribute{Required: true},
					},
				},
			},
			"m3_vpcep_service_permissions": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"permission": schema.StringAttribute{Required: true},
					},
				},
			},

			"m1_plus_vpc_id":       schema.StringAttribute{Required: true},
			"m1_plus_subnet_id":    schema.StringAttribute{Required: true},
			"dns_domain":           schema.StringAttribute{Required: true},
			"dns_domain_suffix":    schema.StringAttribute{Required: true},
			"lbm_dns_service_name": schema.StringAttribute{Required: true},
			"region_code":          schema.StringAttribute{Required: true},
			"vpcep_service_id":     schema.StringAttribute{Computed: true},
			"vpcep_endpoint_id":    schema.StringAttribute{Computed: true},
			"lbm_dns_record_id":    schema.StringAttribute{Computed: true},
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
	if err := r.addVpcepServicePermissions(ctx, vpcepServiceId, plan.M3VpcepServicePermissions); err != nil {
		resp.Diagnostics.AddError("add vpcep-service permission failed", err.Error())
		return
	}
	tflog.Info(ctx, "Step 2 completed: vpcep-service permission added", map[string]any{
		"service_id":  vpcepServiceId,
		"permissions": len(plan.M3VpcepServicePermissions),
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
	lbmDnsRecordValues, recordValueDiags := buildLbmDnsRecordValues([]lbmDnsRecordValueBlock{
		{
			RecordType:  "A",
			RecordValue: clientIp,
		},
	})
	resp.Diagnostics.Append(recordValueDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.LbmDnsRecordValues = lbmDnsRecordValues
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
	normalizeM1ToM3ListState(&plan)
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

	errCount := 0
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
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return "", fmt.Errorf("query vpcep-endpoint status failed: %w", err)
				}
				tflog.Warn(ctx, "query vpcep-endpoint failed, will retry", map[string]any{
					"client_id": clientId,
					"error":     err.Error(),
					"err_count": errCount,
				})
				continue
			}
			errCount = 0

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

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for DNS record: %s", ctx.Err())
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for DNS record creation task: %s", taskId)
		case <-ticker.C:
			resp, err := r.m3LbmDnsClient.GetIntranetDnsDomainTaskStatus(ctx, taskId)
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return "", fmt.Errorf("query lbm-dns task status failed: %w", err)
				}
				tflog.Warn(ctx, "query lbm-dns task failed, will retry", map[string]any{
					"task_id":   taskId,
					"error":     err.Error(),
					"err_count": errCount,
				})
				continue
			}
			errCount = 0

			if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
				tflog.Warn(ctx, "query lbm-dns task status failed (http error), retrying",
					map[string]any{"task_id": taskId, "http_status": resp.HTTPStatusCode, "response_body": resp.Body})
				return "", fmt.Errorf("query lbm-dns task status failed, http status is %d", resp.HTTPStatusCode)
			}

			if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
				return "", fmt.Errorf("query task status failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
					resp.Body.Code, resp.Body.ErrMsg)
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
			if utils.IsHuaweiCloudNotFoundError(innerErr) {
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
			if utils.IsHuaweiCloudNotFoundError(innerErr) {
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
				resp.Body.Code, resp.Body.ErrMsg)
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

	errCount := 0
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for vpcep-service %s to be ready", serviceId)
		case <-ticker.C:
			getReq := &model.ListServiceDetailsRequest{
				VpcEndpointServiceId: serviceId,
			}

			getResp, err := r.m3VpcepClient.ListServiceDetails(getReq)
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return fmt.Errorf("query vpcep-service status failed: %w", err)
				}
				tflog.Warn(ctx, "query vpcep-service failed, will retry", map[string]any{
					"service_id": serviceId,
					"error":      err.Error(),
					"err_count":  errCount,
				})
				continue
			}
			errCount = 0

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

// addVpcepServicePermissions 为 vpcep-service 添加白名单权限。
func (r *netConnectM1ToM3Resource) addVpcepServicePermissions(ctx context.Context, serviceId string,
	permissions []vpcepServicePermissionBlock) error {
	addPermissions := make([]model.EpsAddPermissionRequest, len(permissions))
	for i := range permissions {
		addPermissions[i] = model.EpsAddPermissionRequest{
			Permission:  permissions[i].Permission,
			Description: "Allow access from configured permission",
		}
	}
	req := &model.BatchAddEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchAddEndpointServicePermissionsRequestBody{
			Permissions: addPermissions,
		},
	}

	tflog.Debug(ctx, "Adding vpcep-service permission", map[string]any{
		"service_id":  serviceId,
		"permissions": len(permissions),
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
		"service_id":  serviceId,
		"permissions": len(permissions),
	})
	return nil
}

// Read 逻辑（Partial Repair 策略）：
// - 查询所有子资源的存在性并回填远端真实属性（VpcepService、VpcepEndpoint、LbmDnsRecord）
// - 若某个子资源返回 404，则仅将其 Computed ID 字段置为 null（不调用 RemoveResource）
// - Input 属性仅在远端返回真实值且语义一致时回填，供 Plan 阶段检测属性漂移
// - 若所有子资源均不存在，则调用 RemoveResource
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
		serviceResp, err := r.m3VpcepClient.ListServiceDetails(getReq)
		if err != nil {
			if utils.IsHuaweiCloudNotFoundError(err) {
				tflog.Info(ctx, "Vpcep-service not found, marking as null (Partial Repair)", map[string]any{
					"service_id": state.VpcepServiceId.ValueString(),
				})
				state.VpcepServiceId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("query vpcep-service failed", err.Error())
				return
			}
		} else {
			syncVpcepServiceState(&state, serviceResp)
			if err := r.syncVpcepServicePermissionState(ctx, &state, state.VpcepServiceId.ValueString()); err != nil {
				resp.Diagnostics.AddError("query vpcep-service permission failed", err.Error())
				return
			}
		}
	}

	// 验证 vpcep-endpoint 是否仍存在
	if !state.VpcepEndpointId.IsNull() {
		getReq := &model.ListEndpointInfoDetailsRequest{
			VpcEndpointId: state.VpcepEndpointId.ValueString(),
		}
		endpointResp, err := r.m1PlusVpcepClient.ListEndpointInfoDetails(getReq)
		if err != nil {
			if utils.IsHuaweiCloudNotFoundError(err) {
				tflog.Info(ctx, "Vpcep-endpoint not found, marking as null (Partial Repair)", map[string]any{
					"endpoint_id": state.VpcepEndpointId.ValueString(),
				})
				state.VpcepEndpointId = types.StringNull()
			} else {
				resp.Diagnostics.AddError("query vpcep-endpoint failed", err.Error())
				return
			}
		} else {
			syncVpcepEndpointState(&state, endpointResp)
		}
	}

	// 验证 lbm-dns 记录是否仍存在
	if !state.LbmDnsRecordId.IsNull() {
		dnsResp, err := r.m3LbmDnsClient.GetIntranetDnsDomain(ctx, state.LbmDnsRecordId.ValueString())
		tflog.Debug(ctx, "Receive lbm dns query response", map[string]any{
			"response": dnsResp,
		})
		if err != nil {
			resp.Diagnostics.AddError("query lbm-dns record failed", err.Error())
			return
		} else if dnsResp.HTTPStatusCode < 200 || dnsResp.HTTPStatusCode >= 300 {
			resp.Diagnostics.AddError("query lbm-dns record failed",
				fmt.Sprintf("http status is %d", dnsResp.HTTPStatusCode))
			return
		} else if lbmdnsclient.IsIntranetDnsDomainNotFound(dnsResp) {
			tflog.Info(ctx, "lbm-dns record not found, marking as null (Partial Repair)", map[string]any{
				"dns_record_id": state.LbmDnsRecordId.ValueString(),
				"status":        dnsResp.Body.Status,
				"code":          dnsResp.Body.Code,
				"msg":           dnsResp.Body.ErrMsg,
			})
			state.LbmDnsRecordId = types.StringNull()
			state.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
		} else {
			resp.Diagnostics.Append(syncLbmDnsState(&state, dnsResp)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	// 全部子资源均不存在时，移除整个 resource
	allRemoved := state.VpcepServiceId.IsNull() && state.VpcepEndpointId.IsNull() && state.LbmDnsRecordId.IsNull()
	if allRemoved {
		tflog.Info(ctx, "All sub-resources not found, removing resource from state")
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// syncVpcepServiceState 将 VPCEP-Service 远端真实属性同步到 Terraform state。
func syncVpcepServiceState(state *netConnectM1ToM3Model, serviceResp *model.ListServiceDetailsResponse) {
	if serviceResp.Id != nil {
		state.VpcepServiceId = types.StringValue(*serviceResp.Id)
	}
	if serviceResp.VpcId != nil {
		state.M3VpcId = *serviceResp.VpcId
	}
	if serviceResp.PortId != nil {
		state.M3PortId = *serviceResp.PortId
	}
	if serviceResp.ServerType != nil {
		state.M3ServerType = *serviceResp.ServerType
	}
	if serviceResp.Ports != nil {
		state.M3VpcepServicePorts = normalizeVpcepServicePortsFromRemote(*serviceResp.Ports)
	}
}

// normalizeM1ToM3ListState 稳定化 Create 首次写入 state 的列表顺序。
func normalizeM1ToM3ListState(state *netConnectM1ToM3Model) {
	state.M3VpcepServicePorts = normalizeVpcepServicePortBlocks(state.M3VpcepServicePorts)
	state.M3VpcepServicePermissions = normalizeVpcepServicePermissionBlocks(state.M3VpcepServicePermissions)
}

// normalizeVpcepServicePortsFromRemote 提取 TCP 端口并稳定排序，避免远端返回顺序造成无意义 diff。
func normalizeVpcepServicePortsFromRemote(ports []model.PortList) []vpcepServicePortBlock {
	tcpPorts := make([]vpcepServicePortBlock, 0, len(ports))
	for _, port := range ports {
		if port.ClientPort == nil || port.ServerPort == nil || port.Protocol == nil ||
			port.Protocol.Value() != "TCP" {
			continue
		}
		tcpPorts = append(tcpPorts, vpcepServicePortBlock{
			ClientPort: *port.ClientPort,
			ServerPort: *port.ServerPort,
		})
	}
	return normalizeVpcepServicePortBlocks(tcpPorts)
}

// normalizeVpcepServicePortBlocks 稳定化 VPCEP-Service 端口列表顺序。
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

// syncVpcepEndpointState 将 VPCEP-Endpoint 远端真实属性同步到 Terraform state。
func syncVpcepEndpointState(state *netConnectM1ToM3Model, endpointResp *model.ListEndpointInfoDetailsResponse) {
	if endpointResp.Id != nil {
		state.VpcepEndpointId = types.StringValue(*endpointResp.Id)
	}
	if endpointResp.VpcId != nil {
		state.M1PlusVpcId = *endpointResp.VpcId
	}
	if endpointResp.SubnetId != nil {
		state.M1PlusSubnetId = *endpointResp.SubnetId
	}
}

// syncLbmDnsState 将 lbm-dns 查询结果同步到 Terraform state。
func syncLbmDnsState(state *netConnectM1ToM3Model,
	dnsResp *lbmdnsclient.GetIntranetDnsDomainResponse) diag.Diagnostics {
	if dnsResp == nil || dnsResp.Body.Data == nil {
		return nil
	}

	detail := dnsResp.Body.Data
	state.RegionCode = detail.RegionCode
	state.LbmDnsServiceName = detail.ServiceName
	state.DnsDomain = detail.HostRecord
	state.DnsDomainSuffix = detail.DomainSuffix
	recordValues, diags := buildLbmDnsRecordValues(normalizeLbmDnsRecordValuesFromRemote(detail.RecordValues))
	state.LbmDnsRecordValues = recordValues
	return diags
}

// normalizeLbmDnsRecordValuesFromRemote 将远端 lbm-dns 记录值转换为 state 结构并稳定排序。
func normalizeLbmDnsRecordValuesFromRemote(values []lbmdnsclient.IntranetDnsRecordValue) []lbmDnsRecordValueBlock {
	recordValues := make([]lbmDnsRecordValueBlock, 0, len(values))
	for _, value := range values {
		recordValues = append(recordValues, lbmDnsRecordValueBlock{
			RecordType:  value.RecordType,
			RecordValue: value.RecordValue,
		})
	}
	return normalizeLbmDnsRecordValueBlocks(recordValues)
}

// buildLbmDnsRecordValues 将 lbm-dns 记录值转换为 Terraform list value。
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

// normalizeLbmDnsRecordValueBlocks 稳定化 lbm-dns 记录值顺序，避免无意义 diff。
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

// syncVpcepServicePermissionState 查询并同步 VPCEP-Service 的权限列表。
func (r *netConnectM1ToM3Resource) syncVpcepServicePermissionState(ctx context.Context, state *netConnectM1ToM3Model,
	serviceId string) error {
	limit := int32(500)
	getReq := &model.ListServicePermissionsDetailsRequest{
		VpcEndpointServiceId: serviceId,
		Limit:                &limit,
	}

	getResp, err := r.m3VpcepClient.ListServicePermissionsDetails(getReq)
	if err != nil {
		return err
	}
	state.M3VpcepServicePermissions = normalizeVpcepServicePermissionsFromRemote(getResp.Permissions)
	tflog.Debug(ctx, "Vpcep-service permissions synced", map[string]any{
		"service_id":  serviceId,
		"permissions": len(state.M3VpcepServicePermissions),
	})
	return nil
}

// normalizeVpcepServicePermissionsFromRemote 提取远端权限字符串并稳定排序。
func normalizeVpcepServicePermissionsFromRemote(permissions *[]model.PermissionObject) []vpcepServicePermissionBlock {
	if permissions == nil {
		return nil
	}
	permissionBlocks := make([]vpcepServicePermissionBlock, 0, len(*permissions))
	for _, permission := range *permissions {
		if permission.Permission == nil {
			continue
		}
		permissionBlocks = append(permissionBlocks, vpcepServicePermissionBlock{
			Permission: *permission.Permission,
		})
	}
	return normalizeVpcepServicePermissionBlocks(permissionBlocks)
}

// normalizeVpcepServicePermissionBlocks 稳定化 VPCEP-Service 权限列表顺序。
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
