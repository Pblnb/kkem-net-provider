/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"huawei.com/kkem/kkem-net-provider/internal/service"
)

func TestNewNetConnectM1ToM3Resource(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{
			name: "GIVEN m1 to m3 resource factory WHEN NewNetConnectM1ToM3Resource SHOULD return resource instance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := NewNetConnectM1ToM3Resource()

			assert.NotNil(t, actual)
			assert.IsType(t, &netConnectM1ToM3Resource{}, actual)
		})
	}
}

func Test_netConnectM1ToM3Resource_Metadata(t *testing.T) {
	testCases := []struct {
		name             string
		providerTypeName string
		expected         string
	}{
		{
			name:             "GIVEN provider type name WHEN Metadata SHOULD set m1 to m3 resource type name",
			providerTypeName: "kkem",
			expected:         "kkem_net_connect_m1_to_m3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &resource.MetadataResponse{}

			(&netConnectM1ToM3Resource{}).Metadata(context.Background(),
				resource.MetadataRequest{ProviderTypeName: tc.providerTypeName}, resp)

			assert.Equal(t, tc.expected, resp.TypeName)
		})
	}
}

func Test_netConnectM1ToM3Resource_Schema(t *testing.T) {
	testCases := []struct {
		name               string
		requiredAttributes []string
		computedAttributes []string
	}{
		{
			name: "GIVEN m1 to m3 resource WHEN Schema SHOULD return required and computed attributes",
			requiredAttributes: []string{
				"m3_vpc_id",
				"m3_server_type",
				"m3_port_id",
				"m3_vpcep_service_ports",
				"m3_vpcep_service_permissions",
				"m1_plus_vpc_id",
				"m1_plus_subnet_id",
				"dns_domain",
				"dns_domain_suffix",
				"lbm_dns_service_name",
				"region_code",
			},
			computedAttributes: []string{
				"vpcep_service_id",
				"vpcep_endpoint_id",
				"vpcep_endpoint_ip",
				"vpcep_endpoint_service_id",
				"lbm_dns_record_id",
				"lbm_dns_record_values",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &resource.SchemaResponse{}

			(&netConnectM1ToM3Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)

			assert.False(t, resp.Diagnostics.HasError())
			assert.Len(t, resp.Schema.Attributes, len(tc.requiredAttributes)+len(tc.computedAttributes))
			for _, attrName := range tc.requiredAttributes {
				attribute, ok := resp.Schema.Attributes[attrName]
				require.True(t, ok, "schema should contain attribute: %s", attrName)
				assert.True(t, attribute.IsRequired(), "%s should be required", attrName)
			}
			for _, attrName := range tc.computedAttributes {
				attribute, ok := resp.Schema.Attributes[attrName]
				require.True(t, ok, "schema should contain attribute: %s", attrName)
				assert.True(t, attribute.IsComputed(), "%s should be computed", attrName)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_Configure(t *testing.T) {
	testCases := []struct {
		name         string
		providerData any
		expectedErr  string
		expectedInit bool
	}{
		{
			name: "GIVEN nil provider data WHEN Configure SHOULD keep services unset",
		},
		{
			name:         "GIVEN valid provider data WHEN Configure SHOULD initialize services",
			providerData: &clients{},
			expectedInit: true,
		},
		{
			name:         "GIVEN invalid provider data WHEN Configure SHOULD return diagnostics",
			providerData: "invalid",
			expectedErr:  "configure error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target := &netConnectM1ToM3Resource{}
			resp := &resource.ConfigureResponse{}

			target.Configure(context.Background(), resource.ConfigureRequest{ProviderData: tc.providerData}, resp)

			if tc.expectedErr == "" {
				assert.False(t, resp.Diagnostics.HasError())
			} else {
				assertDiagnostics(t, tc.expectedErr, "invalid provider data type", resp.Diagnostics)
			}
			if tc.expectedInit {
				assert.NotNil(t, target.m1PlusVpcepService)
				assert.NotNil(t, target.m3VpcepService)
				assert.NotNil(t, target.lbmDnsService)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_Create(t *testing.T) {
	testCases := []struct {
		name string
		// Mock service
		endpointService *mockVpcepEndpointService
		vpcepService    *mockVpcepServiceService
		lbmDnsService   *mockLbmDnsService
		// 测试前置条件
		unknownPlan           bool
		patchRecordValueDiags bool
		// 期望的错误与警告
		expectedErr     string
		expectedWarning string
		// 期望的子资源调用次数
		expectedServiceCreateCalls  int
		expectedPermissionsCalls    int
		expectedEndpointCreateCalls int
		expectedDnsCreateCalls      int
		// 期望的回滚删除 ID
		expectedEndpointDeleteIds []string
		expectedServiceDeleteIds  []string
		// 期望的子资源调用入参
		expectedServiceInput      *service.VpcepServiceInput
		expectedPermissionsInput  []service.PermissionInput
		expectedEndpointInput     *service.VpcEndpointInput
		expectedCreateLbmDnsInput *service.CreateLbmDnsInput
		// 期望的 Terraform State 字段
		expectedLbmDnsRecordValues   []lbmDnsRecordValueBlock
		expectedStateVpcepServiceId  string
		expectedStateVpcepEndpointId string
		expectedStateLbmDnsRecordId  string
	}{
		{
			name: "GIVEN all child resources create successfully WHEN Create SHOULD write full state",
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
			},
			lbmDnsService: &mockLbmDnsService{
				createOutput: newCreateLbmDnsOutput(),
			},
			expectedServiceCreateCalls:   1,
			expectedPermissionsCalls:     1,
			expectedEndpointCreateCalls:  1,
			expectedDnsCreateCalls:       1,
			expectedServiceInput:         newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:     newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:        newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:    newExpectedM1ToM3LbmDnsInput(),
			expectedLbmDnsRecordValues:   []lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
			expectedStateVpcepServiceId:  testVpcepServiceId,
			expectedStateVpcepEndpointId: testVpcepEndpointId,
			expectedStateLbmDnsRecordId:  testLbmDnsRecordId,
		},
		{
			name:            "GIVEN unknown plan value WHEN Create SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService:    &mockVpcepServiceService{},
			lbmDnsService:   &mockLbmDnsService{},
			unknownPlan:     true,
			expectedErr:     "Value Conversion Error",
		},
		{
			name:            "GIVEN service create fails WHEN Create SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService: &mockVpcepServiceService{
				createErr: errors.New("create service failed"),
			},
			lbmDnsService:              &mockLbmDnsService{},
			expectedErr:                "create vpcep-service failed",
			expectedServiceCreateCalls: 1,
			expectedServiceInput:       newExpectedM1ToM3VpcepServiceInput(),
		},
		{
			name:            "GIVEN permission add fails WHEN Create SHOULD rollback service and return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
				addErr:          errors.New("add permission failed"),
			},
			lbmDnsService: &mockLbmDnsService{},
			expectedErr: fmt.Sprintf("add vpcep-service permission failed (vpcep_service_id: %s)",
				testVpcepServiceId),
			expectedServiceCreateCalls: 1,
			expectedPermissionsCalls:   1,
			expectedServiceInput:       newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:   newExpectedM1ToM3PermissionInputs(),
			expectedServiceDeleteIds:   []string{testVpcepServiceId},
		},
		{
			name: "GIVEN endpoint create fails WHEN Create SHOULD rollback service and return diagnostics",
			endpointService: &mockVpcepEndpointService{
				createErr: errors.New("create endpoint failed"),
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
			},
			lbmDnsService:               &mockLbmDnsService{},
			expectedErr:                 "create vpcep-endpoint failed",
			expectedServiceCreateCalls:  1,
			expectedPermissionsCalls:    1,
			expectedEndpointCreateCalls: 1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:    newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
		{
			name: "GIVEN dns create fails WHEN Create SHOULD rollback endpoint and service and return diagnostics",
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
			},
			lbmDnsService: &mockLbmDnsService{
				createErr: errors.New("create dns failed"),
			},
			expectedErr:                 "create lbm-dns record failed",
			expectedServiceCreateCalls:  1,
			expectedPermissionsCalls:    1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:    newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:   newExpectedM1ToM3LbmDnsInput(),
			expectedEndpointDeleteIds:   []string{testVpcepEndpointId},
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
		{
			name: "GIVEN dns create succeeds but record value build fails WHEN Create SHOULD rollback endpoint and service and return diagnostics",
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
			},
			lbmDnsService: &mockLbmDnsService{
				createOutput: newCreateLbmDnsOutput(),
			},
			patchRecordValueDiags:       true,
			expectedErr:                 "create lbm-dns record failed",
			expectedServiceCreateCalls:  1,
			expectedPermissionsCalls:    1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:    newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:   newExpectedM1ToM3LbmDnsInput(),
			expectedEndpointDeleteIds:   []string{testVpcepEndpointId},
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
		{
			name: "GIVEN dns create fails and rollback endpoint delete fails WHEN Create SHOULD return diagnostics with cleanup warning",
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
				deleteErr:        errors.New("delete endpoint failed"),
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
			},
			lbmDnsService: &mockLbmDnsService{
				createErr: errors.New("create dns failed"),
			},
			expectedErr:                 "create lbm-dns record failed",
			expectedWarning:             "manual cleanup may be required",
			expectedServiceCreateCalls:  1,
			expectedPermissionsCalls:    1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:    newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:   newExpectedM1ToM3LbmDnsInput(),
			expectedEndpointDeleteIds:   []string{testVpcepEndpointId},
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
		{
			name: "GIVEN dns create fails and rollback service delete fails WHEN Create SHOULD return diagnostics with cleanup warning",
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			vpcepService: &mockVpcepServiceService{
				createServiceId: testVpcepServiceId,
				deleteErr:       errors.New("delete service failed"),
			},
			lbmDnsService: &mockLbmDnsService{
				createErr: errors.New("create dns failed"),
			},
			expectedErr:                 "create lbm-dns record failed",
			expectedWarning:             "manual cleanup may be required",
			expectedServiceCreateCalls:  1,
			expectedPermissionsCalls:    1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissionsInput:    newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:   newExpectedM1ToM3LbmDnsInput(),
			expectedEndpointDeleteIds:   []string{testVpcepEndpointId},
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			target := newM1ToM3ResourceWithMocks(tc.endpointService, tc.vpcepService, tc.lbmDnsService)
			req := resource.CreateRequest{Plan: newM1ToM3Plan(t, newM1ToM3CreateModel())}
			if tc.unknownPlan {
				req.Plan = newUnknownM1ToM3Plan(t)
			}
			resp := &resource.CreateResponse{State: newM1ToM3State(t)}
			if tc.patchRecordValueDiags {
				patches := gomonkey.ApplyFunc(basetypes.NewObjectValue,
					func(_ map[string]attr.Type, _ map[string]attr.Value) (types.Object, diag.Diagnostics) {
						var diags diag.Diagnostics
						diags.AddError("object value failed", "mock record value diagnostics")
						return types.ObjectUnknown(lbmDnsRecordValueAttrTypes), diags
					})
				defer patches.Reset()
			}

			target.Create(ctx, req, resp)

			if tc.expectedErr == "" {
				assert.False(t, resp.Diagnostics.HasError())
				var actual netConnectM1ToM3Model
				diags := resp.State.Get(ctx, &actual)
				assert.False(t, diags.HasError(), "expected state get without diagnostics, got %v", diags)
				assert.Equal(t, tc.expectedStateVpcepServiceId, actual.VpcepServiceId.ValueString())
				assert.Equal(t, tc.expectedStateVpcepEndpointId, actual.VpcepEndpointId.ValueString())
				assert.Equal(t, testVpcepEndpointIp, actual.VpcepEndpointIp.ValueString())
				assert.Equal(t, tc.expectedStateVpcepServiceId, actual.VpcepEndpointServiceId.ValueString())
				assert.Equal(t, tc.expectedStateLbmDnsRecordId, actual.LbmDnsRecordId.ValueString())
				assertRecordValueList(t, tc.expectedLbmDnsRecordValues, actual.LbmDnsRecordValues)
			} else {
				assert.True(t, resp.Diagnostics.HasError())
				assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), tc.expectedErr)
			}
			if tc.expectedWarning != "" {
				if assert.Len(t, resp.Diagnostics.Warnings(), 1) {
					assert.Contains(t, resp.Diagnostics.Warnings()[0].Summary(), tc.expectedWarning)
				}
			}
			assert.Len(t, tc.vpcepService.createInputs, tc.expectedServiceCreateCalls)
			if tc.expectedServiceInput != nil && assert.NotEmpty(t, tc.vpcepService.createInputs) {
				assert.Equal(t, *tc.expectedServiceInput, tc.vpcepService.createInputs[0])
			}
			assert.Len(t, tc.vpcepService.addServiceIds, tc.expectedPermissionsCalls)
			if tc.expectedPermissionsInput != nil && assert.NotEmpty(t, tc.vpcepService.addPermissions) {
				assert.Equal(t, testVpcepServiceId, tc.vpcepService.addServiceIds[0])
				assert.Equal(t, tc.expectedPermissionsInput, tc.vpcepService.addPermissions[0])
			}
			assert.Len(t, tc.endpointService.createInputs, tc.expectedEndpointCreateCalls)
			if tc.expectedEndpointInput != nil && assert.NotEmpty(t, tc.endpointService.createInputs) {
				assert.Equal(t, *tc.expectedEndpointInput, tc.endpointService.createInputs[0])
			}
			assert.Len(t, tc.lbmDnsService.createInputs, tc.expectedDnsCreateCalls)
			if tc.expectedCreateLbmDnsInput != nil && assert.NotEmpty(t, tc.lbmDnsService.createInputs) {
				assert.Equal(t, *tc.expectedCreateLbmDnsInput, tc.lbmDnsService.createInputs[0])
			}
			assert.Equal(t, tc.expectedEndpointDeleteIds, tc.endpointService.deleteIds)
			assert.Equal(t, tc.expectedServiceDeleteIds, tc.vpcepService.deleteIds)
		})
	}
}

func Test_netConnectM1ToM3Resource_Read(t *testing.T) {
	testCases := []struct {
		name string
		// Mock service
		endpointService *mockVpcepEndpointService
		vpcepService    *mockVpcepServiceService
		lbmDnsService   *mockLbmDnsService
		// 测试前置条件
		unknownState bool
		// 期望的子资源 Get 调用次数
		expectedServiceGetCalls  int
		expectedEndpointGetCalls int
		expectedDnsGetCalls      int
		expectedPermissionsCalls int
		// 期望的子资源 Get 调用入参 ID
		expectedServiceGetId     string
		expectedEndpointGetId    string
		expectedLbmDnsGetId      string
		expectedPermissionsGetId string
		// 期望的错误与状态
		expectedErr     string
		expectedRemoved bool
		expectedState   *netConnectM1ToM3Model
	}{
		{
			name: "GIVEN all child resources exist WHEN Read SHOULD sync state from remote",
			endpointService: &mockVpcepEndpointService{
				getOutput: &service.VpcepEndpointOutput{
					EndpointId: testVpcepEndpointId,
					Ip:         testVpcepEndpointIp,
					VpcId:      testM1PlusVpcId,
					SubnetId:   testM1PlusSubnetId,
					ServiceId:  testVpcepServiceId,
				},
			},
			vpcepService: &mockVpcepServiceService{
				getOutput: &service.VpcepServiceOutput{
					ServiceId:  testVpcepServiceId,
					VpcId:      testM3VpcId,
					PortId:     testM3PortId,
					ServerType: testM3ServerType,
					Ports:      []service.PortPair{{ClientPort: 443, ServerPort: 8443}},
				},
				getPermissionsOutput: map[string]string{
					testIamDomainId: testPermissionId,
				},
			},
			lbmDnsService: &mockLbmDnsService{
				getDetailOutput: &service.LbmDnsDetailOutput{
					RecordId:     testLbmDnsRecordId,
					RegionCode:   testRegionCode,
					ServiceName:  testLbmDnsServiceName,
					HostRecord:   testDnsDomain,
					DomainSuffix: testDnsDomainSuffix,
					RecordValues: []service.LbmDnsRecordValue{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
				},
			},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M3VpcId = testM3VpcId
				expected.M3PortId = testM3PortId
				expected.M3ServerType = testM3ServerType
				expected.M3VpcepServicePorts = []vpcepServicePortBlock{{ClientPort: 443, ServerPort: 8443}}
				expected.M3VpcepServicePermissions = []vpcepServicePermissionBlock{
					{Permission: testIamDomainId},
				}
				expected.VpcepEndpointIp = types.StringValue(testVpcepEndpointIp)
				expected.M1PlusVpcId = testM1PlusVpcId
				expected.M1PlusSubnetId = testM1PlusSubnetId
				expected.RegionCode = testRegionCode
				expected.LbmDnsServiceName = testLbmDnsServiceName
				expected.DnsDomain = testDnsDomain
				expected.DnsDomainSuffix = testDnsDomainSuffix
				expected.LbmDnsRecordValues = testLbmDnsRecordValues(
					[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}})
				return &expected
			}(),
		},
		{
			name:                     "GIVEN all child resources are missing WHEN Read SHOULD remove resource from state",
			endpointService:          &mockVpcepEndpointService{},
			vpcepService:             &mockVpcepServiceService{},
			lbmDnsService:            &mockLbmDnsService{},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedRemoved:          true,
		},
		{
			name: "GIVEN service is missing but other child resources exist WHEN Read SHOULD keep partial state",
			endpointService: &mockVpcepEndpointService{
				getOutput: &service.VpcepEndpointOutput{
					EndpointId: testVpcepEndpointId,
					Ip:         testVpcepEndpointIp,
					ServiceId:  testVpcepServiceId,
				},
			},
			vpcepService: &mockVpcepServiceService{},
			lbmDnsService: &mockLbmDnsService{
				getDetailOutput: &service.LbmDnsDetailOutput{
					RecordId:     testLbmDnsRecordId,
					RecordValues: []service.LbmDnsRecordValue{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
				},
			},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M1PlusVpcId = testM1PlusVpcId
				expected.M1PlusSubnetId = testM1PlusSubnetId
				expected.VpcepServiceId = types.StringNull()
				clearM1ToM3ServiceInputState(&expected)
				expected.RegionCode = ""
				expected.LbmDnsServiceName = ""
				expected.DnsDomain = ""
				expected.DnsDomainSuffix = ""
				return &expected
			}(),
		},
		{
			name:            "GIVEN endpoint is missing but other child resources exist WHEN Read SHOULD keep partial state",
			endpointService: &mockVpcepEndpointService{},
			vpcepService: &mockVpcepServiceService{
				getOutput: &service.VpcepServiceOutput{
					ServiceId:  testVpcepServiceId,
					VpcId:      testM3VpcId,
					PortId:     testM3PortId,
					ServerType: testM3ServerType,
					Ports:      []service.PortPair{{ClientPort: 443, ServerPort: 8443}},
				},
				getPermissionsOutput: map[string]string{testIamDomainId: testPermissionId},
			},
			lbmDnsService: &mockLbmDnsService{
				getDetailOutput: &service.LbmDnsDetailOutput{
					RecordId:     testLbmDnsRecordId,
					RegionCode:   testRegionCode,
					ServiceName:  testLbmDnsServiceName,
					HostRecord:   testDnsDomain,
					DomainSuffix: testDnsDomainSuffix,
					RecordValues: []service.LbmDnsRecordValue{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
				},
			},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M3VpcId = testM3VpcId
				expected.M3PortId = testM3PortId
				expected.M3ServerType = testM3ServerType
				expected.M3VpcepServicePorts = []vpcepServicePortBlock{{ClientPort: 443, ServerPort: 8443}}
				expected.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: testIamDomainId}}
				expected.RegionCode = testRegionCode
				expected.LbmDnsServiceName = testLbmDnsServiceName
				expected.DnsDomain = testDnsDomain
				expected.DnsDomainSuffix = testDnsDomainSuffix
				expected.LbmDnsRecordValues = testLbmDnsRecordValues(
					[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}})
				expected.VpcepEndpointId = types.StringNull()
				expected.VpcepEndpointIp = types.StringNull()
				expected.VpcepEndpointServiceId = types.StringNull()
				clearM1ToM3EndpointInputState(&expected)
				return &expected
			}(),
		},
		{
			name: "GIVEN lbm dns record is missing but other child resources exist WHEN Read SHOULD keep partial state",
			endpointService: &mockVpcepEndpointService{
				getOutput: &service.VpcepEndpointOutput{
					EndpointId: testVpcepEndpointId,
					Ip:         testVpcepEndpointIp,
					VpcId:      testM1PlusVpcId,
					SubnetId:   testM1PlusSubnetId,
					ServiceId:  testVpcepServiceId,
				},
			},
			vpcepService: &mockVpcepServiceService{
				getOutput: &service.VpcepServiceOutput{
					ServiceId:  testVpcepServiceId,
					VpcId:      testM3VpcId,
					PortId:     testM3PortId,
					ServerType: testM3ServerType,
					Ports:      []service.PortPair{{ClientPort: 443, ServerPort: 8443}},
				},
				getPermissionsOutput: map[string]string{testIamDomainId: testPermissionId},
			},
			lbmDnsService:            &mockLbmDnsService{},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M3VpcId = testM3VpcId
				expected.M3PortId = testM3PortId
				expected.M3ServerType = testM3ServerType
				expected.M3VpcepServicePorts = []vpcepServicePortBlock{{ClientPort: 443, ServerPort: 8443}}
				expected.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: testIamDomainId}}
				expected.VpcepEndpointIp = types.StringValue(testVpcepEndpointIp)
				expected.M1PlusVpcId = testM1PlusVpcId
				expected.M1PlusSubnetId = testM1PlusSubnetId
				expected.LbmDnsRecordId = types.StringNull()
				expected.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
				clearM1ToM3DnsInputState(&expected)
				return &expected
			}(),
		},
		{
			name:            "GIVEN unknown state values WHEN Read SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService:    &mockVpcepServiceService{},
			lbmDnsService:   &mockLbmDnsService{},
			unknownState:    true,
			expectedErr:     "Value Conversion Error",
		},
		{
			name:            "GIVEN service query fails WHEN Read SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService: &mockVpcepServiceService{
				getErr: errors.New("query service failed"),
			},
			lbmDnsService:           &mockLbmDnsService{},
			expectedServiceGetCalls: 1,
			expectedServiceGetId:    testVpcepServiceId,
			expectedErr:             "query vpcep-service failed",
		},
		{
			name: "GIVEN endpoint query fails WHEN Read SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{
				getErr: errors.New("query endpoint failed"),
			},
			vpcepService: &mockVpcepServiceService{
				getOutput: &service.VpcepServiceOutput{ServiceId: testVpcepServiceId},
			},
			lbmDnsService:            &mockLbmDnsService{},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedErr:              "query vpcep-endpoint failed",
		},
		{
			name: "GIVEN dns query fails WHEN Read SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{
				getOutput: &service.VpcepEndpointOutput{EndpointId: testVpcepEndpointId},
			},
			vpcepService: &mockVpcepServiceService{
				getOutput: &service.VpcepServiceOutput{ServiceId: testVpcepServiceId},
			},
			lbmDnsService: &mockLbmDnsService{
				getDetailErr: errors.New("query dns failed"),
			},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 1,
			expectedDnsGetCalls:      1,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedLbmDnsGetId:      testLbmDnsRecordId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedErr:              "query lbm-dns record failed",
		},
		{
			name: "GIVEN permission query fails WHEN Read SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{
				getOutput: &service.VpcepEndpointOutput{EndpointId: testVpcepEndpointId},
			},
			vpcepService: &mockVpcepServiceService{
				getOutput:         &service.VpcepServiceOutput{ServiceId: testVpcepServiceId},
				getPermissionsErr: errors.New("query permissions failed"),
			},
			lbmDnsService:            &mockLbmDnsService{},
			expectedServiceGetCalls:  1,
			expectedEndpointGetCalls: 0,
			expectedDnsGetCalls:      0,
			expectedPermissionsCalls: 1,
			expectedServiceGetId:     testVpcepServiceId,
			expectedEndpointGetId:    "",
			expectedLbmDnsGetId:      "",
			expectedPermissionsGetId: testVpcepServiceId,
			expectedErr:              "query vpcep-service failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			resourceWithMocks := newM1ToM3ResourceWithMocks(tc.endpointService, tc.vpcepService, tc.lbmDnsService)
			req := resource.ReadRequest{State: newM1ToM3StateWithModel(t, newM1ToM3Model())}
			if tc.unknownState {
				req.State = newUnknownM1ToM3State(t)
			}
			resp := &resource.ReadResponse{State: newM1ToM3State(t)}

			resourceWithMocks.Read(ctx, req, resp)

			assert.Equal(t, tc.expectedServiceGetCalls, tc.vpcepService.getCalls)
			assert.Equal(t, tc.expectedEndpointGetCalls, tc.endpointService.getCalls)
			assert.Equal(t, tc.expectedDnsGetCalls, tc.lbmDnsService.getDetailCalls)
			assert.Equal(t, tc.expectedPermissionsCalls, tc.vpcepService.getPermissionsCalls)
			// 调用 ID 断言前置到错误分支之前；未触发的 Get 调用会保留空字符串零值，可以同时覆盖短路场景下“不应继续查询后续子资源”的行为
			assert.Equal(t, tc.expectedServiceGetId, tc.vpcepService.getId)
			assert.Equal(t, tc.expectedEndpointGetId, tc.endpointService.getId)
			assert.Equal(t, tc.expectedLbmDnsGetId, tc.lbmDnsService.getDetailId)
			assert.Equal(t, tc.expectedPermissionsGetId, tc.vpcepService.getPermissionsId)
			if tc.expectedErr != "" {
				require.True(t, resp.Diagnostics.HasError())
				require.NotEmpty(t, resp.Diagnostics.Errors())
				assert.Equal(t, tc.expectedErr, resp.Diagnostics.Errors()[0].Summary())
				return
			}
			assert.False(t, resp.Diagnostics.HasError())
			if tc.expectedRemoved {
				assert.True(t, resp.State.Raw.IsNull())
				return
			}

			var actual netConnectM1ToM3Model
			diags := resp.State.Get(ctx, &actual)
			require.False(t, diags.HasError(), "expected state get without diagnostics, got %v", diags)
			if tc.expectedState != nil {
				assertM1ToM3State(t, *tc.expectedState, actual)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_refreshVpcepServiceState(t *testing.T) {
	testCases := []struct {
		name                     string
		output                   *service.VpcepServiceOutput
		err                      error
		permissions              map[string]string
		permissionsErr           error
		state                    netConnectM1ToM3Model
		expectedErrMsg           string
		expectedState            *netConnectM1ToM3Model
		expectedServiceGetId     string
		expectedPermissionsGetId string
		expectedServiceGetCalls  int
		expectedPermissionsCalls int
	}{
		{
			name: "GIVEN null service id WHEN refreshVpcepServiceState SHOULD skip query",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepServiceId = types.StringNull()
				return state
			}(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepServiceId = types.StringNull()
				return &expected
			}(),
		},
		{
			name: "GIVEN service exists WHEN refreshVpcepServiceState SHOULD sync service fields",
			output: &service.VpcepServiceOutput{
				ServiceId:  testVpcepServiceId,
				VpcId:      testM3VpcId,
				PortId:     testM3PortId,
				ServerType: testM3ServerType,
				Ports:      []service.PortPair{{ClientPort: 443, ServerPort: 8443}},
			},
			permissions: map[string]string{testIamDomainId: testPermissionId},
			state:       newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M3VpcId = testM3VpcId
				expected.M3PortId = testM3PortId
				expected.M3ServerType = testM3ServerType
				expected.M3VpcepServicePorts = []vpcepServicePortBlock{{ClientPort: 443, ServerPort: 8443}}
				expected.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: testIamDomainId}}
				return &expected
			}(),
			expectedServiceGetId:     testVpcepServiceId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedServiceGetCalls:  1,
			expectedPermissionsCalls: 1,
		},
		{
			name:  "GIVEN service is missing WHEN refreshVpcepServiceState SHOULD clear service state",
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepServiceId = types.StringNull()
				clearM1ToM3ServiceInputState(&expected)
				return &expected
			}(),
			expectedServiceGetId:    testVpcepServiceId,
			expectedServiceGetCalls: 1,
		},
		{
			name:                    "GIVEN service query fails WHEN refreshVpcepServiceState SHOULD return error",
			err:                     errors.New("query service failed"),
			state:                   newM1ToM3Model(),
			expectedErrMsg:          "query service failed",
			expectedServiceGetId:    testVpcepServiceId,
			expectedServiceGetCalls: 1,
		},
		{
			name:                     "GIVEN permission query fails WHEN refreshVpcepServiceState SHOULD return wrapped error",
			output:                   &service.VpcepServiceOutput{ServiceId: testVpcepServiceId},
			permissionsErr:           errors.New("query permission failed"),
			state:                    newM1ToM3Model(),
			expectedErrMsg:           "query vpcep-service permission failed: query permission failed",
			expectedServiceGetId:     testVpcepServiceId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedServiceGetCalls:  1,
			expectedPermissionsCalls: 1,
		},
		{
			name:        "GIVEN service exists with empty permissions WHEN refreshVpcepServiceState SHOULD sync service fields with empty permissions",
			output:      &service.VpcepServiceOutput{ServiceId: testVpcepServiceId},
			permissions: map[string]string{},
			state:       newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M3VpcepServicePermissions = nil
				return &expected
			}(),
			expectedServiceGetId:     testVpcepServiceId,
			expectedPermissionsGetId: testVpcepServiceId,
			expectedServiceGetCalls:  1,
			expectedPermissionsCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vpcepService := &mockVpcepServiceService{
				getOutput:            tc.output,
				getErr:               tc.err,
				getPermissionsOutput: tc.permissions,
				getPermissionsErr:    tc.permissionsErr,
			}
			target := newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, vpcepService, &mockLbmDnsService{})
			state := tc.state

			err := target.refreshVpcepServiceState(context.Background(), &state)

			assert.Equal(t, tc.expectedServiceGetCalls, vpcepService.getCalls)
			assert.Equal(t, tc.expectedPermissionsCalls, vpcepService.getPermissionsCalls)
			assert.Equal(t, tc.expectedServiceGetId, vpcepService.getId)
			assert.Equal(t, tc.expectedPermissionsGetId, vpcepService.getPermissionsId)
			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			if tc.expectedState != nil {
				assertM1ToM3State(t, *tc.expectedState, state)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_refreshVpcepEndpointState(t *testing.T) {
	testCases := []struct {
		name                     string
		output                   *service.VpcepEndpointOutput
		err                      error
		state                    netConnectM1ToM3Model
		expectedErrMsg           string
		expectedState            *netConnectM1ToM3Model
		expectedEndpointGetId    string
		expectedEndpointGetCalls int
	}{
		{
			name: "GIVEN null endpoint id WHEN refreshVpcepEndpointState SHOULD skip query",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointId = types.StringNull()
				return state
			}(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepEndpointId = types.StringNull()
				return &expected
			}(),
		},
		{
			name: "GIVEN endpoint exists WHEN refreshVpcepEndpointState SHOULD sync endpoint fields",
			output: &service.VpcepEndpointOutput{
				EndpointId: testVpcepEndpointId,
				Ip:         testVpcepEndpointIp,
				VpcId:      testM1PlusVpcId,
				SubnetId:   testM1PlusSubnetId,
				ServiceId:  testVpcepServiceId,
			},
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepEndpointIp = types.StringValue(testVpcepEndpointIp)
				expected.M1PlusVpcId = testM1PlusVpcId
				expected.M1PlusSubnetId = testM1PlusSubnetId
				expected.VpcepEndpointServiceId = types.StringValue(testVpcepServiceId)
				return &expected
			}(),
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedEndpointGetCalls: 1,
		},
		{
			name:  "GIVEN endpoint is missing WHEN refreshVpcepEndpointState SHOULD clear endpoint state",
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepEndpointId = types.StringNull()
				expected.VpcepEndpointIp = types.StringNull()
				expected.VpcepEndpointServiceId = types.StringNull()
				clearM1ToM3EndpointInputState(&expected)
				return &expected
			}(),
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedEndpointGetCalls: 1,
		},
		{
			name:                     "GIVEN endpoint query fails WHEN refreshVpcepEndpointState SHOULD return error",
			err:                      errors.New("query endpoint failed"),
			state:                    newM1ToM3Model(),
			expectedErrMsg:           "query endpoint failed",
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedEndpointGetCalls: 1,
		},
		{
			name: "GIVEN endpoint exists with empty ip WHEN refreshVpcepEndpointState SHOULD keep original ip value",
			output: &service.VpcepEndpointOutput{
				EndpointId: testVpcepEndpointId,
				Ip:         "",
				VpcId:      testM1PlusVpcId,
				SubnetId:   testM1PlusSubnetId,
				ServiceId:  "",
			},
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.M1PlusVpcId = testM1PlusVpcId
				expected.M1PlusSubnetId = testM1PlusSubnetId
				return &expected
			}(),
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedEndpointGetCalls: 1,
		},
		{
			name: "GIVEN endpoint exists with empty vpc and subnet WHEN refreshVpcepEndpointState SHOULD keep original network values",
			output: &service.VpcepEndpointOutput{
				EndpointId: testVpcepEndpointId,
				Ip:         testVpcepEndpointIp,
				VpcId:      "",
				SubnetId:   "",
				ServiceId:  testVpcepServiceId,
			},
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.VpcepEndpointIp = types.StringValue(testVpcepEndpointIp)
				expected.VpcepEndpointServiceId = types.StringValue(testVpcepServiceId)
				return &expected
			}(),
			expectedEndpointGetId:    testVpcepEndpointId,
			expectedEndpointGetCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpointService := &mockVpcepEndpointService{
				getOutput: tc.output,
				getErr:    tc.err,
			}
			target := newM1ToM3ResourceWithMocks(endpointService, &mockVpcepServiceService{}, &mockLbmDnsService{})
			state := tc.state

			err := target.refreshVpcepEndpointState(context.Background(), &state)

			assert.Equal(t, tc.expectedEndpointGetCalls, endpointService.getCalls)
			assert.Equal(t, tc.expectedEndpointGetId, endpointService.getId)
			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			if tc.expectedState != nil {
				assertM1ToM3State(t, *tc.expectedState, state)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_refreshLbmDnsState(t *testing.T) {
	testCases := []struct {
		name                   string
		output                 *service.LbmDnsDetailOutput
		err                    error
		state                  netConnectM1ToM3Model
		expectedErrMsg         string
		expectedErrDetail      string
		expectedState          *netConnectM1ToM3Model
		expectedLbmDnsGetId    string
		expectedLbmDnsGetCalls int
	}{
		{
			name: "GIVEN null dns record id WHEN refreshLbmDnsState SHOULD skip query",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.LbmDnsRecordId = types.StringNull()
				return state
			}(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.LbmDnsRecordId = types.StringNull()
				return &expected
			}(),
		},
		{
			name: "GIVEN dns record exists WHEN refreshLbmDnsState SHOULD sync dns fields",
			output: &service.LbmDnsDetailOutput{
				RecordId:     testLbmDnsRecordId,
				RegionCode:   testRegionCode,
				ServiceName:  testLbmDnsServiceName,
				HostRecord:   testDnsDomain,
				DomainSuffix: testDnsDomainSuffix,
				RecordValues: []service.LbmDnsRecordValue{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
			},
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.RegionCode = testRegionCode
				expected.LbmDnsServiceName = testLbmDnsServiceName
				expected.DnsDomain = testDnsDomain
				expected.DnsDomainSuffix = testDnsDomainSuffix
				expected.LbmDnsRecordValues = testLbmDnsRecordValues(
					[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}})
				return &expected
			}(),
			expectedLbmDnsGetId:    testLbmDnsRecordId,
			expectedLbmDnsGetCalls: 1,
		},
		{
			name:  "GIVEN dns record is missing WHEN refreshLbmDnsState SHOULD clear dns state",
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.LbmDnsRecordId = types.StringNull()
				expected.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
				clearM1ToM3DnsInputState(&expected)
				return &expected
			}(),
			expectedLbmDnsGetId:    testLbmDnsRecordId,
			expectedLbmDnsGetCalls: 1,
		},
		{
			name:                   "GIVEN dns query fails WHEN refreshLbmDnsState SHOULD return diagnostics",
			err:                    errors.New("query dns failed"),
			state:                  newM1ToM3Model(),
			expectedErrMsg:         "query lbm-dns record failed",
			expectedErrDetail:      "query dns failed",
			expectedLbmDnsGetId:    testLbmDnsRecordId,
			expectedLbmDnsGetCalls: 1,
		},
		{
			name: "GIVEN dns record exists with empty record values WHEN refreshLbmDnsState SHOULD sync dns fields with empty record values",
			output: &service.LbmDnsDetailOutput{
				RecordId:     testLbmDnsRecordId,
				RegionCode:   testRegionCode,
				ServiceName:  testLbmDnsServiceName,
				HostRecord:   testDnsDomain,
				DomainSuffix: testDnsDomainSuffix,
			},
			state: newM1ToM3Model(),
			expectedState: func() *netConnectM1ToM3Model {
				expected := newM1ToM3Model()
				expected.RegionCode = testRegionCode
				expected.LbmDnsServiceName = testLbmDnsServiceName
				expected.DnsDomain = testDnsDomain
				expected.DnsDomainSuffix = testDnsDomainSuffix
				expected.LbmDnsRecordValues = testLbmDnsRecordValues([]lbmDnsRecordValueBlock{})
				return &expected
			}(),
			expectedLbmDnsGetId:    testLbmDnsRecordId,
			expectedLbmDnsGetCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lbmDnsService := &mockLbmDnsService{
				getDetailOutput: tc.output,
				getDetailErr:    tc.err,
			}
			target := newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, &mockVpcepServiceService{},
				lbmDnsService)
			state := tc.state

			diags := target.refreshLbmDnsState(context.Background(), &state)

			assert.Equal(t, tc.expectedLbmDnsGetCalls, lbmDnsService.getDetailCalls)
			assert.Equal(t, tc.expectedLbmDnsGetId, lbmDnsService.getDetailId)
			if tc.expectedErrMsg != "" {
				assertDiagnostics(t, tc.expectedErrMsg, tc.expectedErrDetail, diags)
				return
			}
			assert.False(t, diags.HasError())
			assert.Empty(t, diags)
			if tc.expectedState != nil {
				assertM1ToM3State(t, *tc.expectedState, state)
			}
		})
	}
}

func Test_requiredM1ToM3StringAttribute(t *testing.T) {
	testCases := []struct {
		name                        string
		expectedRequired            bool
		expectedValidatorsLength    int
		expectedPlanModifiersLength int
	}{
		{
			name:                        "GIVEN string attribute helper WHEN requiredM1ToM3StringAttribute SHOULD return required attribute with validator",
			expectedRequired:            true,
			expectedValidatorsLength:    1,
			expectedPlanModifiersLength: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := requiredM1ToM3StringAttribute()

			assert.Equal(t, tc.expectedRequired, actual.Required)
			assert.Len(t, actual.Validators, tc.expectedValidatorsLength)
			assert.Len(t, actual.PlanModifiers, tc.expectedPlanModifiersLength)
		})
	}
}

func Test_requiredM1ToM3RootStringAttribute(t *testing.T) {
	testCases := []struct {
		name                        string
		expectedRequired            bool
		expectedValidatorsLength    int
		expectedPlanModifiersLength int
	}{
		{
			name:                        "GIVEN root string attribute helper WHEN requiredM1ToM3RootStringAttribute SHOULD return required attribute with replace modifier",
			expectedRequired:            true,
			expectedValidatorsLength:    1,
			expectedPlanModifiersLength: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := requiredM1ToM3RootStringAttribute()

			assert.Equal(t, tc.expectedRequired, actual.Required)
			assert.Len(t, actual.Validators, tc.expectedValidatorsLength)
			assert.Len(t, actual.PlanModifiers, tc.expectedPlanModifiersLength)
		})
	}
}

func Test_requiredM1ToM3PortAttribute(t *testing.T) {
	testCases := []struct {
		name               string
		expectedRequired   bool
		expectedValidators int
	}{
		{
			name:               "GIVEN port attribute helper WHEN requiredM1ToM3PortAttribute SHOULD return required attribute with port validator",
			expectedRequired:   true,
			expectedValidators: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := requiredM1ToM3PortAttribute()

			assert.Equal(t, tc.expectedRequired, actual.Required)
			assert.Len(t, actual.Validators, tc.expectedValidators)
		})
	}
}

func Test_normalizeM1ToM3ListState(t *testing.T) {
	testCases := []struct {
		name                string
		inputPorts          []vpcepServicePortBlock
		inputPermissions    []vpcepServicePermissionBlock
		expectedPorts       []vpcepServicePortBlock
		expectedPermissions []vpcepServicePermissionBlock
	}{
		{
			name: "GIVEN unsorted ports and permissions WHEN normalizeM1ToM3ListState SHOULD sort both lists",
			inputPorts: []vpcepServicePortBlock{
				{ClientPort: 443, ServerPort: 8443},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 80, ServerPort: 8080},
			},
			inputPermissions: []vpcepServicePermissionBlock{
				{Permission: "z-account"},
				{Permission: "a-account"},
			},
			expectedPorts: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 443, ServerPort: 8443},
			},
			expectedPermissions: []vpcepServicePermissionBlock{
				{Permission: "a-account"},
				{Permission: "z-account"},
			},
		},
		{
			name:                "GIVEN empty lists WHEN normalizeM1ToM3ListState SHOULD keep empty lists",
			inputPorts:          []vpcepServicePortBlock{},
			inputPermissions:    []vpcepServicePermissionBlock{},
			expectedPorts:       nil,
			expectedPermissions: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := &netConnectM1ToM3Model{
				M3VpcepServicePorts:       tc.inputPorts,
				M3VpcepServicePermissions: tc.inputPermissions,
			}

			normalizeM1ToM3ListState(state)

			assert.Equal(t, tc.expectedPorts, state.M3VpcepServicePorts)
			assert.Equal(t, tc.expectedPermissions, state.M3VpcepServicePermissions)
		})
	}
}

func Test_normalizePortPairs(t *testing.T) {
	testCases := []struct {
		name     string
		input    []service.PortPair
		expected []vpcepServicePortBlock
	}{
		{
			name: "GIVEN service port pairs WHEN normalizePortPairs SHOULD convert and sort ports",
			input: []service.PortPair{
				{ClientPort: 443, ServerPort: 8443},
				{ClientPort: 80, ServerPort: 8080},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN sorted service port pairs WHEN normalizePortPairs SHOULD keep sorted ports",
			input: []service.PortPair{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN service port pairs with same client port WHEN normalizePortPairs SHOULD sort by server port",
			input: []service.PortPair{
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN duplicate service port pairs WHEN normalizePortPairs SHOULD keep duplicate ports",
			input: []service.PortPair{
				{ClientPort: 443, ServerPort: 8443},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8080},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name:     "GIVEN empty service port pairs WHEN normalizePortPairs SHOULD return empty ports",
			input:    []service.PortPair{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizePortPairs(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_normalizeVpcepServicePortBlocks(t *testing.T) {
	testCases := []struct {
		name     string
		input    []vpcepServicePortBlock
		expected []vpcepServicePortBlock
	}{
		{
			name: "GIVEN unsorted vpcep service ports WHEN normalizeVpcepServicePortBlocks SHOULD sort by client port then server port",
			input: []vpcepServicePortBlock{
				{ClientPort: 443, ServerPort: 8443},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 80, ServerPort: 8080},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN sorted vpcep service ports WHEN normalizeVpcepServicePortBlocks SHOULD keep sorted ports",
			input: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN vpcep service ports with same client port WHEN normalizeVpcepServicePortBlocks SHOULD sort by server port",
			input: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8081},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name: "GIVEN duplicate vpcep service ports WHEN normalizeVpcepServicePortBlocks SHOULD keep duplicate ports",
			input: []vpcepServicePortBlock{
				{ClientPort: 443, ServerPort: 8443},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8080},
			},
			expected: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name:     "GIVEN empty vpcep service ports WHEN normalizeVpcepServicePortBlocks SHOULD return empty ports",
			input:    []vpcepServicePortBlock{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizeVpcepServicePortBlocks(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_buildLbmDnsRecordValues(t *testing.T) {
	testCases := []struct {
		name     string
		values   []lbmDnsRecordValueBlock
		expected []lbmDnsRecordValueBlock
	}{
		{
			name: "GIVEN unsorted record values WHEN buildLbmDnsRecordValues SHOULD return normalized Terraform list",
			values: []lbmDnsRecordValueBlock{
				{RecordType: "CNAME", RecordValue: "api.example.com"},
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
			},
			expected: []lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "CNAME", RecordValue: "api.example.com"},
			},
		},
		{
			name:     "GIVEN empty record values WHEN buildLbmDnsRecordValues SHOULD return empty Terraform list",
			values:   []lbmDnsRecordValueBlock{},
			expected: []lbmDnsRecordValueBlock{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, diags := buildLbmDnsRecordValues(tc.values)

			if diags.HasError() {
				t.Fatalf("expected no diagnostics, got %v", diags)
			}
			assertRecordValueList(t, tc.expected, actual)
		})
	}
}

func Test_normalizeLbmDnsRecordValueBlocks(t *testing.T) {
	testCases := []struct {
		name     string
		input    []lbmDnsRecordValueBlock
		expected []lbmDnsRecordValueBlock
	}{
		{
			name: "GIVEN unsorted lbm dns record values WHEN normalizeLbmDnsRecordValueBlocks SHOULD sort by type then value",
			input: []lbmDnsRecordValueBlock{
				{RecordType: "CNAME", RecordValue: "b.example.com"},
				{RecordType: "A", RecordValue: "10.0.0.9"},
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
			},
			expected: []lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "A", RecordValue: "10.0.0.9"},
				{RecordType: "CNAME", RecordValue: "b.example.com"},
			},
		},
		{
			name: "GIVEN sorted lbm dns record values WHEN normalizeLbmDnsRecordValueBlocks SHOULD keep sorted values",
			input: []lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "CNAME", RecordValue: "b.example.com"},
			},
			expected: []lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "CNAME", RecordValue: "b.example.com"},
			},
		},
		{
			name: "GIVEN duplicate lbm dns record values WHEN normalizeLbmDnsRecordValueBlocks SHOULD keep duplicate values",
			input: []lbmDnsRecordValueBlock{
				{RecordType: "CNAME", RecordValue: "b.example.com"},
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
			},
			expected: []lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
				{RecordType: "CNAME", RecordValue: "b.example.com"},
			},
		},
		{
			name:     "GIVEN empty lbm dns record values WHEN normalizeLbmDnsRecordValueBlocks SHOULD return empty values",
			input:    []lbmDnsRecordValueBlock{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizeLbmDnsRecordValueBlocks(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_normalizeVpcepServicePermissionBlocks(t *testing.T) {
	testCases := []struct {
		name     string
		input    []vpcepServicePermissionBlock
		expected []vpcepServicePermissionBlock
	}{
		{
			name: "GIVEN unsorted vpcep service permissions WHEN normalizeVpcepServicePermissionBlocks SHOULD sort permissions",
			input: []vpcepServicePermissionBlock{
				{Permission: "z-account"},
				{Permission: "a-account"},
			},
			expected: []vpcepServicePermissionBlock{
				{Permission: "a-account"},
				{Permission: "z-account"},
			},
		},
		{
			name: "GIVEN sorted vpcep service permissions WHEN normalizeVpcepServicePermissionBlocks SHOULD keep sorted permissions",
			input: []vpcepServicePermissionBlock{
				{Permission: "a-account"},
				{Permission: "z-account"},
			},
			expected: []vpcepServicePermissionBlock{
				{Permission: "a-account"},
				{Permission: "z-account"},
			},
		},
		{
			name: "GIVEN duplicate vpcep service permissions WHEN normalizeVpcepServicePermissionBlocks SHOULD keep duplicate permissions",
			input: []vpcepServicePermissionBlock{
				{Permission: "z-account"},
				{Permission: "a-account"},
				{Permission: "a-account"},
			},
			expected: []vpcepServicePermissionBlock{
				{Permission: "a-account"},
				{Permission: "a-account"},
				{Permission: "z-account"},
			},
		},
		{
			name:     "GIVEN empty vpcep service permissions WHEN normalizeVpcepServicePermissionBlocks SHOULD return empty permissions",
			input:    []vpcepServicePermissionBlock{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizeVpcepServicePermissionBlocks(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_setM1ToM3UpdateState(t *testing.T) {
	testCases := []struct {
		name                string
		plan                netConnectM1ToM3Model
		expectedPorts       []vpcepServicePortBlock
		expectedPermissions []vpcepServicePermissionBlock
	}{
		{
			name: "GIVEN update plan WHEN setM1ToM3UpdateState SHOULD normalize list fields and set state",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePorts = []vpcepServicePortBlock{
					{ClientPort: 443, ServerPort: 8443},
					{ClientPort: 80, ServerPort: 8080},
				}
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{
					{Permission: testAnotherIamDomainId},
					{Permission: testIamDomainId},
				}
				return plan
			}(),
			expectedPorts: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expectedPermissions: []vpcepServicePermissionBlock{
				{Permission: testIamDomainId},
				{Permission: testAnotherIamDomainId},
			},
		},
		{
			name: "GIVEN sorted update plan WHEN setM1ToM3UpdateState SHOULD keep normalized list fields",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePorts = []vpcepServicePortBlock{
					{ClientPort: 80, ServerPort: 8080},
					{ClientPort: 443, ServerPort: 8443},
				}
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{
					{Permission: testIamDomainId},
					{Permission: testAnotherIamDomainId},
				}
				return plan
			}(),
			expectedPorts: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expectedPermissions: []vpcepServicePermissionBlock{
				{Permission: testIamDomainId},
				{Permission: testAnotherIamDomainId},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.plan
			resp := &resource.UpdateResponse{State: newM1ToM3State(t)}

			actual := setM1ToM3UpdateState(context.Background(), resp, &plan)

			assert.True(t, actual)
			assert.Empty(t, resp.Diagnostics)
			assert.Equal(t, tc.expectedPorts, plan.M3VpcepServicePorts)
			assert.Equal(t, tc.expectedPermissions, plan.M3VpcepServicePermissions)
			var actualState netConnectM1ToM3Model
			diags := resp.State.Get(context.Background(), &actualState)
			assert.Empty(t, diags)
			assert.Equal(t, tc.expectedPorts, actualState.M3VpcepServicePorts)
			assert.Equal(t, tc.expectedPermissions, actualState.M3VpcepServicePermissions)
			assertM1ToM3State(t, plan, actualState)
		})
	}
}

func Test_netConnectM1ToM3Resource_reconcileM1ToM3Service(t *testing.T) {
	testCases := []struct {
		name                   string
		state                  netConnectM1ToM3Model
		plan                   netConnectM1ToM3Model
		vpcepService           *mockVpcepServiceService
		expectedErr            string
		expectedUpdateReq      *service.VpcepServiceInput
		expectedReconcile      []service.PermissionInput
		expectedUpdateCall     int
		expectedPermissionCall int
	}{
		{
			state:        newM1ToM3Model(),
			plan:         newM1ToM3Model(),
			vpcepService: &mockVpcepServiceService{},
		},
		{
			name:  "GIVEN service config changes WHEN reconcileM1ToM3Service SHOULD update existing service",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				return plan
			}(),
			vpcepService: &mockVpcepServiceService{},
			expectedUpdateReq: func() *service.VpcepServiceInput {
				expected := newExpectedM1ToM3VpcepServiceInput()
				expected.PortId = "port-2"
				return expected
			}(),
			expectedUpdateCall: 1,
		},
		{
			name:  "GIVEN service permissions change WHEN reconcileM1ToM3Service SHOULD reconcile permissions",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService:           &mockVpcepServiceService{},
			expectedReconcile:      []service.PermissionInput{{Permission: "domain-id-3"}},
			expectedPermissionCall: 1,
		},
		{
			name:  "GIVEN service config and permissions change WHEN reconcileM1ToM3Service SHOULD update config and reconcile permissions",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				plan.VpcepServiceId = types.StringUnknown()
				return plan
			}(),
			vpcepService: &mockVpcepServiceService{},
			expectedUpdateReq: func() *service.VpcepServiceInput {
				expected := newExpectedM1ToM3VpcepServiceInput()
				expected.PortId = "port-2"
				return expected
			}(),
			expectedReconcile:      []service.PermissionInput{{Permission: "domain-id-3"}},
			expectedUpdateCall:     1,
			expectedPermissionCall: 1,
		},
		{
			name: "GIVEN service id missing WHEN reconcileM1ToM3Service SHOULD return replacement required error",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepServiceId = types.StringNull()
				return state
			}(),
			plan:         newM1ToM3Model(),
			vpcepService: &mockVpcepServiceService{},
			expectedErr:  "vpcep-service is missing; Terraform replacement is required",
		},
		{
			name:  "GIVEN root service attribute changes WHEN reconcileM1ToM3Service SHOULD return Terraform replacement error",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcId = "m3-vpc-2"
				return plan
			}(),
			vpcepService: &mockVpcepServiceService{},
			expectedErr:  "vpcep-service replacement should be handled by Terraform resource replacement",
		},
		{
			name:  "GIVEN service update fails WHEN reconcileM1ToM3Service SHOULD return update error",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				return plan
			}(),
			vpcepService:       &mockVpcepServiceService{updateErr: errors.New("UpdateConfig failed")},
			expectedErr:        "UpdateConfig failed",
			expectedUpdateCall: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.plan
			r := newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, tc.vpcepService, &mockLbmDnsService{})

			err := r.reconcileM1ToM3Service(context.Background(), tc.state, &plan)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr)
			}
			assert.Len(t, tc.vpcepService.updateServiceIds, tc.expectedUpdateCall)
			assert.Len(t, tc.vpcepService.reconcilePermissionIds, tc.expectedPermissionCall)
			if tc.expectedUpdateReq != nil {
				assert.Equal(t, []string{testVpcepServiceId}, tc.vpcepService.updateServiceIds)
				assert.Equal(t, []service.VpcepServiceInput{*tc.expectedUpdateReq}, tc.vpcepService.updateServiceInputs)
			}
			if tc.expectedReconcile != nil {
				assert.Equal(t, []string{testVpcepServiceId}, tc.vpcepService.reconcilePermissionIds)
				assert.Equal(t, [][]service.PermissionInput{tc.expectedReconcile}, tc.vpcepService.reconcilePermissionInputs)
			}
			if tc.expectedUpdateCall > 0 || tc.expectedPermissionCall > 0 {
				assert.Equal(t, tc.state.VpcepServiceId, plan.VpcepServiceId)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_reconcileM1ToM3Service_idempotent(t *testing.T) {
	state := newM1ToM3Model()
	plan := newM1ToM3Model()
	plan.M3PortId = "port-2"
	plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
	firstService := &mockVpcepServiceService{}
	r := newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, firstService, &mockLbmDnsService{})

	err := r.reconcileM1ToM3Service(context.Background(), state, &plan)
	require.NoError(t, err)

	secondService := &mockVpcepServiceService{}
	r = newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, secondService, &mockLbmDnsService{})
	secondPlan := plan
	err = r.reconcileM1ToM3Service(context.Background(), plan, &secondPlan)

	assert.NoError(t, err)
	assert.Empty(t, secondService.updateServiceIds)
	assert.Empty(t, secondService.reconcilePermissionIds)
}

func Test_netConnectM1ToM3Resource_updateExistingM1ToM3Service(t *testing.T) {
	testCases := []struct {
		name                   string
		plan                   netConnectM1ToM3Model
		vpcepService           *mockVpcepServiceService
		expectedErr            string
		expectedUpdateReq      *service.VpcepServiceInput
		expectedReconcile      []service.PermissionInput
		expectedUpdateCall     int
		expectedPermissionCall int
	}{
		{
			name: "GIVEN service config and permissions changed WHEN updateExistingM1ToM3Service SHOULD update both",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService: &mockVpcepServiceService{},
			expectedUpdateReq: func() *service.VpcepServiceInput {
				expected := newExpectedM1ToM3VpcepServiceInput()
				expected.PortId = "port-2"
				return expected
			}(),
			expectedReconcile:      []service.PermissionInput{{Permission: "domain-id-3"}},
			expectedUpdateCall:     1,
			expectedPermissionCall: 1,
		},
		{
			name: "GIVEN only service permissions changed WHEN updateExistingM1ToM3Service SHOULD reconcile permissions only",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService:           &mockVpcepServiceService{},
			expectedReconcile:      []service.PermissionInput{{Permission: "domain-id-3"}},
			expectedPermissionCall: 1,
		},
		{
			name: "GIVEN service config update fails WHEN updateExistingM1ToM3Service SHOULD return update error",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService:       &mockVpcepServiceService{updateErr: errors.New("UpdateConfig failed")},
			expectedErr:        "UpdateConfig failed",
			expectedUpdateCall: 1,
		},
		{
			name: "GIVEN permission reconcile fails WHEN updateExistingM1ToM3Service SHOULD return reconcile error",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService:           &mockVpcepServiceService{reconcileErr: errors.New("ReconcilePermissions failed")},
			expectedErr:            "ReconcilePermissions failed",
			expectedPermissionCall: 1,
		},
		{
			name: "GIVEN config update succeeds and permission reconcile fails WHEN updateExistingM1ToM3Service SHOULD keep update call",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-3"}}
				return plan
			}(),
			vpcepService: &mockVpcepServiceService{reconcileErr: errors.New("ReconcilePermissions failed")},
			expectedUpdateReq: func() *service.VpcepServiceInput {
				expected := newExpectedM1ToM3VpcepServiceInput()
				expected.PortId = "port-2"
				return expected
			}(),
			expectedErr:            "ReconcilePermissions failed",
			expectedUpdateCall:     1,
			expectedPermissionCall: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newM1ToM3ResourceWithMocks(&mockVpcepEndpointService{}, tc.vpcepService, &mockLbmDnsService{})

			err := r.updateExistingM1ToM3Service(context.Background(), newM1ToM3Model(), &tc.plan)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr)
			}
			assert.Len(t, tc.vpcepService.updateServiceIds, tc.expectedUpdateCall)
			assert.Len(t, tc.vpcepService.reconcilePermissionIds, tc.expectedPermissionCall)
			if tc.expectedUpdateReq != nil {
				assert.Equal(t, []string{testVpcepServiceId}, tc.vpcepService.updateServiceIds)
				assert.Equal(t, []service.VpcepServiceInput{*tc.expectedUpdateReq}, tc.vpcepService.updateServiceInputs)
			}
			if tc.expectedReconcile != nil {
				assert.Equal(t, []string{testVpcepServiceId}, tc.vpcepService.reconcilePermissionIds)
				assert.Equal(t, [][]service.PermissionInput{tc.expectedReconcile}, tc.vpcepService.reconcilePermissionInputs)
			}
		})
	}
}

func Test_netConnectM1ToM3Resource_reconcileM1ToM3Endpoint(t *testing.T) {
	testCases := []struct {
		name                       string
		state                      netConnectM1ToM3Model
		plan                       netConnectM1ToM3Model
		endpointService            *mockVpcepEndpointService
		expectedErr                string
		expectedCreateInput        *service.VpcEndpointInput
		expectedEndpointCreateCall int
		expectedStaleEndpointId    string
		expectedEndpointId         string
		expectedEndpointIp         string
		expectedEndpointServiceId  string
	}{
		{
			name:                      "GIVEN endpoint unchanged WHEN reconcileM1ToM3Endpoint SHOULD keep endpoint unchanged",
			state:                     newM1ToM3Model(),
			plan:                      newM1ToM3Model(),
			endpointService:           &mockVpcepEndpointService{},
			expectedEndpointId:        testVpcepEndpointId,
			expectedEndpointIp:        testVpcepEndpointIp,
			expectedEndpointServiceId: testVpcepServiceId,
		},
		{
			name: "GIVEN endpoint id missing WHEN reconcileM1ToM3Endpoint SHOULD create endpoint",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointId = types.StringNull()
				return state
			}(),
			plan: newM1ToM3Model(),
			endpointService: &mockVpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			expectedCreateInput:        newExpectedM1ToM3EndpointInput(),
			expectedEndpointCreateCall: 1,
			expectedEndpointId:         testVpcepEndpointId,
			expectedEndpointIp:         testVpcepEndpointIp,
			expectedEndpointServiceId:  testVpcepServiceId,
		},
		{
			name:  "GIVEN endpoint network changes WHEN reconcileM1ToM3Endpoint SHOULD create endpoint and mark old endpoint stale",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M1PlusSubnetId = "subnet-2"
				return plan
			}(),
			endpointService: &mockVpcepEndpointService{
				createEndpointId: "endpoint-2",
				createEndpointIp: "10.0.0.9",
			},
			expectedCreateInput: func() *service.VpcEndpointInput {
				expected := newExpectedM1ToM3EndpointInput()
				expected.SubnetId = "subnet-2"
				return expected
			}(),
			expectedEndpointCreateCall: 1,
			expectedStaleEndpointId:    testVpcepEndpointId,
			expectedEndpointId:         "endpoint-2",
			expectedEndpointIp:         "10.0.0.9",
			expectedEndpointServiceId:  testVpcepServiceId,
		},
		{
			name: "GIVEN endpoint create fails WHEN reconcileM1ToM3Endpoint SHOULD return create error",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointId = types.StringNull()
				return state
			}(),
			plan: newM1ToM3Model(),
			endpointService: &mockVpcepEndpointService{
				createErr: errors.New("create endpoint failed"),
			},
			expectedErr:                "create endpoint failed",
			expectedCreateInput:        newExpectedM1ToM3EndpointInput(),
			expectedEndpointCreateCall: 1,
			expectedEndpointId:         testVpcepEndpointId,
			expectedEndpointIp:         testVpcepEndpointIp,
			expectedEndpointServiceId:  testVpcepServiceId,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.plan
			stale := &m1ToM3StaleResources{}
			r := newM1ToM3ResourceWithMocks(tc.endpointService, &mockVpcepServiceService{}, &mockLbmDnsService{})

			err := r.reconcileM1ToM3Endpoint(context.Background(), tc.state, &plan, stale)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr)
			}
			assert.Len(t, tc.endpointService.createInputs, tc.expectedEndpointCreateCall)
			if tc.expectedCreateInput != nil {
				assert.Equal(t, []service.VpcEndpointInput{*tc.expectedCreateInput}, tc.endpointService.createInputs)
			}
			assert.Equal(t, tc.expectedStaleEndpointId, stale.endpointId)
			assert.Equal(t, types.StringValue(tc.expectedEndpointId), plan.VpcepEndpointId)
			assert.Equal(t, types.StringValue(tc.expectedEndpointIp), plan.VpcepEndpointIp)
			assert.Equal(t, types.StringValue(tc.expectedEndpointServiceId), plan.VpcepEndpointServiceId)
		})
	}
}

func Test_preserveKnownComputedFields(t *testing.T) {
	stateValues := mustLbmDnsRecordValues(t, []lbmDnsRecordValueBlock{
		{RecordType: "A", RecordValue: testVpcepEndpointIp},
	})
	knownValues := mustLbmDnsRecordValues(t, []lbmDnsRecordValueBlock{
		{RecordType: "A", RecordValue: "10.0.0.9"},
	})
	state := netConnectM1ToM3Model{
		VpcepServiceId:         types.StringValue(testVpcepServiceId),
		VpcepEndpointId:        types.StringValue(testVpcepEndpointId),
		VpcepEndpointIp:        types.StringValue(testVpcepEndpointIp),
		VpcepEndpointServiceId: types.StringValue(testVpcepServiceId),
		LbmDnsRecordId:         types.StringValue(testLbmDnsRecordId),
		LbmDnsRecordValues:     stateValues,
	}
	unknownPlan := netConnectM1ToM3Model{
		VpcepServiceId:         types.StringUnknown(),
		VpcepEndpointId:        types.StringUnknown(),
		VpcepEndpointIp:        types.StringUnknown(),
		VpcepEndpointServiceId: types.StringUnknown(),
		LbmDnsRecordId:         types.StringUnknown(),
		LbmDnsRecordValues:     types.ListUnknown(lbmDnsRecordValueObjectType),
	}
	knownPlan := netConnectM1ToM3Model{
		VpcepServiceId:         types.StringValue("service-2"),
		VpcepEndpointId:        types.StringValue("endpoint-2"),
		VpcepEndpointIp:        types.StringValue("10.0.0.9"),
		VpcepEndpointServiceId: types.StringValue("service-2"),
		LbmDnsRecordId:         types.StringValue("dns-record-2"),
		LbmDnsRecordValues:     knownValues,
	}
	partialUnknownPlan := knownPlan
	partialUnknownPlan.VpcepEndpointIp = types.StringUnknown()
	partialUnknownExpected := knownPlan
	partialUnknownExpected.VpcepEndpointIp = state.VpcepEndpointIp

	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected netConnectM1ToM3Model
	}{
		{
			name:     "GIVEN unknown computed fields WHEN preserveKnownComputedFields SHOULD copy state values",
			plan:     unknownPlan,
			expected: state,
		},
		{
			name:     "GIVEN partial unknown computed fields WHEN preserveKnownComputedFields SHOULD only copy unknown values",
			plan:     partialUnknownPlan,
			expected: partialUnknownExpected,
		},
		{
			name:     "GIVEN known computed fields WHEN preserveKnownComputedFields SHOULD keep plan values",
			plan:     knownPlan,
			expected: knownPlan,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.plan

			preserveKnownComputedFields(&plan, state)

			assert.Equal(t, tc.expected, plan)
		})
	}
}

func Test_clearM1ToM3ServiceInputState(t *testing.T) {
	newClearedState := func() netConnectM1ToM3Model {
		state := newM1ToM3Model()
		state.M3VpcId = ""
		state.M3ServerType = ""
		state.M3PortId = ""
		state.M3VpcepServicePorts = []vpcepServicePortBlock{}
		state.M3VpcepServicePermissions = []vpcepServicePermissionBlock{}
		return state
	}

	partialClearedState := newM1ToM3Model()
	partialClearedState.M3VpcId = ""
	partialClearedState.M3VpcepServicePorts = []vpcepServicePortBlock{}

	testCases := []struct {
		name     string
		state    netConnectM1ToM3Model
		expected netConnectM1ToM3Model
	}{
		{
			name:     "GIVEN populated state WHEN clearM1ToM3ServiceInputState SHOULD clear service input fields only",
			state:    newM1ToM3Model(),
			expected: newClearedState(),
		},
		{
			name:     "GIVEN partial empty service input fields WHEN clearM1ToM3ServiceInputState SHOULD clear remaining service input fields only",
			state:    partialClearedState,
			expected: newClearedState(),
		},
		{
			name:     "GIVEN empty service input fields WHEN clearM1ToM3ServiceInputState SHOULD keep service input fields empty",
			state:    newClearedState(),
			expected: newClearedState(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.state

			clearM1ToM3ServiceInputState(&state)

			assert.Equal(t, tc.expected, state)
		})
	}
}

func Test_clearM1ToM3EndpointInputState(t *testing.T) {
	newClearedState := func() netConnectM1ToM3Model {
		state := newM1ToM3Model()
		state.M1PlusVpcId = ""
		state.M1PlusSubnetId = ""
		return state
	}

	partialClearedState := newM1ToM3Model()
	partialClearedState.M1PlusVpcId = ""

	testCases := []struct {
		name     string
		state    netConnectM1ToM3Model
		expected netConnectM1ToM3Model
	}{
		{
			name:     "GIVEN populated state WHEN clearM1ToM3EndpointInputState SHOULD clear endpoint input fields only",
			state:    newM1ToM3Model(),
			expected: newClearedState(),
		},
		{
			name:     "GIVEN partial empty endpoint input fields WHEN clearM1ToM3EndpointInputState SHOULD clear remaining endpoint input fields only",
			state:    partialClearedState,
			expected: newClearedState(),
		},
		{
			name:     "GIVEN empty endpoint input fields WHEN clearM1ToM3EndpointInputState SHOULD keep endpoint input fields empty",
			state:    newClearedState(),
			expected: newClearedState(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.state

			clearM1ToM3EndpointInputState(&state)

			assert.Equal(t, tc.expected, state)
		})
	}
}

func Test_clearM1ToM3DnsInputState(t *testing.T) {
	newClearedState := func() netConnectM1ToM3Model {
		state := newM1ToM3Model()
		state.DnsDomain = ""
		state.DnsDomainSuffix = ""
		state.LbmDnsServiceName = ""
		state.RegionCode = ""
		return state
	}

	partialClearedState := newM1ToM3Model()
	partialClearedState.DnsDomain = ""
	partialClearedState.RegionCode = ""

	testCases := []struct {
		name     string
		state    netConnectM1ToM3Model
		expected netConnectM1ToM3Model
	}{
		{
			name:     "GIVEN populated state WHEN clearM1ToM3DnsInputState SHOULD clear dns input fields only",
			state:    newM1ToM3Model(),
			expected: newClearedState(),
		},
		{
			name:     "GIVEN partial empty dns input fields WHEN clearM1ToM3DnsInputState SHOULD clear remaining dns input fields only",
			state:    partialClearedState,
			expected: newClearedState(),
		},
		{
			name:     "GIVEN empty dns input fields WHEN clearM1ToM3DnsInputState SHOULD keep dns input fields empty",
			state:    newClearedState(),
			expected: newClearedState(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.state

			clearM1ToM3DnsInputState(&state)

			assert.Equal(t, tc.expected, state)
		})
	}
}

func Test_m1ToM3AllChildIdentitiesMissing(t *testing.T) {
	newState := func(serviceMissing, endpointMissing, dnsMissing bool) netConnectM1ToM3Model {
		state := newM1ToM3Model()
		if serviceMissing {
			state.VpcepServiceId = types.StringNull()
		}
		if endpointMissing {
			state.VpcepEndpointId = types.StringNull()
		}
		if dnsMissing {
			state.LbmDnsRecordId = types.StringNull()
		}
		return state
	}

	testCases := []struct {
		name     string
		state    netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN all child identities are null WHEN m1ToM3AllChildIdentitiesMissing SHOULD return true",
			state:    newState(true, true, true),
			expected: true,
		},
		{
			name:     "GIVEN service identity remains WHEN m1ToM3AllChildIdentitiesMissing SHOULD return false",
			state:    newState(false, true, true),
			expected: false,
		},
		{
			name:     "GIVEN endpoint identity remains WHEN m1ToM3AllChildIdentitiesMissing SHOULD return false",
			state:    newState(true, false, true),
			expected: false,
		},
		{
			name:     "GIVEN dns identity remains WHEN m1ToM3AllChildIdentitiesMissing SHOULD return false",
			state:    newState(true, true, false),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := m1ToM3AllChildIdentitiesMissing(tc.state)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_serviceRequiresReplacement(t *testing.T) {
	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN same service identity WHEN serviceRequiresReplacement SHOULD return false",
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN changed m3 vpc WHEN serviceRequiresReplacement SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcId = "vpc-2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN changed server type WHEN serviceRequiresReplacement SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3ServerType = "VM"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN empty m3 vpc WHEN serviceRequiresReplacement SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcId = ""
				return plan
			}(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := serviceRequiresReplacement(newM1ToM3Model(), tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_serviceRequiresInPlaceUpdate(t *testing.T) {
	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN same service config WHEN serviceRequiresInPlaceUpdate SHOULD return false",
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN changed service port config WHEN serviceRequiresInPlaceUpdate SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN changed service permissions WHEN serviceRequiresInPlaceUpdate SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-id-c"}}
				return plan
			}(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := serviceRequiresInPlaceUpdate(newM1ToM3Model(), tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_servicePortConfigChanged(t *testing.T) {
	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN identical ports WHEN servicePortConfigChanged SHOULD return false",
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN same ports in reordered order WHEN servicePortConfigChanged SHOULD return false",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				ports := testVpcepServicePorts()
				plan.M3VpcepServicePorts = []vpcepServicePortBlock{ports[1], ports[0]}
				return plan
			}(),
			expected: false,
		},
		{
			name: "GIVEN changed port id WHEN servicePortConfigChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3PortId = "port-2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN changed ports WHEN servicePortConfigChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePorts = []vpcepServicePortBlock{{ClientPort: 8080, ServerPort: 8080}}
				return plan
			}(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := servicePortConfigChanged(newM1ToM3Model(), tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_servicePermissionsChanged(t *testing.T) {
	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN identical permissions WHEN servicePermissionsChanged SHOULD return false",
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN same permissions in reordered order WHEN servicePermissionsChanged SHOULD return false",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				permissions := testVpcepServicePermissions()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{permissions[1], permissions[0]}
				return plan
			}(),
			expected: false,
		},
		{
			name: "GIVEN changed permissions WHEN servicePermissionsChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M3VpcepServicePermissions = []vpcepServicePermissionBlock{{Permission: "domain-c"}}
				return plan
			}(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := servicePermissionsChanged(newM1ToM3Model(), tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_shouldReplaceEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		state    netConnectM1ToM3Model
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN endpoint matches plan WHEN shouldReplaceEndpoint SHOULD return false",
			state:    newM1ToM3Model(),
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN endpoint id is null WHEN shouldReplaceEndpoint SHOULD return false",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointId = types.StringNull()
				return state
			}(),
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name:  "GIVEN endpoint vpc changed WHEN shouldReplaceEndpoint SHOULD return true",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M1PlusVpcId = "m1-vpc-2"
				return plan
			}(),
			expected: true,
		},
		{
			name:  "GIVEN endpoint subnet changed WHEN shouldReplaceEndpoint SHOULD return true",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.M1PlusSubnetId = "subnet-2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN endpoint service id is null WHEN shouldReplaceEndpoint SHOULD return true",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointServiceId = types.StringNull()
				return state
			}(),
			plan:     newM1ToM3Model(),
			expected: true,
		},
		{
			name:  "GIVEN plan service id is null WHEN shouldReplaceEndpoint SHOULD return true",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.VpcepServiceId = types.StringNull()
				return plan
			}(),
			expected: true,
		},
		{
			name:  "GIVEN plan service id is unknown WHEN shouldReplaceEndpoint SHOULD return true",
			state: newM1ToM3Model(),
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.VpcepServiceId = types.StringUnknown()
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN endpoint service id differs from plan service id WHEN shouldReplaceEndpoint SHOULD return true",
			state: func() netConnectM1ToM3Model {
				state := newM1ToM3Model()
				state.VpcepEndpointServiceId = types.StringValue("service-old")
				return state
			}(),
			plan:     newM1ToM3Model(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := shouldReplaceEndpoint(tc.state, tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_dnsIdentityChanged(t *testing.T) {
	testCases := []struct {
		name     string
		plan     netConnectM1ToM3Model
		expected bool
	}{
		{
			name:     "GIVEN same dns identity WHEN dnsIdentityChanged SHOULD return false",
			plan:     newM1ToM3Model(),
			expected: false,
		},
		{
			name: "GIVEN changed region code WHEN dnsIdentityChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.RegionCode = "region-2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN changed dns domain WHEN dnsIdentityChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.DnsDomain = "api2"
				return plan
			}(),
			expected: true,
		},
		{
			name: "GIVEN changed dns domain suffix WHEN dnsIdentityChanged SHOULD return true",
			plan: func() netConnectM1ToM3Model {
				plan := newM1ToM3Model()
				plan.DnsDomainSuffix = "internal2"
				return plan
			}(),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := dnsIdentityChanged(newM1ToM3Model(), tc.plan)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_lbmDnsRecordValueNeedsUpdate(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name                       string
		values                     types.List
		endpointIp                 types.String
		expected                   bool
		expectedDiagSummary        string
		expectedDiagDetailContains string
	}{
		{
			name: "GIVEN matching A record WHEN lbmDnsRecordValueNeedsUpdate SHOULD return false",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}}),
			endpointIp: types.StringValue(testVpcepEndpointIp),
			expected:   false,
		},
		{
			name: "GIVEN endpoint IP is null WHEN lbmDnsRecordValueNeedsUpdate SHOULD return true",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}}),
			endpointIp: types.StringNull(),
			expected:   true,
		},
		{
			name: "GIVEN endpoint IP is unknown WHEN lbmDnsRecordValueNeedsUpdate SHOULD return true",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}}),
			endpointIp: types.StringUnknown(),
			expected:   true,
		},
		{
			name: "GIVEN no A record WHEN lbmDnsRecordValueNeedsUpdate SHOULD return true",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "CNAME", RecordValue: "api.example.com"}}),
			endpointIp: types.StringValue(testVpcepEndpointIp),
			expected:   true,
		},
		{
			name: "GIVEN different A record WHEN lbmDnsRecordValueNeedsUpdate SHOULD return true",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: "10.0.0.9"}}),
			endpointIp: types.StringValue(testVpcepEndpointIp),
			expected:   true,
		},
		{
			name:                       "GIVEN invalid record values WHEN lbmDnsRecordValueNeedsUpdate SHOULD return diagnostics",
			values:                     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("bad")}),
			endpointIp:                 types.StringValue(testVpcepEndpointIp),
			expected:                   false,
			expectedDiagSummary:        "Value Conversion Error",
			expectedDiagDetailContains: "cannot reflect tftypes.String into a struct, must be an object",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, diags := lbmDnsRecordValueNeedsUpdate(ctx, tc.values, tc.endpointIp)

			assert.Equal(t, tc.expected, actual)
			assertDiagnostics(t, tc.expectedDiagSummary, tc.expectedDiagDetailContains, diags)
		})
	}
}

func Test_lbmDnsRecordAValue(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name                       string
		values                     types.List
		expectedValue              string
		expectedFound              bool
		expectedDiagSummary        string
		expectedDiagDetailContains string
	}{
		{
			name: "GIVEN record values with A record WHEN lbmDnsRecordAValue SHOULD return A record value",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}}),
			expectedValue: testVpcepEndpointIp,
			expectedFound: true,
		},
		{
			name:          "GIVEN null record values WHEN lbmDnsRecordAValue SHOULD return not found",
			values:        types.ListNull(lbmDnsRecordValueObjectType),
			expectedValue: "",
			expectedFound: false,
		},
		{
			name:          "GIVEN unknown record values WHEN lbmDnsRecordAValue SHOULD return not found",
			values:        types.ListUnknown(lbmDnsRecordValueObjectType),
			expectedValue: "",
			expectedFound: false,
		},
		{
			name: "GIVEN record values without A record WHEN lbmDnsRecordAValue SHOULD return not found",
			values: mustLbmDnsRecordValues(t,
				[]lbmDnsRecordValueBlock{{RecordType: "CNAME", RecordValue: "api.example.com"}}),
			expectedValue: "",
			expectedFound: false,
		},
		{
			name:                       "GIVEN invalid record values WHEN lbmDnsRecordAValue SHOULD return diagnostics",
			values:                     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("bad")}),
			expectedValue:              "",
			expectedFound:              false,
			expectedDiagSummary:        "Value Conversion Error",
			expectedDiagDetailContains: "cannot reflect tftypes.String into a struct, must be an object",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualValue, actualFound, diags := lbmDnsRecordAValue(ctx, tc.values)

			assert.Equal(t, tc.expectedValue, actualValue)
			assert.Equal(t, tc.expectedFound, actualFound)
			assertDiagnostics(t, tc.expectedDiagSummary, tc.expectedDiagDetailContains, diags)
		})
	}
}

func Test_convertPorts(t *testing.T) {
	testCases := []struct {
		name     string
		input    []vpcepServicePortBlock
		expected []service.PortPair
	}{
		{
			name: "GIVEN resource port blocks WHEN convertPorts SHOULD keep port values",
			input: []vpcepServicePortBlock{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
			expected: []service.PortPair{
				{ClientPort: 80, ServerPort: 8080},
				{ClientPort: 443, ServerPort: 8443},
			},
		},
		{
			name:     "GIVEN empty resource port blocks WHEN convertPorts SHOULD return empty port pairs",
			input:    []vpcepServicePortBlock{},
			expected: []service.PortPair{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := convertPorts(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_convertPermissions(t *testing.T) {
	testCases := []struct {
		name     string
		input    []vpcepServicePermissionBlock
		expected []service.PermissionInput
	}{
		{
			name:  "GIVEN resource permission blocks WHEN convertPermissions SHOULD keep permission values",
			input: testVpcepServicePermissions(),
			expected: []service.PermissionInput{
				{Permission: testIamDomainId},
				{Permission: testAnotherIamDomainId},
			},
		},
		{
			name:     "GIVEN empty permission blocks WHEN convertPermissions SHOULD return empty permission inputs",
			input:    []vpcepServicePermissionBlock{},
			expected: []service.PermissionInput{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := convertPermissions(tc.input)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_buildLbmDnsRecordValues_errorBranches(t *testing.T) {
	// types.ObjectValue/ListValue 是薄封装，可能被编译器内联；
	// 这里 patch 底层构造函数覆盖防御性 diagnostics 分支，避免为测试修改生产代码。
	testCases := []struct {
		name                       string
		objectValue                func(map[string]attr.Type, map[string]attr.Value) (types.Object, diag.Diagnostics)
		listValue                  func(attr.Type, []attr.Value) (types.List, diag.Diagnostics)
		expectedDiagSummary        string
		expectedDiagDetailContains string
	}{
		{
			name: "GIVEN list value diagnostics WHEN buildLbmDnsRecordValues SHOULD return unknown list and diagnostics",
			listValue: func(_ attr.Type, _ []attr.Value) (types.List, diag.Diagnostics) {
				var diags diag.Diagnostics
				diags.AddError("list value failed", "mock list value diagnostics")
				return types.ListUnknown(lbmDnsRecordValueObjectType), diags
			},
			expectedDiagSummary:        "list value failed",
			expectedDiagDetailContains: "mock list value diagnostics",
		},
		{
			name: "GIVEN object value diagnostics WHEN buildLbmDnsRecordValues SHOULD return unknown list and diagnostics",
			objectValue: func(_ map[string]attr.Type, _ map[string]attr.Value) (types.Object, diag.Diagnostics) {
				var diags diag.Diagnostics
				diags.AddError("object value failed", "mock object value diagnostics")
				return types.ObjectUnknown(lbmDnsRecordValueAttrTypes), diags
			},
			listValue: func(_ attr.Type, _ []attr.Value) (types.List, diag.Diagnostics) {
				t.Fatal("list value should not be called when object value returns diagnostics")
				return types.ListUnknown(lbmDnsRecordValueObjectType), nil
			},
			expectedDiagSummary:        "object value failed",
			expectedDiagDetailContains: "mock object value diagnostics",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var patches *gomonkey.Patches
			if tc.objectValue != nil {
				patches = gomonkey.ApplyFunc(basetypes.NewObjectValue, tc.objectValue)
			}
			if tc.listValue != nil {
				if patches == nil {
					patches = gomonkey.ApplyFunc(basetypes.NewListValue, tc.listValue)
				} else {
					patches.ApplyFunc(basetypes.NewListValue, tc.listValue)
				}
			}
			defer patches.Reset()

			actual, diags := buildLbmDnsRecordValues([]lbmDnsRecordValueBlock{
				{RecordType: "A", RecordValue: testVpcepEndpointIp},
			})

			assert.True(t, actual.IsUnknown())
			assertDiagnostics(t, tc.expectedDiagSummary, tc.expectedDiagDetailContains, diags)
		})
	}
}

func newM1ToM3Model() netConnectM1ToM3Model {
	return netConnectM1ToM3Model{
		M3VpcId:                   testM3VpcId,
		M3ServerType:              testM3ServerType,
		M3PortId:                  testM3PortId,
		M3VpcepServicePorts:       testVpcepServicePorts(),
		M3VpcepServicePermissions: testVpcepServicePermissions(),
		M1PlusVpcId:               testM1PlusVpcId,
		M1PlusSubnetId:            testM1PlusSubnetId,
		DnsDomain:                 testDnsDomain,
		DnsDomainSuffix:           testDnsDomainSuffix,
		LbmDnsServiceName:         testLbmDnsServiceName,
		RegionCode:                testRegionCode,
		VpcepServiceId:            types.StringValue(testVpcepServiceId),
		VpcepEndpointId:           types.StringValue(testVpcepEndpointId),
		VpcepEndpointIp:           types.StringValue(testVpcepEndpointIp),
		VpcepEndpointServiceId:    types.StringValue(testVpcepServiceId),
		LbmDnsRecordId:            types.StringValue(testLbmDnsRecordId),
		LbmDnsRecordValues: testLbmDnsRecordValues([]lbmDnsRecordValueBlock{
			{RecordType: "A", RecordValue: testVpcepEndpointIp},
		}),
	}
}

// mustLbmDnsRecordValues 用于具体测试用例中构造 record values，并通过断言暴露意外 diagnostics。
func mustLbmDnsRecordValues(t *testing.T, values []lbmDnsRecordValueBlock) types.List {
	t.Helper()

	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		objectValue, diags := types.ObjectValue(lbmDnsRecordValueAttrTypes, map[string]attr.Value{
			"record_type":  types.StringValue(value.RecordType),
			"record_value": types.StringValue(value.RecordValue),
		})
		if !assert.False(t, diags.HasError(), "expected record value object to build without diagnostics, got %v",
			diags) {
			return types.ListUnknown(lbmDnsRecordValueObjectType)
		}
		elements = append(elements, objectValue)
	}

	recordValues, diags := types.ListValue(lbmDnsRecordValueObjectType, elements)
	assert.False(t, diags.HasError(), "expected record values to build without diagnostics, got %v", diags)
	return recordValues
}

func assertDiagnostics(t *testing.T, expectedSummary string, expectedDetailContains string, actual diag.Diagnostics) {
	t.Helper()

	if expectedSummary == "" {
		assert.Empty(t, actual)
		return
	}
	if assert.Len(t, actual, 1) {
		assert.Equal(t, expectedSummary, actual[0].Summary())
		assert.Contains(t, actual[0].Detail(), expectedDetailContains)
	}
}

func assertRecordValueList(t *testing.T, expected []lbmDnsRecordValueBlock, actual types.List) {
	t.Helper()

	var actualBlocks []lbmDnsRecordValueBlock
	diags := actual.ElementsAs(context.Background(), &actualBlocks, false)
	if !assert.False(t, diags.HasError(), "expected record values to decode without diagnostics, got %v", diags) {
		return
	}
	assert.Equal(t, expected, actualBlocks)
}

// assertM1ToM3State 逐字段比较 netConnectM1ToM3Model，不使用 assert.Equal 是因为
// LbmDnsRecordValues (types.List) 的内部状态无法通过 reflect.DeepEqual 可靠比较，
// 需要先判断 IsNull/IsUnknown，再解码后逐元素比较。
func assertM1ToM3State(t *testing.T, expected, actual netConnectM1ToM3Model) {
	t.Helper()

	assert.Equal(t, expected.M3VpcId, actual.M3VpcId)
	assert.Equal(t, expected.M3ServerType, actual.M3ServerType)
	assert.Equal(t, expected.M3PortId, actual.M3PortId)
	assert.Equal(t, expected.M3VpcepServicePorts, actual.M3VpcepServicePorts)
	assert.Equal(t, expected.M3VpcepServicePermissions, actual.M3VpcepServicePermissions)
	assert.Equal(t, expected.M1PlusVpcId, actual.M1PlusVpcId)
	assert.Equal(t, expected.M1PlusSubnetId, actual.M1PlusSubnetId)
	assert.Equal(t, expected.DnsDomain, actual.DnsDomain)
	assert.Equal(t, expected.DnsDomainSuffix, actual.DnsDomainSuffix)
	assert.Equal(t, expected.LbmDnsServiceName, actual.LbmDnsServiceName)
	assert.Equal(t, expected.RegionCode, actual.RegionCode)
	assert.Equal(t, expected.VpcepServiceId, actual.VpcepServiceId)
	assert.Equal(t, expected.VpcepEndpointId, actual.VpcepEndpointId)
	assert.Equal(t, expected.VpcepEndpointIp, actual.VpcepEndpointIp)
	assert.Equal(t, expected.VpcepEndpointServiceId, actual.VpcepEndpointServiceId)
	assert.Equal(t, expected.LbmDnsRecordId, actual.LbmDnsRecordId)
	assert.Equal(t, expected.LbmDnsRecordValues.IsNull(), actual.LbmDnsRecordValues.IsNull())
	assert.Equal(t, expected.LbmDnsRecordValues.IsUnknown(), actual.LbmDnsRecordValues.IsUnknown())
	if expected.LbmDnsRecordValues.IsNull() || expected.LbmDnsRecordValues.IsUnknown() {
		return
	}
	var expectedRecordValues []lbmDnsRecordValueBlock
	diags := expected.LbmDnsRecordValues.ElementsAs(context.Background(), &expectedRecordValues, false)
	require.False(t, diags.HasError(), "expected record values to decode without diagnostics, got %v", diags)
	assertRecordValueList(t, expectedRecordValues, actual.LbmDnsRecordValues)
}

func newM1ToM3ResourceWithMocks(endpoint m1ToM3VpcepEndpointService, vpcep m1ToM3VpcepService,
	dns m1ToM3LbmDnsService) *netConnectM1ToM3Resource {
	return &netConnectM1ToM3Resource{
		m1PlusVpcepService: endpoint,
		m3VpcepService:     vpcep,
		lbmDnsService:      dns,
	}
}

func newM1ToM3Plan(t *testing.T, model netConnectM1ToM3Model) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: m1ToM3Schema(t)}
	diags := plan.Set(context.Background(), &model)
	assert.False(t, diags.HasError(), "expected plan set without diagnostics, got %v", diags)
	return plan
}

func newUnknownM1ToM3Plan(t *testing.T) tfsdk.Plan {
	t.Helper()

	resourceSchema := m1ToM3Schema(t)
	return tfsdk.Plan{
		Schema: resourceSchema,
		Raw:    tftypes.NewValue(resourceSchema.Type().TerraformType(context.Background()), tftypes.UnknownValue),
	}
}

func newM1ToM3StateWithModel(t *testing.T, model netConnectM1ToM3Model) tfsdk.State {
	t.Helper()

	state := newM1ToM3State(t)
	diags := state.Set(context.Background(), &model)
	require.False(t, diags.HasError(), "expected state set without diagnostics, got %v", diags)
	return state
}

func newUnknownM1ToM3State(t *testing.T) tfsdk.State {
	t.Helper()

	resourceSchema := m1ToM3Schema(t)
	return tfsdk.State{
		Schema: resourceSchema,
		Raw:    tftypes.NewValue(resourceSchema.Type().TerraformType(context.Background()), tftypes.UnknownValue),
	}
}

func newM1ToM3State(t *testing.T) tfsdk.State {
	t.Helper()

	return tfsdk.State{Schema: m1ToM3Schema(t)}
}

func m1ToM3Schema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	(&netConnectM1ToM3Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	assert.False(t, resp.Diagnostics.HasError())
	return resp.Schema
}

func newM1ToM3CreateModel() netConnectM1ToM3Model {
	model := newM1ToM3Model()
	model.VpcepServiceId = types.StringNull()
	model.VpcepEndpointId = types.StringNull()
	model.VpcepEndpointIp = types.StringNull()
	model.VpcepEndpointServiceId = types.StringNull()
	model.LbmDnsRecordId = types.StringNull()
	model.LbmDnsRecordValues = types.ListNull(lbmDnsRecordValueObjectType)
	return model
}

func newExpectedM1ToM3VpcepServiceInput() *service.VpcepServiceInput {
	return &service.VpcepServiceInput{
		VpcId:      testM3VpcId,
		PortId:     testM3PortId,
		ServerType: testM3ServerType,
		Ports: []service.PortPair{
			{ClientPort: 80, ServerPort: 8080},
			{ClientPort: 443, ServerPort: 8443},
		},
	}
}

func newExpectedM1ToM3PermissionInputs() []service.PermissionInput {
	return []service.PermissionInput{
		{Permission: testIamDomainId},
		{Permission: testAnotherIamDomainId},
	}
}

func newExpectedM1ToM3EndpointInput() *service.VpcEndpointInput {
	return &service.VpcEndpointInput{
		EndpointServiceId: testVpcepServiceId,
		VpcId:             testM1PlusVpcId,
		SubnetId:          testM1PlusSubnetId,
	}
}

func newExpectedM1ToM3LbmDnsInput() *service.CreateLbmDnsInput {
	return &service.CreateLbmDnsInput{
		RegionCode:   testRegionCode,
		ServiceName:  testLbmDnsServiceName,
		HostRecord:   testDnsDomain,
		DomainSuffix: testDnsDomainSuffix,
		EndpointIp:   testVpcepEndpointIp,
	}
}

func newCreateLbmDnsOutput() *service.CreateLbmDnsOutput {
	return &service.CreateLbmDnsOutput{
		RecordId:     testLbmDnsRecordId,
		RecordValues: []service.LbmDnsRecordValue{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
	}
}
