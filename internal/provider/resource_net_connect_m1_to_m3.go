/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	lbmDnsClient      *lbmdnsclient.Client
}

type netConnectM1ToM3Model struct {
	// M3 Parameters
	M3VpcId                   string                        `tfsdk:"m3_vpc_id"`
	M3ServerType              string                        `tfsdk:"m3_server_type"`
	M3PortId                  string                        `tfsdk:"m3_port_id"`
	M3VpcepServicePorts       []vpcepServicePortBlock       `tfsdk:"m3_vpcep_service_ports"`
	M3VpcepServicePermissions []vpcepServicePermissionBlock `tfsdk:"m3_vpcep_service_permissions"`

	// M1+ Parameters
	M1PlusVpcId    string `tfsdk:"m1_plus_vpc_id"`
	M1PlusSubnetId string `tfsdk:"m1_plus_subnet_id"`

	// LBM DNS Parameters
	DnsDomain         string `tfsdk:"dns_domain"`
	DnsDomainSuffix   string `tfsdk:"dns_domain_suffix"`
	LbmDnsServiceName string `tfsdk:"lbm_dns_service_name"`
	RegionCode        string `tfsdk:"region_code"`

	// Computed Parameters
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
	serviceId   string
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
			// M3 Parameters
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

			// M1+ Parameters
			"m1_plus_vpc_id":    schema.StringAttribute{Required: true},
			"m1_plus_subnet_id": schema.StringAttribute{Required: true},

			// LBM DNS Parameters
			"dns_domain":           schema.StringAttribute{Required: true},
			"dns_domain_suffix":    schema.StringAttribute{Required: true},
			"lbm_dns_service_name": schema.StringAttribute{Required: true},
			"region_code":          schema.StringAttribute{Required: true},

			// Computed Parameters
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
	r.lbmDnsClient = clients.lbmDnsClient
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

	// 全部流程成功完成
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
	vpcepServiceId, err = r.createM3VpcepService(ctx, plan)
	if err != nil {
		return "", fmt.Errorf("create vpcep-service failed: %w", err)
	}
	plan.VpcepServiceId = types.StringValue(vpcepServiceId)
	tflog.Info(ctx, "Step 1.1 completed: vpcep-service created", map[string]any{
		"service_id": vpcepServiceId,
	})

	if err := r.waitForVpcepServiceReady(ctx, vpcepServiceId); err != nil {
		return "", fmt.Errorf("wait for vpcep-service ready failed: %w", err)
	}
	tflog.Info(ctx, "Step 1.2 completed: vpcep-service is ready", map[string]any{
		"service_id": vpcepServiceId,
	})

	if err := r.addVpcepServicePermissions(ctx, vpcepServiceId, plan.M3VpcepServicePermissions); err != nil {
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
	vpcepEndpointId, err = r.createM1PlusVpcepEndpoint(ctx, plan, vpcepServiceId)
	if err != nil {
		return "", "", fmt.Errorf("create vpcep-endpoint failed: %w", err)
	}
	plan.VpcepEndpointId = types.StringValue(vpcepEndpointId)
	plan.VpcepEndpointServiceId = types.StringValue(vpcepServiceId)
	tflog.Info(ctx, "Step 3.1 completed: vpcep-endpoint created", map[string]any{
		"endpoint_id": vpcepEndpointId,
	})

	endpointIp, err = r.waitForVpcepEndpointReady(ctx, vpcepEndpointId)
	plan.VpcepEndpointIp = types.StringValue(endpointIp)
	if err != nil {
		return "", "", fmt.Errorf("wait for vpcep-endpoint ready failed: %w", err)
	}
	tflog.Info(ctx, "Step 3.2 completed: vpcep-endpoint is ready", map[string]any{
		"endpoint_id": vpcepEndpointId,
		"ip":          endpointIp,
	})
	return vpcepEndpointId, endpointIp, nil
}

func (r *netConnectM1ToM3Resource) createAndWaitLbmDnsRecord(ctx context.Context, plan *netConnectM1ToM3Model,
	endpointIp string) (dnsRecordId string, err error) {
	taskId, err := r.createLbmDnsRecord(ctx, plan, endpointIp)
	if err != nil {
		return "", fmt.Errorf("create lbm-dns record failed: %w", err)
	}

	dnsRecordId, err = r.waitForLbmDnsRecordReady(ctx, taskId)
	if err != nil {
		return "", fmt.Errorf("wait for lbm-dns record ready failed: %w", err)
	}
	plan.LbmDnsRecordId = types.StringValue(dnsRecordId)
	if err := setLbmDnsRecordValues(plan, endpointIp); err != nil {
		return "", err
	}
	tflog.Info(ctx, "Step 4.2 completed: lbm-dns record created", map[string]any{
		"dns_record_id": dnsRecordId,
	})
	return dnsRecordId, nil
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
		"endpoint_id": *createResp.Id,
		"status":      *createResp.Status,
	})

	return *createResp.Id, nil
}

// waitForVpcepEndpointReady 轮询等待 vpcep-endpoint 状态变为 accepted 后返回 endpoint IP。
func (r *netConnectM1ToM3Resource) waitForVpcepEndpointReady(ctx context.Context,
	endpointId string) (string, error) {
	timeout := time.After(vpcepEndpointPollingTimeout)
	ticker := time.NewTicker(vpcepEndpointPollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for vpcep-endpoint %s", endpointId)
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for vpcep-endpoint %s to be ready", endpointId)
		case <-ticker.C:
			getReq := &model.ListEndpointInfoDetailsRequest{
				VpcEndpointId: endpointId,
			}

			getResp, err := r.m1PlusVpcepClient.ListEndpointInfoDetails(getReq)
			if err != nil {
				errCount++
				if errCount >= pollingErrTolerance {
					return "", fmt.Errorf("query vpcep-endpoint status failed: %w", err)
				}
				tflog.Warn(ctx, "query vpcep-endpoint failed, will retry", map[string]any{
					"endpoint_id": endpointId,
					"error":       err.Error(),
					"err_count":   errCount,
				})
				continue
			}
			errCount = 0

			if getResp.Status == nil {
				return "", fmt.Errorf("vpcep-endpoint response has no status")
			}

			status := *getResp.Status
			tflog.Debug(ctx, "Vpcep-endpoint status check", map[string]any{
				"endpoint_id": endpointId,
				"status":      status,
			})

			endpointIp, isReady, err := handleVpcepEndpointStatus(ctx, endpointId, status, getResp.Ip)
			if err != nil || isReady {
				return endpointIp, err
			}
		}
	}
}

func handleVpcepEndpointStatus(ctx context.Context, endpointId, status string,
	endpointIp *string) (readyEndpointIp string, isReady bool, err error) {
	switch status {
	case "accepted":
		if endpointIp == nil {
			return "", true, fmt.Errorf("vpcep-endpoint is accepted but has no IP")
		}
		tflog.Info(ctx, "Vpcep-endpoint is ready", map[string]any{
			"endpoint_id": endpointId,
			"ip":          *endpointIp,
		})
		return *endpointIp, true, nil
	case "failed", "rejected":
		return "", true, fmt.Errorf("vpcep-endpoint %s status is %s", endpointId, status)
	case "deleting":
		return "", true, fmt.Errorf("vpcep-endpoint %s is being deleted", endpointId)
	case "creating", "pendingAcceptance":
		return "", false, nil
	default:
		tflog.Warn(ctx, "Vpcep-endpoint unknown status", map[string]any{
			"endpoint_id": endpointId,
			"status":      status,
		})
		return "", false, nil
	}
}

// waitForLbmDnsRecordReady 轮询等待 lbm-dns 记录创建完成，返回 DNS 记录 ID。
func (r *netConnectM1ToM3Resource) waitForLbmDnsRecordReady(ctx context.Context, taskId string) (string, error) {
	resp, err := r.waitForLbmDnsTaskCompleted(ctx, taskId, "DNS record creation")
	if err != nil {
		return "", err
	}
	if resp.Body.Data.ResourceId == "" {
		return "", fmt.Errorf("task completed but no resource_id returned")
	}
	return resp.Body.Data.ResourceId, nil
}

func (r *netConnectM1ToM3Resource) waitForLbmDnsTaskCompleted(ctx context.Context,
	taskId, taskName string) (*lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, error) {
	timeout := time.After(lbmDnsPollingTimeout)
	ticker := time.NewTicker(lbmDnsPollingInterval)
	defer ticker.Stop()

	errCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for %s: %s", taskName, ctx.Err())
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for %s task: %s", taskName, taskId)
		case <-ticker.C:
			resp, shouldContinue, err := r.queryLbmDnsTaskStatus(ctx, taskId, &errCount)
			if err != nil {
				return nil, err
			}
			if shouldContinue {
				continue
			}

			status := resp.Body.Data.Status
			tflog.Debug(ctx, taskName+" task status check", map[string]any{"task_id": taskId, "status": status})

			switch status {
			case lbmdnsclient.TaskStatusSuccess:
				return resp, nil
			case lbmdnsclient.TaskStatusFailed:
				return nil, fmt.Errorf("%s task failed: %s", taskName, resp.Body.Data.Message)
			}
		}
	}
}

func (r *netConnectM1ToM3Resource) queryLbmDnsTaskStatus(ctx context.Context, taskId string,
	errCount *int) (resp *lbmdnsclient.GetIntranetDnsDomainTaskStatusResponse, shouldContinue bool, err error) {
	resp, err = r.lbmDnsClient.GetIntranetDnsDomainTaskStatus(ctx, taskId)
	if err != nil {
		*errCount++
		if *errCount >= pollingErrTolerance {
			return nil, false, fmt.Errorf("query lbm-dns task status failed: %w", err)
		}
		tflog.Warn(ctx, "query lbm-dns task failed, will retry", map[string]any{
			"task_id":   taskId,
			"error":     err.Error(),
			"err_count": *errCount,
		})
		return nil, true, nil
	}
	*errCount = 0

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		tflog.Warn(ctx, "query lbm-dns task status failed (http error)",
			map[string]any{"task_id": taskId, "http_status": resp.HTTPStatusCode, "response_body": resp.Body})
		return nil, false, fmt.Errorf("query lbm-dns task status failed, http status is %d", resp.HTTPStatusCode)
	}
	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return nil, false, fmt.Errorf("query task status failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
			resp.Body.Code, resp.Body.ErrMsg)
	}
	return resp, false, nil
}

func (r *netConnectM1ToM3Resource) deleteM1PlusVpcepEndpoint(ctx context.Context, endpointId string) error {
	deleteReq := &model.DeleteEndpointRequest{
		VpcEndpointId: endpointId,
	}

	tflog.Debug(ctx, "Deleting vpcep-endpoint", map[string]any{
		"endpoint_id": endpointId,
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m1PlusVpcepClient.DeleteEndpoint(deleteReq)
		if innerErr != nil {
			// 若响应是资源不存在则表示资源已删除，不需要重试
			if utils.IsVpcepNotFoundError(innerErr) {
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
		"endpoint_id": endpointId,
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
			// 若响应是资源不存在则表示资源已删除，不需要重试
			if utils.IsVpcepNotFoundError(innerErr) {
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

func (r *netConnectM1ToM3Resource) deleteLbmDnsRecord(ctx context.Context, recordId string) error {
	if r.lbmDnsClient == nil {
		return fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Deleting lbm-dns record", map[string]any{
		"dns_record_id": recordId,
	})

	var resp *lbmdnsclient.DeleteIntranetDnsDomainResponse
	var err error
	if resp, err = r.lbmDnsClient.DeleteIntranetDnsDomain(ctx, recordId); err != nil {
		return fmt.Errorf("call DeleteIntranetDnsDomain API failed: %w", err)
	}

	if resp == nil {
		return errors.New("response is nil")
	}

	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return fmt.Errorf("httpStatusCode=%d", resp.HTTPStatusCode)
	}

	if lbmdnsclient.IsNotFound(resp.Body.Code) {
		tflog.Info(ctx, "call DeleteIntranetDnsDomain API successfully, but record already have been deleted",
			map[string]any{
				"dns_record_id":        recordId,
				"lbm_response_status":  resp.Body.Status,
				"lbm_response_code":    resp.Body.Code,
				"lbm_response_err_msg": resp.Body.ErrMsg,
			})
		return nil
	}

	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return fmt.Errorf("response from lbm dns server contains unsuccessful code: status=%d, code=%d, errMsg=%s",
			resp.Body.Status, resp.Body.Code, resp.Body.ErrMsg)
	}

	taskId := resp.Body.TaskId
	if taskId == "" {
		return errors.New("delete lbm-dns record response has no task id")
	}

	if _, err := r.waitForLbmDnsTaskCompleted(ctx, taskId, "DNS record deletion"); err != nil {
		return err
	}
	tflog.Info(ctx, "lbm-dns record deleted", map[string]any{
		"dns_record_id": recordId,
		"task_id":       taskId,
	})
	return nil
}

// createLbmDnsRecord 创建 lbm-dns 记录，返回 taskId。
func (r *netConnectM1ToM3Resource) createLbmDnsRecord(ctx context.Context, plan *netConnectM1ToM3Model,
	endpointIp string) (string, error) {
	if r.lbmDnsClient == nil {
		return "", fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Creating lbm-dns record", map[string]any{
		"region_code":   plan.RegionCode,
		"service_name":  plan.LbmDnsServiceName,
		"host_record":   plan.DnsDomain,
		"domain_suffix": plan.DnsDomainSuffix,
		"endpoint_ip":   endpointIp,
	})

	var taskId string
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		resp, innerErr := r.lbmDnsClient.CreateIntranetDnsDomain(ctx, plan.RegionCode, plan.LbmDnsServiceName,
			plan.DnsDomain, plan.DnsDomainSuffix, endpointIp)
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

func (r *netConnectM1ToM3Resource) updateLbmDnsRecordValues(ctx context.Context, recordId, endpointIp string) error {
	if r.lbmDnsClient == nil {
		return fmt.Errorf("m3 lbm-dns client is not initialized")
	}

	tflog.Debug(ctx, "Updating lbm-dns record values", map[string]any{
		"dns_record_id": recordId,
		"endpoint_ip":   endpointIp,
	})

	resp, err := r.lbmDnsClient.UpdateIntranetDnsDomain(ctx, recordId, endpointIp)
	if err != nil {
		return fmt.Errorf("call UpdateIntranetDnsDomain API failed: %w", err)
	}
	if resp == nil {
		return errors.New("response is nil")
	}
	if resp.HTTPStatusCode < 200 || resp.HTTPStatusCode >= 300 {
		return fmt.Errorf("update DNS record failed: httpStatusCode=%d", resp.HTTPStatusCode)
	}
	if isLbmDnsNoChanges(resp.Body.Status, resp.Body.Code, resp.Body.ErrMsg) {
		tflog.Info(ctx, "lbm-dns record values already up to date", map[string]any{
			"dns_record_id": recordId,
		})
		return nil
	}
	if resp.Body.Status != lbmdnsclient.StatusCodeSuccess || resp.Body.Code != lbmdnsclient.StatusCodeSuccess {
		return fmt.Errorf("update DNS record failed: status=%d, code=%d, errMsg=%s", resp.Body.Status,
			resp.Body.Code, resp.Body.ErrMsg)
	}
	if resp.Body.TaskId == "" {
		return errors.New("update lbm-dns record response has no task id")
	}
	if _, err := r.waitForLbmDnsTaskCompleted(ctx, resp.Body.TaskId, "DNS record update"); err != nil {
		return err
	}
	tflog.Info(ctx, "lbm-dns record values updated", map[string]any{
		"dns_record_id": recordId,
		"task_id":       resp.Body.TaskId,
	})
	return nil
}

func isLbmDnsNoChanges(status, code int, msg string) bool {
	return status == lbmdnsclient.StatusCodeNoChanges && code == lbmdnsclient.StatusCodeNoChanges &&
		strings.Contains(msg, "No changes")
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
	if err := r.refreshVpcepServiceState(ctx, &state); err != nil {
		resp.Diagnostics.AddError("query vpcep-service failed", err.Error())
		return
	}

	// 验证 vpcep-endpoint 是否仍存在
	if err := r.refreshVpcepEndpointState(ctx, &state); err != nil {
		resp.Diagnostics.AddError("query vpcep-endpoint failed", err.Error())
		return
	}

	// 验证 lbm-dns 记录是否仍存在
	resp.Diagnostics.Append(r.refreshLbmDnsState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
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

func (r *netConnectM1ToM3Resource) refreshVpcepServiceState(ctx context.Context,
	state *netConnectM1ToM3Model) error {
	if state.VpcepServiceId.IsNull() {
		return nil
	}

	getReq := &model.ListServiceDetailsRequest{VpcEndpointServiceId: state.VpcepServiceId.ValueString()}
	var serviceResp *model.ListServiceDetailsResponse
	serviceNotFound := false
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		serviceResp, innerErr = r.m3VpcepClient.ListServiceDetails(getReq)
		if utils.IsVpcepNotFoundError(innerErr) {
			serviceNotFound = true
			return nil
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	if serviceNotFound {
		tflog.Info(ctx, "Vpcep-service not found, marking as null (Partial Repair)", map[string]any{
			"service_id": state.VpcepServiceId.ValueString(),
		})
		state.VpcepServiceId = types.StringNull()
		return nil
	}

	syncVpcepServiceState(state, serviceResp)
	if err := r.syncVpcepServicePermissionState(ctx, state, state.VpcepServiceId.ValueString()); err != nil {
		return fmt.Errorf("query vpcep-service permission failed: %w", err)
	}
	return nil
}

func (r *netConnectM1ToM3Resource) refreshVpcepEndpointState(ctx context.Context,
	state *netConnectM1ToM3Model) error {
	if state.VpcepEndpointId.IsNull() {
		return nil
	}

	getReq := &model.ListEndpointInfoDetailsRequest{
		VpcEndpointId: state.VpcepEndpointId.ValueString(),
	}
	var endpointResp *model.ListEndpointInfoDetailsResponse
	endpointNotFound := false
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		endpointResp, innerErr = r.m1PlusVpcepClient.ListEndpointInfoDetails(getReq)
		if utils.IsVpcepNotFoundError(innerErr) {
			endpointNotFound = true
			return nil
		}
		return innerErr
	})
	if err != nil {
		return err
	}
	if endpointNotFound {
		tflog.Info(ctx, "Vpcep-endpoint not found, marking as null (Partial Repair)", map[string]any{
			"endpoint_id": state.VpcepEndpointId.ValueString(),
		})
		state.VpcepEndpointId = types.StringNull()
		state.VpcepEndpointIp = types.StringNull()
		state.VpcepEndpointServiceId = types.StringNull()
		return nil
	}

	syncVpcepEndpointState(state, endpointResp)
	return nil
}

func (r *netConnectM1ToM3Resource) refreshLbmDnsState(ctx context.Context,
	state *netConnectM1ToM3Model) diag.Diagnostics {
	if state.LbmDnsRecordId.IsNull() {
		return nil
	}

	dnsResp, err := r.lbmDnsClient.GetIntranetDnsDomain(ctx, state.LbmDnsRecordId.ValueString())
	tflog.Debug(ctx, "Receive lbm dns query response", map[string]any{
		"response": dnsResp,
	})
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("query lbm-dns record failed", err.Error())
		return diags
	}
	return syncLbmDnsQueryResponse(ctx, state, dnsResp)
}

func syncLbmDnsQueryResponse(ctx context.Context, state *netConnectM1ToM3Model,
	dnsResp *lbmdnsclient.GetIntranetDnsDomainResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	if dnsResp.HTTPStatusCode < 200 || dnsResp.HTTPStatusCode >= 300 {
		diags.AddError("query lbm-dns record failed", fmt.Sprintf("http status is %d", dnsResp.HTTPStatusCode))
		return diags
	}
	if lbmdnsclient.IsNotFound(dnsResp.Body.Code) {
		tflog.Info(ctx, "lbm-dns record not found, marking as null (Partial Repair)", map[string]any{
			"dns_record_id": state.LbmDnsRecordId.ValueString(),
			"status":        dnsResp.Body.Status,
			"code":          dnsResp.Body.Code,
			"msg":           dnsResp.Body.ErrMsg,
		})
		state.LbmDnsRecordId = types.StringNull()
		state.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
		return diags
	}
	diags.Append(syncLbmDnsState(state, dnsResp)...)
	return diags
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
	if endpointResp.Ip != nil {
		state.VpcepEndpointIp = types.StringValue(*endpointResp.Ip)
	}
	if endpointResp.EndpointServiceId != nil {
		state.VpcepEndpointServiceId = types.StringValue(*endpointResp.EndpointServiceId)
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

func (r *netConnectM1ToM3Resource) updateM3VpcepServiceConfig(ctx context.Context, serviceId string,
	plan *netConnectM1ToM3Model) error {
	tcpProtocol := model.GetPortListProtocolEnum().TCP
	ports := make([]model.PortList, len(plan.M3VpcepServicePorts))
	for i := range plan.M3VpcepServicePorts {
		ports[i] = model.PortList{
			ClientPort: &plan.M3VpcepServicePorts[i].ClientPort,
			ServerPort: &plan.M3VpcepServicePorts[i].ServerPort,
			Protocol:   &tcpProtocol,
		}
	}

	updateReq := &model.UpdateEndpointServiceRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.UpdateEndpointServiceRequestBody{
			PortId: &plan.M3PortId,
			Ports:  &ports,
		},
	}

	tflog.Debug(ctx, "Updating vpcep-service", map[string]any{
		"service_id": serviceId,
		"port_id":    plan.M3PortId,
		"ports":      len(plan.M3VpcepServicePorts),
	})

	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m3VpcepClient.UpdateEndpointService(updateReq)
		if innerErr != nil {
			tflog.Warn(ctx, "UpdateEndpointService API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("updateEndpointService API failed after retries: %w", err)
	}
	if err := r.waitForVpcepServiceReady(ctx, serviceId); err != nil {
		return fmt.Errorf("wait for vpcep-service ready failed: %w", err)
	}
	return nil
}

func (r *netConnectM1ToM3Resource) reconcileVpcepServicePermissions(ctx context.Context, serviceId string,
	desired []vpcepServicePermissionBlock) error {
	remote, err := r.listVpcepServicePermissions(ctx, serviceId)
	if err != nil {
		return err
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, permission := range desired {
		desiredSet[permission.Permission] = struct{}{}
	}

	var addPermissions []vpcepServicePermissionBlock
	for permission := range desiredSet {
		if _, ok := remote[permission]; !ok {
			addPermissions = append(addPermissions, vpcepServicePermissionBlock{Permission: permission})
		}
	}
	if len(addPermissions) > 0 {
		if err := r.addVpcepServicePermissions(ctx, serviceId, addPermissions); err != nil {
			return err
		}
	}

	var removePermissions []model.EpsRemovePermissionRequest
	for permission, id := range remote {
		if _, ok := desiredSet[permission]; ok {
			continue
		}
		if id == "" {
			return fmt.Errorf("vpcep-service permission %s has no id", permission)
		}
		removePermissions = append(removePermissions, model.EpsRemovePermissionRequest{Id: id})
	}
	if len(removePermissions) == 0 {
		return nil
	}

	removeReq := &model.BatchRemoveEndpointServicePermissionsRequest{
		VpcEndpointServiceId: serviceId,
		Body: &model.BatchRemoveEndpointServicePermissionsRequestBody{
			Permissions: removePermissions,
		},
	}
	err = utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		_, innerErr := r.m3VpcepClient.BatchRemoveEndpointServicePermissions(removeReq)
		if innerErr != nil {
			tflog.Warn(ctx, "BatchRemoveEndpointServicePermissions API failed, retrying", map[string]any{
				"error": innerErr.Error(),
			})
		}
		return innerErr
	})
	if err != nil {
		return fmt.Errorf("batchRemoveEndpointServicePermissions API failed after retries: %w", err)
	}
	tflog.Info(ctx, "Vpcep-service permissions reconciled", map[string]any{
		"service_id": serviceId,
		"added":      len(addPermissions),
		"removed":    len(removePermissions),
	})
	return nil
}

func (r *netConnectM1ToM3Resource) listVpcepServicePermissions(ctx context.Context,
	serviceId string) (map[string]string, error) {
	getReq := &model.ListServicePermissionsDetailsRequest{
		VpcEndpointServiceId: serviceId,
	}
	var getResp *model.ListServicePermissionsDetailsResponse
	err := utils.RetryWithBackoff(ctx, 3, time.Second, func() error {
		var innerErr error
		getResp, innerErr = r.m3VpcepClient.ListServicePermissionsDetails(getReq)
		return innerErr
	})
	if err != nil {
		return nil, fmt.Errorf("listServicePermissionsDetails API failed after retries: %w", err)
	}

	permissions := make(map[string]string)
	if getResp.Permissions == nil {
		return permissions, nil
	}
	for _, permission := range *getResp.Permissions {
		if permission.Permission == nil {
			continue
		}
		id := ""
		if permission.Id != nil {
			id = *permission.Id
		}
		permissions[*permission.Permission] = id
	}
	return permissions, nil
}

func (r *netConnectM1ToM3Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}

	var plan netConnectM1ToM3Model
	var state netConnectM1ToM3Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceMissing := state.VpcepServiceId.IsNull()
	serviceReplace := !serviceMissing && serviceRequiresReplacement(state, plan)
	serviceUpdate := !serviceMissing && !serviceReplace && serviceRequiresInPlaceUpdate(state, plan)
	endpointUnknown := endpointRequiresUpdate(state, plan, serviceMissing || serviceReplace)
	dnsUnknown, diags := dnsRequiresUpdate(ctx, state, plan, endpointUnknown)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if serviceMissing || serviceReplace || serviceUpdate {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vpcep_service_id"), types.StringUnknown())...)
	}
	if endpointUnknown {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vpcep_endpoint_id"), types.StringUnknown())...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vpcep_endpoint_ip"), types.StringUnknown())...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vpcep_endpoint_service_id"),
			types.StringUnknown())...)
	}
	if dnsUnknown {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("lbm_dns_record_id"), types.StringUnknown())...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("lbm_dns_record_values"),
			types.ListUnknown(lbmDnsRecordValueObjectType))...)
	}
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

	serviceReplaced, err := r.reconcileM1ToM3Service(ctx, state, &plan, &stale)
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
	plan *netConnectM1ToM3Model, stale *m1ToM3StaleResources) (bool, error) {
	serviceMissing := state.VpcepServiceId.IsNull()
	serviceReplace := !serviceMissing && serviceRequiresReplacement(state, *plan)
	serviceUpdate := !serviceMissing && !serviceReplace && serviceRequiresInPlaceUpdate(state, *plan)

	switch {
	case serviceMissing:
		serviceId, err := r.createAndWaitVpcepService(ctx, plan)
		if err != nil {
			return false, err
		}
		plan.VpcepServiceId = types.StringValue(serviceId)
	case serviceReplace:
		oldServiceId := state.VpcepServiceId.ValueString()
		serviceId, err := r.createAndWaitVpcepService(ctx, plan)
		if err != nil {
			return false, err
		}
		plan.VpcepServiceId = types.StringValue(serviceId)
		stale.serviceId = oldServiceId
	case serviceUpdate:
		if err := r.updateExistingM1ToM3Service(ctx, state, plan); err != nil {
			return false, err
		}
		plan.VpcepServiceId = state.VpcepServiceId
	}
	return serviceReplace, nil
}

func (r *netConnectM1ToM3Resource) updateExistingM1ToM3Service(ctx context.Context, state netConnectM1ToM3Model,
	plan *netConnectM1ToM3Model) error {
	serviceId := state.VpcepServiceId.ValueString()
	if servicePortConfigChanged(state, *plan) {
		if err := r.updateM3VpcepServiceConfig(ctx, serviceId, plan); err != nil {
			return err
		}
	}
	if servicePermissionsChanged(state, *plan) {
		return r.reconcileVpcepServicePermissions(ctx, serviceId, plan.M3VpcepServicePermissions)
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
		if err := r.updateLbmDnsRecordValues(ctx, state.LbmDnsRecordId.ValueString(),
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

func endpointRequiresUpdate(state, plan netConnectM1ToM3Model, serviceWillBeReplaced bool) bool {
	return state.VpcepEndpointId.IsNull() || serviceWillBeReplaced ||
		shouldReplaceEndpoint(state, plan, false)
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

func dnsRequiresUpdate(ctx context.Context, state, plan netConnectM1ToM3Model,
	endpointWillBeUpdated bool) (bool, diag.Diagnostics) {
	if state.LbmDnsRecordId.IsNull() || dnsIdentityChanged(state, plan) || endpointWillBeUpdated {
		return true, nil
	}
	return lbmDnsRecordValueNeedsUpdate(ctx, state.LbmDnsRecordValues, plan.VpcepEndpointIp)
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

func lbmDnsRecordAValue(ctx context.Context, values types.List) (string, bool, diag.Diagnostics) {
	var diags diag.Diagnostics
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
		if err := r.deleteLbmDnsRecord(ctx, stale.dnsRecordId); err != nil {
			warnings = append(warnings, fmt.Sprintf("delete stale lbm-dns record %s failed: %s", stale.dnsRecordId,
				err.Error()))
		}
	}
	if stale.endpointId != "" {
		if err := r.deleteM1PlusVpcepEndpoint(ctx, stale.endpointId); err != nil {
			warnings = append(warnings, fmt.Sprintf("delete stale vpcep-endpoint %s failed: %s", stale.endpointId,
				err.Error()))
		}
	}
	if stale.serviceId != "" {
		if err := r.deleteM3VpcepService(ctx, stale.serviceId); err != nil {
			warnings = append(warnings, fmt.Sprintf("delete stale vpcep-service %s failed: %s", stale.serviceId,
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

	// 删除顺序：DNS → vpcep-endpoint → vpcep-service。由于资源间有依赖关系，因此资源删除失败时会终止整体删除流程
	var deleteErr error
	if !state.LbmDnsRecordId.IsNull() {
		if err := r.deleteLbmDnsRecord(ctx, state.LbmDnsRecordId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete lbm-dns record %s, the vpcep endpoint and service remain intact: %w",
				state.LbmDnsRecordId.ValueString(), err)
		} else {
			state.LbmDnsRecordId = types.StringNull()
			state.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
		}
	}

	if !state.VpcepEndpointId.IsNull() && deleteErr == nil {
		if err := r.deleteM1PlusVpcepEndpoint(ctx, state.VpcepEndpointId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("failed to delete vpcep-endpoint %s, the vpcep service remains intact: %w",
				state.VpcepEndpointId.ValueString(), err)
		} else {
			state.VpcepEndpointId = types.StringNull()
			state.VpcepEndpointIp = types.StringNull()
			state.VpcepEndpointServiceId = types.StringNull()
		}
	}

	if !state.VpcepServiceId.IsNull() && deleteErr == nil {
		if err := r.deleteM3VpcepService(ctx, state.VpcepServiceId.ValueString()); err != nil {
			deleteErr = fmt.Errorf("delete vpcep-service %s failed: %w", state.VpcepServiceId.ValueString(), err)
		} else {
			state.VpcepServiceId = types.StringNull()
		}
	}

	// 删除失败：更新 state 以反映部分删除的状态
	if deleteErr != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		resp.Diagnostics.AddError("delete m1-to-m3 network connection failed", deleteErr.Error())
		return
	}

	// 删除成功：从 state 中完全移除资源
	resp.State.RemoveResource(ctx)
}
