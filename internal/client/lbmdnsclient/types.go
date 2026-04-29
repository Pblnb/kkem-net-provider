/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

import (
	"context"
)

type LbmDnsClient interface {
	CreateIntranetDnsDomain(ctx context.Context, regionCode, serviceName, hostRecord,
		domainSuffix, ip string) (*AsyncTaskResponse, error)
	GetIntranetDnsDomainTaskStatus(ctx context.Context, taskId string) (*GetIntranetDnsDomainTaskStatusResponse, error)
	GetIntranetDnsDomain(ctx context.Context, resourceId string) (*GetIntranetDnsDomainResponse, error)
	UpdateIntranetDnsDomain(ctx context.Context, resourceId, ip string) (*AsyncTaskResponse, error)
	DeleteIntranetDnsDomain(ctx context.Context, resourceId string) (*AsyncTaskResponse, error)
}

const (
	pathCreateIntranetDnsDomain        = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet`
	pathGetIntranetDnsDomainTaskStatus = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/result/%s`
	pathIntranetDnsDomainResource      = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/%s`
)

const (
	StatusCodeSuccess          = 0
	StatusCodeNoChanges        = 101
	StatusCodeResourceNotFound = 6702
	TaskStatusSuccess          = "success"
	TaskStatusFailed           = "failed"
)

type baseResponse struct {
	Status       int    `json:"status"`
	Code         int    `json:"code"`
	ErrMsg       string `json:"msg"`
	ProviderCode string `json:"provider_code"`
}

// AsyncTaskResponseBody 异步任务响应体，包含任务 ID。
type AsyncTaskResponseBody struct {
	baseResponse
	TaskId string `json:"data,omitempty"`
}

// AsyncTaskResponse 异步任务响应，包含响应体和 HTTP 状态码。
type AsyncTaskResponse struct {
	Body           AsyncTaskResponseBody
	HTTPStatusCode int
}

// GetIntranetDnsDomainTaskStatusResponseBody 查询任务状态的响应体。
type GetIntranetDnsDomainTaskStatusResponseBody struct {
	baseResponse
	Data domainStatus `json:"data"`
}

// GetIntranetDnsDomainTaskStatusResponse 查询任务状态响应，包含响应体和 HTTP 状态码。
type GetIntranetDnsDomainTaskStatusResponse struct {
	Body           GetIntranetDnsDomainTaskStatusResponseBody
	HTTPStatusCode int
}

// GetIntranetDnsDomainResponseBody 查询域名记录的响应体。
type GetIntranetDnsDomainResponseBody struct {
	baseResponse
	Data *IntranetDnsDomainResource `json:"data"`
}

// GetIntranetDnsDomainResponse 查询域名记录响应，包含响应体和 HTTP 状态码。
type GetIntranetDnsDomainResponse struct {
	Body           GetIntranetDnsDomainResponseBody
	HTTPStatusCode int
}

// IsNotFound 检查 lbm-dns 的 not-found 响应码。
func IsNotFound(code int) bool {
	return code == StatusCodeResourceNotFound
}

type domainStatus struct {
	ResourceId string `json:"resourceId,omitempty"`
	Status     string `json:"status,omitempty"`
	Message    string `json:"msg,omitempty"`
}

// IntranetDnsDomainResource 描述 lbm-dns 记录资源。
type IntranetDnsDomainResource struct {
	RegionCode   string                   `json:"regionCode" required:"true"`
	ServiceName  string                   `json:"serviceName" required:"true"`
	HostRecord   string                   `json:"hostRecord" required:"true"`
	DomainSuffix string                   `json:"domainSuffix" required:"true"`
	RecordValues []IntranetDnsRecordValue `json:"recordValues" required:"true"`
}

// IntranetDnsDomainRecordValues 描述 lbm-dns 记录值的更新。
type IntranetDnsDomainRecordValues struct {
	RecordValues []IntranetDnsRecordValue `json:"recordValues" required:"true"`
}

// IntranetDnsRecordValue 描述单条 lbm-dns 记录值。
type IntranetDnsRecordValue struct {
	RecordType  string `json:"recordType" required:"true"`
	RecordValue string `json:"recordValue" required:"true"`
}
