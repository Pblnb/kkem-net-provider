/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/manager"
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

type mockVpcepServiceManager struct {
	// 错误注入
	createErr    error
	addErr       error
	deleteErr    error
	updateErr    error
	reconcileErr error

	// 返回数据
	createServiceId      string
	getOutput            *manager.VpcepServiceOutput
	getErr               error
	getPermissionsOutput map[string]string
	getPermissionsErr    error

	// 调用记录
	getId                     string
	getCalls                  int
	getPermissionsId          string
	getPermissionsCalls       int
	createInputs              []manager.VpcepServiceInput
	deleteIds                 []string
	addServiceIds             []string
	addPermissions            [][]manager.PermissionInput
	updateServiceIds          []string
	updateServiceInputs       []manager.VpcepServiceInput
	reconcilePermissionIds    []string
	reconcilePermissionInputs [][]manager.PermissionInput
}

func (m *mockVpcepServiceManager) Create(_ context.Context, input manager.VpcepServiceInput) (string, error) {
	m.createInputs = append(m.createInputs, input)
	return m.createServiceId, m.createErr
}

func (m *mockVpcepServiceManager) Delete(_ context.Context, serviceId string) error {
	m.deleteIds = append(m.deleteIds, serviceId)
	return m.deleteErr
}

func (m *mockVpcepServiceManager) Get(_ context.Context, serviceId string) (*manager.VpcepServiceOutput, error) {
	m.getId = serviceId
	m.getCalls++
	return m.getOutput, m.getErr
}

func (m *mockVpcepServiceManager) AddPermissions(_ context.Context, serviceId string,
	permissions []manager.PermissionInput) error {
	m.addServiceIds = append(m.addServiceIds, serviceId)
	m.addPermissions = append(m.addPermissions, permissions)
	return m.addErr
}

func (m *mockVpcepServiceManager) GetPermissions(_ context.Context, serviceId string) (map[string]string, error) {
	m.getPermissionsId = serviceId
	m.getPermissionsCalls++
	return m.getPermissionsOutput, m.getPermissionsErr
}

func (m *mockVpcepServiceManager) UpdateConfig(_ context.Context, serviceId string,
	input manager.VpcepServiceInput) error {
	m.updateServiceIds = append(m.updateServiceIds, serviceId)
	m.updateServiceInputs = append(m.updateServiceInputs, input)
	return m.updateErr
}

func (m *mockVpcepServiceManager) ReconcilePermissions(_ context.Context, serviceId string,
	permissions []manager.PermissionInput) error {
	m.reconcilePermissionIds = append(m.reconcilePermissionIds, serviceId)
	m.reconcilePermissionInputs = append(m.reconcilePermissionInputs, permissions)
	return m.reconcileErr
}

type mockVpcepEndpointManager struct {
	// 错误注入
	createErr error
	deleteErr error
	getErr    error

	// 返回数据
	createEndpointId string
	createEndpointIp string
	getOutput        *manager.VpcepEndpointOutput

	// 调用记录
	getId        string
	getCalls     int
	createInputs []manager.VpcEndpointInput
	deleteIds    []string
}

func (m *mockVpcepEndpointManager) Create(_ context.Context, input manager.VpcEndpointInput) (string, string, error) {
	m.createInputs = append(m.createInputs, input)
	return m.createEndpointId, m.createEndpointIp, m.createErr
}

func (m *mockVpcepEndpointManager) Delete(_ context.Context, endpointId string) error {
	m.deleteIds = append(m.deleteIds, endpointId)
	return m.deleteErr
}

func (m *mockVpcepEndpointManager) Get(_ context.Context, endpointId string) (*manager.VpcepEndpointOutput, error) {
	m.getId = endpointId
	m.getCalls++
	return m.getOutput, m.getErr
}

type mockLbmDnsManager struct {
	// 错误注入
	createErr    error
	getDetailErr error

	// 返回数据
	createOutput    *manager.CreateLbmDnsOutput
	getDetailOutput *manager.LbmDnsDetailOutput

	// 调用记录
	getDetailId    string
	getDetailCalls int
	createInputs   []manager.CreateLbmDnsInput
}

func (f *mockLbmDnsManager) CreateIntranetDnsDomain(_ context.Context,
	input manager.CreateLbmDnsInput) (*manager.CreateLbmDnsOutput, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createOutput, f.createErr
}

func (f *mockLbmDnsManager) DeleteIntranetDnsDomain(_ context.Context, _ string) error {
	return nil
}

func (f *mockLbmDnsManager) UpdateRecordValue(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *mockLbmDnsManager) GetLbmDnsDetail(_ context.Context,
	recordId string) (*manager.LbmDnsDetailOutput, error) {
	f.getDetailId = recordId
	f.getDetailCalls++
	return f.getDetailOutput, f.getDetailErr
}

type mockM3ToM1DnsManager struct {
	// 错误注入
	createZoneErr    error
	createRecordErr  error
	deletePrivateErr error

	// 返回数据
	createPrivateZoneId string
	createRecordSetId   string

	// 调用记录
	zoneInputs    []manager.DnsZoneInput
	recordInputs  []manager.DnsRecordSetInput
	deleteZoneIds []string
}

func (m *mockM3ToM1DnsManager) CreatePrivateZone(_ context.Context, input manager.DnsZoneInput) (string, error) {
	m.zoneInputs = append(m.zoneInputs, input)
	return m.createPrivateZoneId, m.createZoneErr
}

func (m *mockM3ToM1DnsManager) CreateRecordSet(_ context.Context, input manager.DnsRecordSetInput) (string, error) {
	m.recordInputs = append(m.recordInputs, input)
	return m.createRecordSetId, m.createRecordErr
}

func (m *mockM3ToM1DnsManager) DeletePrivateZone(_ context.Context, zoneId string) error {
	m.deleteZoneIds = append(m.deleteZoneIds, zoneId)
	return m.deletePrivateErr
}

func (m *mockM3ToM1DnsManager) GetPrivateZone(_ context.Context, _ string) (*manager.DnsZoneOutput, error) {
	return nil, nil
}

type mockM3ToM1SniProxyManager struct {
	// 错误注入
	accessErr error
	deleteErr error

	// 返回数据
	accessResourceId string

	// 调用记录
	accessInputs []manager.AccessSniProxyInput
	deleteIds    []string
}

func (m *mockM3ToM1SniProxyManager) AccessSniProxy(_ context.Context,
	input manager.AccessSniProxyInput) (string, error) {
	m.accessInputs = append(m.accessInputs, input)
	return m.accessResourceId, m.accessErr
}

func (m *mockM3ToM1SniProxyManager) DeleteSniProxy(_ context.Context, resourceId string) error {
	m.deleteIds = append(m.deleteIds, resourceId)
	return m.deleteErr
}

func (m *mockM3ToM1SniProxyManager) GetSniProxy(_ context.Context,
	_ string) (*manager.AccessSniProxyOutput, *sniproxyclient.GetAccessServiceResponse, error) {
	return nil, nil, nil
}
