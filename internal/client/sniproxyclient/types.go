/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package sniproxyclient

const (
	statusCodeNoExist          = 6082
	statusCodeNoStaticResource = -1

	pathServiceAccess       = `/external-api/v1/lsp/iac/service/access`
	pathServiceAccessDelete = `/external-api/v1/lsp/iac/service/access/%s`
	pathServiceAccessGet    = `/external-api/v1/lsp/iac/service/access/%s`
)

// AccessServiceRequest 接入服务请求
type AccessServiceRequest struct {
	RegionCode       string   `json:"region_code" required:"true"`
	ServiceName      string   `json:"service_name" required:"true"`
	AccessObject     string   `json:"access_object" required:"true"`
	IamDomainAccount []string `json:"iam_domain_account" required:"true"`
	ChangeReason     string   `json:"change_reason"`
}

type BaseResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// EpServiceInfo VPCE 服务信息
type EpServiceInfo struct {
	RegionCode    string `json:"region_code"`
	Az            string `json:"az"`
	EpServiceId   string `json:"ep_service_id"`
	EpServiceName string `json:"ep_service_name"`
}

// AccessServiceResponseData 接入服务响应数据
type AccessServiceResponseData struct {
	ResourceId       string          `json:"resource_id"`
	ServiceName      string          `json:"service_name"`
	AccessObject     string          `json:"access_object"`
	RegionCode       string          `json:"region_code"`
	IamDomainAccount []string        `json:"iam_domain_account"`
	EpServiceIds     []EpServiceInfo `json:"ep_service_ids"`
}

// AccessServiceResponse 接入服务响应
type AccessServiceResponse struct {
	Body           AccessServiceResponseBody
	HTTPStatusCode int
}

// AccessServiceResponseBody 接入服务响应体
type AccessServiceResponseBody struct {
	BaseResponse
	Data AccessServiceResponseData `json:"data"`
}

// DeleteAccessServiceResponse 删除服务接入响应
type DeleteAccessServiceResponse struct {
	Body           BaseResponse
	HTTPStatusCode int
}

// GetAccessServiceResponse 查询服务接入响应
type GetAccessServiceResponse struct {
	Body           GetAccessServiceResponseBody
	HTTPStatusCode int
}

// GetAccessServiceResponseBody 查询服务接入响应体
type GetAccessServiceResponseBody struct {
	BaseResponse
	Data AccessServiceResponseData `json:"data"`
}
