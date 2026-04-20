/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

const (
	pathCreateIntranetDnsDomain        = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet`
	pathGetIntranetDnsDomainTaskStatus = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/result/%s`
	pathGetIntranetDnsDomain           = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/%s`
)

const (
	StatusCodeSuccess          = 0
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

// CreateIntranetDnsDomainResponseBody 创建域名记录的响应体。
type CreateIntranetDnsDomainResponseBody struct {
	baseResponse
	TaskId string `json:"data,omitempty"`
}

// CreateIntranetDnsDomainResponse 创建域名记录响应，包含响应体和 HTTP 状态码。
type CreateIntranetDnsDomainResponse struct {
	Body           CreateIntranetDnsDomainResponseBody
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

// IsIntranetDnsDomainNotFound checks lbm-dns not-found responses.
func IsIntranetDnsDomainNotFound(resp *GetIntranetDnsDomainResponse) bool {
	if resp == nil {
		return false
	}
	return resp.Body.Code == StatusCodeResourceNotFound
}

type domainStatus struct {
	ResourceId string `json:"resourceId,omitempty"`
	Status     string `json:"status,omitempty"`
	Message    string `json:"msg,omitempty"`
}

// IntranetDnsDomainResource describes an lbm-dns record resource.
type IntranetDnsDomainResource struct {
	RegionCode   string                   `json:"regionCode" required:"true"`
	ServiceName  string                   `json:"serviceName" required:"true"`
	HostRecord   string                   `json:"hostRecord" required:"true"`
	DomainSuffix string                   `json:"domainSuffix" required:"true"`
	RecordValues []IntranetDnsRecordValue `json:"recordValues" required:"true"`
}

// IntranetDnsRecordValue describes a single lbm-dns record value.
type IntranetDnsRecordValue struct {
	RecordType  string `json:"recordType" required:"true"`
	RecordValue string `json:"recordValue" required:"true"`
}
