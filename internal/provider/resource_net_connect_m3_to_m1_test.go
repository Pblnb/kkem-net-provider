/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

func m3ToM1ResourceSchema(t *testing.T) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	(&netConnectM3ToM1Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	assert.False(t, resp.Diagnostics.HasError())
	return resp.Schema
}

func newM3ToM1ResourcePlan(t *testing.T, model netConnectM3ToM1ResourceModel) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: m3ToM1ResourceSchema(t)}
	diags := plan.Set(context.Background(), &model)
	assert.False(t, diags.HasError(), "failed to set plan: %v", diags)
	return plan
}

func newM3ToM1ResourceState(t *testing.T) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: m3ToM1ResourceSchema(t)}
}

func Test_netConnectM3ToM1Resource_Schema(t *testing.T) {
	testCases := []struct {
		name     string
		resource resource.Resource
	}{
		{
			name:     "GIVEN net_connect_m3_to_m1 resource WHEN Schema called SHOULD return schema with all attributes",
			resource: NewNetConnectM3ToM1Resource(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req := resource.SchemaRequest{}
			resp := &resource.SchemaResponse{}

			tc.resource.Schema(ctx, req, resp)

			assert.False(t, resp.Diagnostics.HasError())
			assert.NotNil(t, resp.Schema.Attributes)

			expectedAttributes := map[string]struct {
				isRequired bool
				isComputed bool
			}{
				"m3_vpcep_id":           {isRequired: false, isComputed: true},
				"m3_vpc_id":             {isRequired: true, isComputed: false},
				"m3_vpcep_ip":           {isRequired: false, isComputed: true},
				"m3_vpcep_subnet_id":    {isRequired: true, isComputed: false},
				"sni_vpcep_server_id":   {isRequired: true, isComputed: false},
				"m3_dns_domain_name":    {isRequired: true, isComputed: false},
				"m3_dns_privatezone_id": {isRequired: false, isComputed: true},
				"sni_proxy_resource_id": {isRequired: false, isComputed: true},
				"region_code":           {isRequired: true, isComputed: false},
				"service_name":          {isRequired: true, isComputed: false},
				"m3_iam_domain_account": {isRequired: true, isComputed: false},
			}

			assert.Equal(t, len(expectedAttributes), len(resp.Schema.Attributes),
				"schema attribute count mismatch")

			for attrName, expected := range expectedAttributes {
				attr, ok := resp.Schema.Attributes[attrName]
				assert.True(t, ok, "schema should contain attribute: %s", attrName)

				if expected.isRequired {
					assert.True(t, attr.IsRequired(), "%s should be required", attrName)
				}
				if expected.isComputed {
					assert.True(t, attr.IsComputed(), "%s should be computed", attrName)
				}
			}
		})
	}
}

func Test_netConnectM3ToM1Resource_Configure(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		providerData interface{}
		expectedErr  bool
	}{
		{
			name:         "GIVEN nil provider data WHEN Configure SHOULD return without error",
			providerData: nil,
			expectedErr:  false,
		},
		{
			name:         "GIVEN invalid provider data type WHEN Configure SHOULD return error",
			providerData: "invalid type",
			expectedErr:  true,
		},
		{
			name:         "GIVEN valid clients struct WHEN Configure SHOULD initialize services",
			providerData: &clients{},
			expectedErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{}

			req := resource.ConfigureRequest{
				ProviderData: tc.providerData,
			}
			resp := &resource.ConfigureResponse{}

			r.Configure(ctx, req, resp)

			if tc.expectedErr {
				assert.True(t, resp.Diagnostics.HasError())
				assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "invalid provider data")
			} else {
				assert.False(t, resp.Diagnostics.HasError())
				if tc.providerData != nil {
					assert.NotNil(t, r.m3VpcepEndpointManager)
					assert.NotNil(t, r.m3DnsManager)
					assert.NotNil(t, r.m3SniProxyManager)
				}
			}
		})
	}
}

func Test_netConnectM3ToM1Resource_Metadata(t *testing.T) {
	testCases := []struct {
		name             string
		resource         resource.Resource
		providerTypeName string
		expectedTypeName string
	}{
		{
			name:             "GIVEN resource WHEN Metadata called SHOULD set correct type name",
			resource:         NewNetConnectM3ToM1Resource(),
			providerTypeName: "kkem",
			expectedTypeName: "kkem_net_connect_m3_to_m1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req := resource.MetadataRequest{ProviderTypeName: tc.providerTypeName}
			resp := &resource.MetadataResponse{}

			tc.resource.Metadata(ctx, req, resp)

			assert.Equal(t, tc.expectedTypeName, resp.TypeName)
		})
	}
}

func Test_netConnectM3ToM1Resource_Create_InvalidPlan(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		plan     tfsdk.Plan
		expected string
	}{
		{
			name: "GIVEN invalid plan WHEN Create SHOULD return diagnostics error",
			plan: tfsdk.Plan{
				Schema: m3ToM1ResourceSchema(t),
				Raw:    tftypes.NewValue(m3ToM1ResourceSchema(t).Type().TerraformType(ctx), tftypes.UnknownValue),
			},
			expected: "Value Conversion Error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: &mockVpcepEndpointManager{},
				m3DnsManager:           &mockM3ToM1DnsManager{},
				m3SniProxyManager:      &mockM3ToM1SniProxyManager{},
			}

			req := resource.CreateRequest{
				Plan: tc.plan,
			}
			resp := &resource.CreateResponse{
				State: tfsdk.State{
					Raw:    tc.plan.Raw.Copy(),
					Schema: tc.plan.Schema,
				},
			}

			r.Create(ctx, req, resp)

			assertErrorContains(t, resp.Diagnostics, tc.expected)
		})
	}
}

type wantState struct {
	sniProxyResourceId string
	m3VpcEndpointId    string
	m3VpcEndpointIp    string
	m3DnsPrivateZoneId string
}

type wantCalls struct {
	sniAccessCalls   int
	vpcepCreateCalls int
	dnsZoneCalls     int
	dnsRecordCalls   int
	sniDeleteCalls   int
	vpcepDeleteCalls int
	dnsDeleteCalls   int
}

func Test_netConnectM3ToM1Resource_Create_Success(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name string
		plan netConnectM3ToM1ResourceModel
		*mockM3ToM1SniProxyManager
		*mockVpcepEndpointManager
		*mockM3ToM1DnsManager
		wantState wantState
		wantCalls wantCalls
	}{
		{
			name: "GIVEN valid plan with domain WHEN Create SHOULD create all resources and set state",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				createPrivateZoneId: testDnsID,
				createRecordSetId:   testLbmDnsRecordId,
			},
			wantState: wantState{
				sniProxyResourceId: testSniProxyID,
				m3VpcEndpointId:    testVpcepEndpointId,
				m3VpcEndpointIp:    testVpcepEndpointIp,
				m3DnsPrivateZoneId: testDnsID,
			},
			wantCalls: wantCalls{
				sniAccessCalls:   1,
				vpcepCreateCalls: 1,
				dnsZoneCalls:     1,
				dnsRecordCalls:   1,
			},
		},
		{
			name: "GIVEN plan without domain name WHEN Create SHOULD skip DNS creation",
			plan: func() netConnectM3ToM1ResourceModel {
				plan := newM3ToM1ResourceModel()
				plan.M3DnsDomainName = types.StringNull()
				return plan
			}(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantState: wantState{
				sniProxyResourceId: testSniProxyID,
				m3VpcEndpointId:    testVpcepEndpointId,
				m3VpcEndpointIp:    testVpcepEndpointIp,
			},
			wantCalls: wantCalls{
				sniAccessCalls:   1,
				vpcepCreateCalls: 1,
				dnsZoneCalls:     0,
				dnsRecordCalls:   0,
			},
		},
		{
			name: "GIVEN empty domain name WHEN Create SHOULD skip DNS creation",
			plan: func() netConnectM3ToM1ResourceModel {
				plan := newM3ToM1ResourceModel()
				plan.M3DnsDomainName = types.StringValue("")
				return plan
			}(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantState: wantState{
				sniProxyResourceId: testSniProxyID,
				m3VpcEndpointId:    testVpcepEndpointId,
				m3VpcEndpointIp:    testVpcepEndpointIp,
			},
			wantCalls: wantCalls{
				sniAccessCalls:   1,
				vpcepCreateCalls: 1,
				dnsZoneCalls:     0,
				dnsRecordCalls:   0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.mockVpcepEndpointManager,
				m3DnsManager:           tc.mockM3ToM1DnsManager,
				m3SniProxyManager:      tc.mockM3ToM1SniProxyManager,
			}

			req := resource.CreateRequest{Plan: newM3ToM1ResourcePlan(t, tc.plan)}
			resp := &resource.CreateResponse{State: newM3ToM1ResourceState(t)}

			r.Create(ctx, req, resp)

			assert.False(t, resp.Diagnostics.HasError())

			var state netConnectM3ToM1ResourceModel
			resp.State.Get(ctx, &state)

			assert.Equal(t, tc.wantState.sniProxyResourceId, state.SniProxyResourceId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcEndpointId, state.M3VpcEndpointId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcEndpointIp, state.M3VpcEndpointIp.ValueString())
			if tc.wantState.m3DnsPrivateZoneId != "" {
				assert.Equal(t, tc.wantState.m3DnsPrivateZoneId, state.M3DnsPrivateZoneId.ValueString())
			} else {
				assert.True(t, state.M3DnsPrivateZoneId.IsUnknown() || state.M3DnsPrivateZoneId.IsNull())
			}

			assert.Equal(t, tc.wantCalls.sniAccessCalls, len(tc.mockM3ToM1SniProxyManager.accessInputs))
			assert.Equal(t, tc.wantCalls.vpcepCreateCalls, len(tc.mockVpcepEndpointManager.createInputs))
			assert.Equal(t, tc.wantCalls.dnsZoneCalls, len(tc.mockM3ToM1DnsManager.zoneInputs))
			assert.Equal(t, tc.wantCalls.dnsRecordCalls, len(tc.mockM3ToM1DnsManager.recordInputs))
		})
	}
}

func Test_netConnectM3ToM1Resource_Create_Failure_And_Rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name string
		plan netConnectM3ToM1ResourceModel
		*mockM3ToM1SniProxyManager
		*mockVpcepEndpointManager
		*mockM3ToM1DnsManager
		expectedErrContains string
		expectWarning       bool
	}{
		{
			name: "GIVEN sni proxy creation fails WHEN Create SHOULD return error without creating other resources",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessErr: errors.New("sni proxy access failed"),
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:     &mockM3ToM1DnsManager{},
			expectedErrContains:      "create sni-proxy failed",
		},
		{
			name: "GIVEN vpcep endpoint creation fails WHEN Create SHOULD rollback sni proxy",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createErr: errors.New("vpcep endpoint creation failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrContains:  "create vpc-endpoint failed",
		},
		{
			name: "GIVEN dns private zone creation fails WHEN Create SHOULD rollback vpcep and sni proxy",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				createZoneErr: errors.New("dns private zone creation failed"),
			},
			expectedErrContains: "create M3 intranet domain failed",
		},
		{
			name: "GIVEN dns record set creation fails WHEN Create SHOULD rollback all created resources",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				createPrivateZoneId: testDnsID,
				createRecordErr:     errors.New("dns record set creation failed"),
			},
			expectedErrContains: "create M3 intranet domain record set failed",
		},
		{
			name: "GIVEN vpcep creation fails and rollback also fails WHEN Create SHOULD return warning about rollback failure",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyID,
				deleteErr:        errors.New("sni rollback failed"),
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				createErr: errors.New("vpcep creation failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrContains:  "create vpc-endpoint failed",
			expectWarning:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.mockVpcepEndpointManager,
				m3DnsManager:           tc.mockM3ToM1DnsManager,
				m3SniProxyManager:      tc.mockM3ToM1SniProxyManager,
			}

			req := resource.CreateRequest{Plan: newM3ToM1ResourcePlan(t, tc.plan)}
			resp := &resource.CreateResponse{State: newM3ToM1ResourceState(t)}

			r.Create(ctx, req, resp)

			assertErrorContains(t, resp.Diagnostics, tc.expectedErrContains)

			if tc.expectWarning {
				assert.Len(t, resp.Diagnostics.Warnings(), 1)
				assert.Contains(t, resp.Diagnostics.Warnings()[0].Detail(), "sni rollback failed")
			}
		})
	}
}

func assertErrorContains(t *testing.T, diags diag.Diagnostics, expected string) {
	t.Helper()
	assert.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Summary(), expected)
}

func Test_netConnectM3ToM1Resource_rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name    string
		created []m3ToM1CreatedChildResource
		*mockM3ToM1SniProxyManager
		*mockVpcepEndpointManager
		*mockM3ToM1DnsManager
		expectedErrCount    int
		expectedErrContains string
	}{
		{
			name:                      "GIVEN empty created list WHEN rollback SHOULD return no errors",
			created:                   []m3ToM1CreatedChildResource{},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager:  &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          0,
		},
		{
			name: "GIVEN all delete success WHEN rollback SHOULD return no errors",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepEndpointType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager:  &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          0,
		},
		{
			name: "GIVEN dns delete fails WHEN rollback SHOULD return dns error",
			created: []m3ToM1CreatedChildResource{
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager:  &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsType,
		},
		{
			name: "GIVEN vpcep delete fails WHEN rollback SHOULD return vpcep error",
			created: []m3ToM1CreatedChildResource{
				{Type: vpcepEndpointType, ID: testVpcepID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				deleteErr: errors.New("vpcep delete failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + vpcepEndpointType,
		},
		{
			name: "GIVEN sni delete fails WHEN rollback SHOULD return sni error",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni delete failed"),
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:     &mockM3ToM1DnsManager{},
			expectedErrCount:         1,
			expectedErrContains:      "delete " + sniProxyType,
		},
		{
			name: "GIVEN vpcep delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepEndpointType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				deleteErr: errors.New("vpcep delete failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + vpcepEndpointType,
		},
		{
			name: "GIVEN dns delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepEndpointType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager:  &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsType,
		},
		{
			name: "GIVEN sni delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepEndpointType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni delete failed"),
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:     &mockM3ToM1DnsManager{},
			expectedErrCount:         1,
			expectedErrContains:      "delete " + sniProxyType,
		},
		{
			name: "GIVEN all three delete fail WHEN rollback SHOULD return all three errors",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepEndpointType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni failed"),
			},
			mockVpcepEndpointManager: &mockVpcepEndpointManager{
				deleteErr: errors.New("vpcep failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				deletePrivateErr: errors.New("dns failed"),
			},
			expectedErrCount: 3,
		},
		{
			name: "GIVEN unknown resource type WHEN rollback SHOULD return error",
			created: []m3ToM1CreatedChildResource{
				{Type: "unknown_type", ID: "unknown-id"},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			mockVpcepEndpointManager:  &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          1,
			expectedErrContains:       "unknown resource type: unknown_type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.mockVpcepEndpointManager,
				m3DnsManager:           tc.mockM3ToM1DnsManager,
				m3SniProxyManager:      tc.mockM3ToM1SniProxyManager,
			}

			errs := r.rollback(ctx, tc.created)

			assert.Equal(t, tc.expectedErrCount, len(errs))
			if tc.expectedErrContains != "" && len(errs) > 0 {
				assert.Contains(t, errs[0].Error(), tc.expectedErrContains)
			}
			if tc.expectedErrCount > 1 {
				assert.Contains(t, errs[0].Error(), dnsType)
				assert.Contains(t, errs[1].Error(), vpcepEndpointType)
				assert.Contains(t, errs[2].Error(), sniProxyType)
			}
		})
	}
}

func newM3ToM1ResourceModel() netConnectM3ToM1ResourceModel {
	return netConnectM3ToM1ResourceModel{
		M3VpcID:               types.StringValue(testM3VpcId),
		M3VpcEndpointSubnetId: types.StringValue(testM3SubnetId),
		SniVpcepServerId:      types.StringValue(testVpcepServiceId),
		M3DnsDomainName:       types.StringValue(testDomainName),
		RegionCode:            types.StringValue(testRegionCode),
		ServiceName:           types.StringValue(testServiceName),
		DomainAccount:         types.StringValue(testDomainAccount),
	}
}
