/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

const (
	pathCreateIntranetDnsDomain        = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet`
	pathGetIntranetDnsDomainTaskStatus = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/result/%s`
)

const (
	StatusCodeSuccess = 0
	TaskStatusSuccess = "success"
	TaskStatusFailed  = "failed"
)

type baseResponse struct {
	Status       int    `json:"status"`
	Code         int    `json:"code"`
	ErrMsg       string `json:"msg"`
	ProviderCode string `json:"provider_code"`
}

type domainChangeResponseBody struct {
	baseResponse
	TaskId string `json:"data,omitempty"`
}

type domainChangeResponse struct {
	Body           domainChangeResponseBody
	HTTPStatusCode int
}

type domainTaskStatusResponseBody struct {
	baseResponse
	Data domainStatus `json:"data,omitempty"`
}

type domainTaskStatusResponse struct {
	Body           domainTaskStatusResponseBody
	HTTPStatusCode int
}

type domainStatus struct {
	ResourceId string `json:"resourceId,omitempty"`
	Status     string `json:"status,omitempty"`
	Message    string `json:"msg,omitempty"`
}

type intranetDnsDomainResource struct {
	RegionCode   string        `json:"regionCode" required:"true"`
	ServiceName  string        `json:"serviceName" required:"true"`
	HostRecord   string        `json:"hostRecord" required:"true"`
	DomainSuffix string        `json:"domainSuffix" required:"true"`
	RecordValues []recordValue `json:"recordValues" required:"true"`
}

type recordValue struct {
	RecordType  string `json:"recordType" required:"true"`
	RecordValue string `json:"recordValue" required:"true"`
}
