/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/service"
)

const (
	testM3VpcId            = "m3-vpc-1"
	testM3ServerType       = "LB"
	testM3PortId           = "port-1"
	testM1PlusVpcId        = "m1-vpc-1"
	testM1PlusSubnetId     = "subnet-1"
	testDnsDomain          = "api"
	testDnsDomainSuffix    = "internal"
	testLbmDnsServiceName  = "service-name-1"
	testRegionCode         = "region-1"
	testVpcepServiceId     = "service-1"
	testVpcepEndpointId    = "endpoint-1"
	testVpcepEndpointIp    = "10.0.0.8"
	testLbmDnsRecordId     = "dns-record-1"
	testSniProxyID         = "sni-1"
	testVpcepID            = "vpcep-1"
	testDnsID              = "dns-1"
	testIamDomainId        = "domain-id-1"
	testAnotherIamDomainId = "domain-id-2"
	testPermissionId       = "permission-1"
	testM3SubnetId         = "m3-subnet-1"
	testDomainName         = "test.example.com"
	testDomainAccount      = "account-1"
	testServiceName        = "KKEM"
)

func testVpcepServicePorts() []vpcepServicePortBlock {
	return []vpcepServicePortBlock{
		{ClientPort: 80, ServerPort: 8080},
		{ClientPort: 443, ServerPort: 8443},
	}
}

func testVpcepServicePermissions() []vpcepServicePermissionBlock {
	return []vpcepServicePermissionBlock{
		{Permission: testIamDomainId},
		{Permission: testAnotherIamDomainId},
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

type mockVpcepServiceService struct {
	// 错误注入
	createErr    error
	addErr       error
	deleteErr    error
	updateErr    error
	reconcileErr error

	// 返回数据
	createServiceId      string
	getOutput            *service.VpcepServiceOutput
	getErr               error
	getPermissionsOutput map[string]string
	getPermissionsErr    error

	// 调用记录
	getId                     string
	getCalls                  int
	getPermissionsId          string
	getPermissionsCalls       int
	createInputs              []service.VpcepServiceInput
	deleteIds                 []string
	addServiceIds             []string
	addPermissions            [][]service.PermissionInput
	updateServiceIds          []string
	updateServiceInputs       []service.VpcepServiceInput
	reconcilePermissionIds    []string
	reconcilePermissionInputs [][]service.PermissionInput
}

func (f *mockVpcepServiceService) Create(_ context.Context, input service.VpcepServiceInput) (string, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createServiceId, f.createErr
}

func (f *mockVpcepServiceService) Delete(_ context.Context, serviceId string) error {
	f.deleteIds = append(f.deleteIds, serviceId)
	return f.deleteErr
}

func (f *mockVpcepServiceService) Get(_ context.Context, serviceId string) (*service.VpcepServiceOutput, error) {
	f.getId = serviceId
	f.getCalls++
	return f.getOutput, f.getErr
}

func (f *mockVpcepServiceService) AddPermissions(_ context.Context, serviceId string,
	permissions []service.PermissionInput) error {
	f.addServiceIds = append(f.addServiceIds, serviceId)
	f.addPermissions = append(f.addPermissions, permissions)
	return f.addErr
}

func (f *mockVpcepServiceService) GetPermissions(_ context.Context, serviceId string) (map[string]string, error) {
	f.getPermissionsId = serviceId
	f.getPermissionsCalls++
	return f.getPermissionsOutput, f.getPermissionsErr
}

func (f *mockVpcepServiceService) UpdateConfig(_ context.Context, serviceId string,
	input service.VpcepServiceInput) error {
	f.updateServiceIds = append(f.updateServiceIds, serviceId)
	f.updateServiceInputs = append(f.updateServiceInputs, input)
	return f.updateErr
}

func (f *mockVpcepServiceService) ReconcilePermissions(_ context.Context, serviceId string,
	permissions []service.PermissionInput) error {
	f.reconcilePermissionIds = append(f.reconcilePermissionIds, serviceId)
	f.reconcilePermissionInputs = append(f.reconcilePermissionInputs, permissions)
	return f.reconcileErr
}

type mockVpcepEndpointService struct {
	// 错误注入
	createErr error
	deleteErr error
	getErr    error

	// 返回数据
	createEndpointId string
	createEndpointIp string
	getOutput        *service.VpcepEndpointOutput

	// 调用记录
	getId        string
	getCalls     int
	createInputs []service.VpcEndpointInput
	deleteIds    []string
}

func (f *mockVpcepEndpointService) Create(_ context.Context, input service.VpcEndpointInput) (string, string, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createEndpointId, f.createEndpointIp, f.createErr
}

func (f *mockVpcepEndpointService) Delete(_ context.Context, endpointId string) error {
	f.deleteIds = append(f.deleteIds, endpointId)
	return f.deleteErr
}

func (f *mockVpcepEndpointService) Get(_ context.Context, endpointId string) (*service.VpcepEndpointOutput, error) {
	f.getId = endpointId
	f.getCalls++
	return f.getOutput, f.getErr
}

type mockLbmDnsService struct {
	// 错误注入
	createErr    error
	getDetailErr error

	// 返回数据
	createOutput    *service.CreateLbmDnsOutput
	getDetailOutput *service.LbmDnsDetailOutput

	// 调用记录
	getDetailId    string
	getDetailCalls int
	createInputs   []service.CreateLbmDnsInput
}

func (f *mockLbmDnsService) CreateIntranetDnsDomain(_ context.Context,
	input service.CreateLbmDnsInput) (*service.CreateLbmDnsOutput, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createOutput, f.createErr
}

func (f *mockLbmDnsService) DeleteIntranetDnsDomain(_ context.Context, _ string) error {
	return nil
}

func (f *mockLbmDnsService) UpdateRecordValue(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *mockLbmDnsService) GetLbmDnsDetail(_ context.Context,
	recordId string) (*service.LbmDnsDetailOutput, error) {
	f.getDetailId = recordId
	f.getDetailCalls++
	return f.getDetailOutput, f.getDetailErr
}

type mockM3ToM1DnsService struct {
	// 错误注入
	createZoneErr    error
	createRecordErr  error
	deletePrivateErr error

	// 返回数据
	createPrivateZoneId string
	createRecordSetId   string

	// 调用记录
	zoneInputs    []service.DnsZoneInput
	recordInputs  []service.DnsRecordSetInput
	deleteZoneIds []string
}

func (m *mockM3ToM1DnsService) CreatePrivateZone(_ context.Context, input service.DnsZoneInput) (string, error) {
	m.zoneInputs = append(m.zoneInputs, input)
	return m.createPrivateZoneId, m.createZoneErr
}

func (m *mockM3ToM1DnsService) CreateRecordSet(_ context.Context, input service.DnsRecordSetInput) (string, error) {
	m.recordInputs = append(m.recordInputs, input)
	return m.createRecordSetId, m.createRecordErr
}

func (m *mockM3ToM1DnsService) DeletePrivateZone(_ context.Context, zoneId string) error {
	m.deleteZoneIds = append(m.deleteZoneIds, zoneId)
	return m.deletePrivateErr
}

func (m *mockM3ToM1DnsService) GetPrivateZone(_ context.Context, _ string) (*service.DnsZoneOutput, error) {
	return nil, nil
}

type mockM3ToM1SniProxyService struct {
	// 错误注入
	accessErr error
	deleteErr error

	// 返回数据
	accessResourceId string

	// 调用记录
	accessInputs []service.AccessSniProxyInput
	deleteIds    []string
}

func (m *mockM3ToM1SniProxyService) AccessSniProxy(_ context.Context,
	input service.AccessSniProxyInput) (string, error) {
	m.accessInputs = append(m.accessInputs, input)
	return m.accessResourceId, m.accessErr
}

func (m *mockM3ToM1SniProxyService) DeleteSniProxy(_ context.Context, resourceId string) error {
	m.deleteIds = append(m.deleteIds, resourceId)
	return m.deleteErr
}

func (m *mockM3ToM1SniProxyService) GetSniProxy(_ context.Context,
	_ string) (*service.AccessSniProxyOutput, *sniproxyclient.GetAccessServiceResponse, error) {
	return nil, nil, nil
}
