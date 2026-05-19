/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import (
	"context"
	"net/http"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
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

// VPCEP 测试数据/函数
const (
	testSubnetId = "subnet-1"
	testVpcId    = "vpc-1"

	testVpcepServiceId           = "service-1"
	testVpcepServicePortId       = "port-1"
	testVpcepServicePermission   = "iam:domain::domain-1"
	testVpcepServicePermissionId = "permission-1"

	testVpcepEndpointId = "endpoint-1"
	testVpcepEndpointIp = "10.0.0.8"

	testVpcepNotFoundErrorCode = "EndPoint.0005"
)

var vpcepNotFoundError = &sdkerr.ServiceResponseError{
	StatusCode: http.StatusNotFound,
	ErrorCode:  testVpcepNotFoundErrorCode,
}

// SNI Proxy 测试数据/函数
const (
	testSniProxyRegionCode       = "region-1"
	testSniProxyServiceName      = "KKEM"
	testSniProxyIamDomainAccount = "account-1"
	testSniProxyResourceId       = "2026"
	testSniProxyEpServiceId      = "ep-2026"
	testSniProxyAccessObject     = "APIGW"
)
