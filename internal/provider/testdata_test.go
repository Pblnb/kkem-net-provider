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
	testM1PlusVpcId       = "m1-vpc-1"
	testM1PlusSubnetId    = "subnet-1"
	testDnsDomain         = "api"
	testDnsDomainSuffix   = "internal"
	testLbmDnsServiceName = "service-name-1"
	testRegionCode        = "region-1"
	testVpcepServiceId    = "service-1"
	testVpcepEndpointId   = "endpoint-1"
	testVpcepEndpointIp   = "10.0.0.8"
	testLbmDnsRecordId    = "dns-record-1"
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
