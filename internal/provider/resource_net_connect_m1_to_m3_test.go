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
		name                         string
		endpointService              *mockVpcepEndpointService
		vpcepService                 *mockVpcepServiceService
		lbmDnsService                *mockLbmDnsService
		invalidPlan                  bool
		patchRecordValueDiags        bool
		expectedErr                  string
		expectedServiceCreateCalls   int
		expectedPermissionCalls      int
		expectedEndpointCreateCalls  int
		expectedDnsCreateCalls       int
		expectedEndpointDeleteIds    []string
		expectedServiceDeleteIds     []string
		expectedWarning              string
		expectedServiceInput         *service.VpcepServiceInput
		expectedPermissions          []service.PermissionInput
		expectedEndpointInput        *service.VpcEndpointInput
		expectedCreateLbmDnsInput    *service.CreateLbmDnsInput
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
			expectedPermissionCalls:      1,
			expectedEndpointCreateCalls:  1,
			expectedDnsCreateCalls:       1,
			expectedServiceInput:         newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:          newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:        newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:    newExpectedM1ToM3LbmDnsInput(),
			expectedLbmDnsRecordValues:   []lbmDnsRecordValueBlock{{RecordType: "A", RecordValue: testVpcepEndpointIp}},
			expectedStateVpcepServiceId:  testVpcepServiceId,
			expectedStateVpcepEndpointId: testVpcepEndpointId,
			expectedStateLbmDnsRecordId:  testLbmDnsRecordId,
		},
		{
			name:            "GIVEN invalid plan value WHEN Create SHOULD return diagnostics",
			endpointService: &mockVpcepEndpointService{},
			vpcepService:    &mockVpcepServiceService{},
			lbmDnsService:   &mockLbmDnsService{},
			invalidPlan:     true,
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
			expectedPermissionCalls:    1,
			expectedServiceInput:       newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:        newExpectedM1ToM3PermissionInputs(),
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
			expectedPermissionCalls:     1,
			expectedEndpointCreateCalls: 1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:         newExpectedM1ToM3PermissionInputs(),
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
			expectedPermissionCalls:     1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:         newExpectedM1ToM3PermissionInputs(),
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
			expectedPermissionCalls:     1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:         newExpectedM1ToM3PermissionInputs(),
			expectedEndpointInput:       newExpectedM1ToM3EndpointInput(),
			expectedCreateLbmDnsInput:   newExpectedM1ToM3LbmDnsInput(),
			expectedEndpointDeleteIds:   []string{testVpcepEndpointId},
			expectedServiceDeleteIds:    []string{testVpcepServiceId},
		},
		{
			name: "GIVEN dns create fails and rollback delete fails WHEN Create SHOULD return diagnostics with cleanup warning",
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
			expectedPermissionCalls:     1,
			expectedEndpointCreateCalls: 1,
			expectedDnsCreateCalls:      1,
			expectedServiceInput:        newExpectedM1ToM3VpcepServiceInput(),
			expectedPermissions:         newExpectedM1ToM3PermissionInputs(),
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
			if tc.invalidPlan {
				req.Plan = newInvalidM1ToM3Plan(t)
			}
			resp := &resource.CreateResponse{State: newM1ToM3State(t)}
			if tc.patchRecordValueDiags {
				patches := gomonkey.ApplyFunc(basetypes.NewObjectValue,
					func(map[string]attr.Type, map[string]attr.Value) (types.Object, diag.Diagnostics) {
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
			assert.Len(t, tc.vpcepService.addServiceIds, tc.expectedPermissionCalls)
			if tc.expectedPermissions != nil && assert.NotEmpty(t, tc.vpcepService.addPermissions) {
				assert.Equal(t, testVpcepServiceId, tc.vpcepService.addServiceIds[0])
				assert.Equal(t, tc.expectedPermissions, tc.vpcepService.addPermissions[0])
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

func TestNormalizeM1ToM3ListState(t *testing.T) {
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

func TestNormalizePortPairs(t *testing.T) {
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

func TestNormalizeVpcepServicePortBlocks(t *testing.T) {
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

func TestBuildLbmDnsRecordValues(t *testing.T) {
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

func TestNormalizeLbmDnsRecordValueBlocks(t *testing.T) {
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

func TestNormalizeVpcepServicePermissionBlocks(t *testing.T) {
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

func TestPreserveKnownComputedFields(t *testing.T) {
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

func TestServiceRequiresReplacement(t *testing.T) {
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

func TestServiceRequiresInPlaceUpdate(t *testing.T) {
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

func TestServicePortConfigChanged(t *testing.T) {
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

func TestServicePermissionsChanged(t *testing.T) {
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

func TestShouldReplaceEndpoint(t *testing.T) {
	testCases := []struct {
		name            string
		state           netConnectM1ToM3Model
		plan            netConnectM1ToM3Model
		serviceReplaced bool
		expected        bool
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
			name:            "GIVEN service replaced WHEN shouldReplaceEndpoint SHOULD return true",
			state:           newM1ToM3Model(),
			plan:            newM1ToM3Model(),
			serviceReplaced: true,
			expected:        true,
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
			actual := shouldReplaceEndpoint(tc.state, tc.plan, tc.serviceReplaced)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestDnsIdentityChanged(t *testing.T) {
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

func TestLbmDnsRecordValueNeedsUpdate(t *testing.T) {
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

func TestLbmDnsRecordAValue(t *testing.T) {
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

func TestConvertPorts(t *testing.T) {
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

func TestConvertPermissions(t *testing.T) {
	testCases := []struct {
		name     string
		input    []vpcepServicePermissionBlock
		expected []service.PermissionInput
	}{
		{
			name:  "GIVEN resource permission blocks WHEN convertPermissions SHOULD keep permission values",
			input: testVpcepServicePermissions(),
			expected: []service.PermissionInput{
				{Permission: "domain-id-a"},
				{Permission: "domain-id-b"},
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

func TestBuildLbmDnsRecordValuesErrorBranches(t *testing.T) {
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
			listValue: func(attr.Type, []attr.Value) (types.List, diag.Diagnostics) {
				var diags diag.Diagnostics
				diags.AddError("list value failed", "mock list value diagnostics")
				return types.ListUnknown(lbmDnsRecordValueObjectType), diags
			},
			expectedDiagSummary:        "list value failed",
			expectedDiagDetailContains: "mock list value diagnostics",
		},
		{
			name: "GIVEN object value diagnostics WHEN buildLbmDnsRecordValues SHOULD return unknown list and diagnostics",
			objectValue: func(map[string]attr.Type, map[string]attr.Value) (types.Object, diag.Diagnostics) {
				var diags diag.Diagnostics
				diags.AddError("object value failed", "mock object value diagnostics")
				return types.ObjectUnknown(lbmDnsRecordValueAttrTypes), diags
			},
			listValue: func(attr.Type, []attr.Value) (types.List, diag.Diagnostics) {
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

type mockVpcepEndpointService struct {
	createEndpointId string
	createEndpointIp string
	createErr        error
	deleteErr        error
	createInputs     []service.VpcEndpointInput
	deleteIds        []string
}

func (f *mockVpcepEndpointService) Create(ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createEndpointId, f.createEndpointIp, f.createErr
}

func (f *mockVpcepEndpointService) Delete(ctx context.Context, endpointId string) error {
	f.deleteIds = append(f.deleteIds, endpointId)
	return f.deleteErr
}

func (f *mockVpcepEndpointService) Get(ctx context.Context, endpointId string) (*service.VpcepEndpointOutput, error) {
	return nil, nil
}

type mockVpcepServiceService struct {
	createServiceId string
	createErr       error
	addErr          error
	deleteErr       error
	createInputs    []service.VpcepServiceInput
	deleteIds       []string
	addServiceIds   []string
	addPermissions  [][]service.PermissionInput
}

func (f *mockVpcepServiceService) Create(ctx context.Context, input service.VpcepServiceInput) (string, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createServiceId, f.createErr
}

func (f *mockVpcepServiceService) Delete(ctx context.Context, serviceId string) error {
	f.deleteIds = append(f.deleteIds, serviceId)
	return f.deleteErr
}

func (f *mockVpcepServiceService) Get(ctx context.Context, serviceId string) (*service.VpcepServiceOutput, error) {
	return nil, nil
}

func (f *mockVpcepServiceService) AddPermissions(ctx context.Context, serviceId string,
	permissions []service.PermissionInput) error {
	f.addServiceIds = append(f.addServiceIds, serviceId)
	f.addPermissions = append(f.addPermissions, permissions)
	return f.addErr
}

func (f *mockVpcepServiceService) GetPermissions(ctx context.Context, serviceId string) (map[string]string, error) {
	return nil, nil
}

func (f *mockVpcepServiceService) UpdateConfig(ctx context.Context, serviceId string,
	input service.VpcepServiceInput) error {
	return nil
}

func (f *mockVpcepServiceService) ReconcilePermissions(ctx context.Context, serviceId string,
	desired []service.PermissionInput) error {
	return nil
}

type mockLbmDnsService struct {
	createOutput *service.CreateLbmDnsOutput
	createErr    error
	createInputs []service.CreateLbmDnsInput
}

func (f *mockLbmDnsService) CreateIntranetDnsDomain(ctx context.Context,
	input service.CreateLbmDnsInput) (*service.CreateLbmDnsOutput, error) {
	f.createInputs = append(f.createInputs, input)
	return f.createOutput, f.createErr
}

func (f *mockLbmDnsService) DeleteIntranetDnsDomain(ctx context.Context, recordId string) error {
	return nil
}

func (f *mockLbmDnsService) UpdateRecordValue(ctx context.Context, recordId, endpointIp string) error {
	return nil
}

func (f *mockLbmDnsService) GetLbmDnsDetail(ctx context.Context,
	recordId string) (*service.LbmDnsDetailOutput, error) {
	return nil, nil
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

func newInvalidM1ToM3Plan(t *testing.T) tfsdk.Plan {
	t.Helper()

	resourceSchema := m1ToM3Schema(t)
	return tfsdk.Plan{
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
		{Permission: "domain-id-a"},
		{Permission: "domain-id-b"},
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
