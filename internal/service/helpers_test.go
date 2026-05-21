/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"net/http"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpcep/v1/model"
)

// 通用测试数据/函数
func ptr[T any](v T) *T {
	return &v
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// VPCEP 通用测试数据/函数
const (
	testVpcId = "vpc-1"

	testVpcepNotFoundErrorCode = "EndPoint.0005"
)

var vpcepNotFoundError = &sdkerr.ServiceResponseError{
	StatusCode: http.StatusNotFound,
	ErrorCode:  testVpcepNotFoundErrorCode,
}

// VPCEP Service 测试数据/函数
const (
	testVpcepServiceId     = "service-1"
	testVpcepServicePortId = "port-1"

	testVpcepServicePermission      = "iam:domain::domain-1"
	testVpcepServicePermissionId    = "permission-1"
	testVpcepServiceExtraPermission = "iam:domain::extra"
)

var (
	testVpcepServiceClientPort  = int32(80)
	testVpcepServiceServerPort  = int32(8080)
	testVpcepServiceTcpProtocol = model.GetPortListProtocolEnum().TCP
)

// VPCEP Endpoint 测试数据/函数
const (
	testSubnetId        = "subnet-1"
	testVpcepEndpointId = "endpoint-1"
	testVpcepEndpointIp = "10.0.0.8"
)

// SNI Proxy 测试数据/函数
const (
	testSniProxyRegionCode       = "region-1"
	testSniProxyServiceName      = "KKEM"
	testSniProxyIamDomainAccount = "account-1"
	testSniProxyResourceId       = "2026"
	testSniProxyEpServiceId      = "ep-2026"
	testSniProxyAccessObject     = "APIGW"
)
