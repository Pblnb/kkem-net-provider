/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package lbmdnsclient

const (
	pathCreateIntranetDnsDomain        = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet`
	pathGetIntranetDnsDomainTaskStatus = `/CloudLBMgmt/external-api/v1/cloud-lb/iac/dns-config/intranet/result/%s`
)

const (
	taskStatusFailed = "failed"
	taskStatusCancel = "cancel"
)

type baseResponse struct {
	Status       int    `json:"status"`
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	ProviderCode string `json:"provider_code"`
}

type domainChangeResponse struct {
	baseResponse
	TaskId string `json:"data,omitempty"`
}

type domainTaskStatusResponse struct {
	baseResponse
	Data domainStatus `json:"data,omitempty"`
}

type domainStatus struct {
	ResourceId   string `json:"resourceId,omitempty"`
	Status       string `json:"status,omitempty"`
	ErrorMessage string `json:"msg,omitempty"`
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
