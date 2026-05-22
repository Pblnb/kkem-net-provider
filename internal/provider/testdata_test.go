/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	testM3VpcId           = "m3-vpc-1"
	testM3ServerType      = "LB"
	testM3PortId          = "port-1"
	testM3SubnetId        = "m3-subnet-1"
	testM1PlusVpcId       = "m1-vpc-1"
	testM1PlusSubnetId    = "m1-subnet-1"
	testDnsDomain         = "api"
	testDnsDomainSuffix   = "internal"
	testLbmDnsServiceName = "service-name-1"
	testRegionCode        = "region-1"
	testVpcepServiceId    = "service-1"
	testVpcepEndpointId   = "endpoint-1"
	testVpcepEndpointIp   = "10.0.0.8"
	testLbmDnsRecordId    = "dns-record-1"
	testSniProxyID        = "sni-1"
	testVpcepID           = "vpcep-1"
	testDnsID             = "dns-1"
	testDomainName        = "test.example.com"
	testDomainAccount     = "account-1"
	testServiceName       = "KKEM"
)

func testVpcepServicePorts() []vpcepServicePortBlock {
	return []vpcepServicePortBlock{
		{ClientPort: 80, ServerPort: 8080},
		{ClientPort: 443, ServerPort: 8443},
	}
}

func testVpcepServicePermissions() []vpcepServicePermissionBlock {
	return []vpcepServicePermissionBlock{
		{Permission: "domain-id-a"},
		{Permission: "domain-id-b"},
	}
}

// testLbmDnsRecordValues 用于无 *testing.T 的基础 fixture 构造，输入固定为有效测试数据。
func testLbmDnsRecordValues(values []lbmDnsRecordValueBlock) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.ObjectValueMust(lbmDnsRecordValueAttrTypes, map[string]attr.Value{
			"record_type":  types.StringValue(value.RecordType),
			"record_value": types.StringValue(value.RecordValue),
		}))
	}
	return types.ListValueMust(lbmDnsRecordValueObjectType, elements)
}

func newTestPlan() netConnectM3ToM1Model {
	return netConnectM3ToM1Model{
		M3VpcID:               types.StringValue(testM3VpcId),
		M3VpcEndpointSubnetId: types.StringValue(testM3SubnetId),
		SniVpcepServerId:      types.StringValue(testVpcepServiceId),
		M3DnsDomainName:       types.StringValue(testDomainName),
		RegionCode:            types.StringValue(testRegionCode),
		ServiceName:           types.StringValue(testServiceName),
		DomainAccount:         types.StringValue(testDomainAccount),
	}
}
