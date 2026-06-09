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
	"github.com/stretchr/testify/require"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/manager"
)

func m3ToM1Schema(t *testing.T) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	(&netConnectM3ToM1Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	return resp.Schema
}

func m3ToM1ResourceSchema(t *testing.T) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	(&netConnectM3ToM1Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError())
	return resp.Schema
}

func newM3ToM1ResourcePlan(t *testing.T, model netConnectM3ToM1ResourceModel) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: m3ToM1ResourceSchema(t)}
	diags := plan.Set(context.Background(), &model)
	require.False(t, diags.HasError(), "failed to set plan: %v", diags)
	return plan
}

func newM3ToM1ResourceState(t *testing.T) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: m3ToM1ResourceSchema(t)}
}

func newM3ToM1ResourceStateWithModel(t *testing.T, model netConnectM3ToM1ResourceModel) tfsdk.State {
	t.Helper()

	state := newM3ToM1ResourceState(t)
	diags := state.Set(context.Background(), &model)
	require.False(t, diags.HasError(), "failed to set state: %v", diags)
	return state
}

func newInvalidM3ToM1ResourceState(t *testing.T) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: m3ToM1Schema(t)}
	state.Raw = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	return state
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

// wantState captures expected state after Create completes.
type wantState struct {
	sniProxyResourceId string
	m3VpcepEndpointId  string
	m3VpcepEndpointIp  string
	m3DnsPrivateZoneId string
}

// createWantCalls captures expected service call counts after Create completes.
type createWantCalls struct {
	sniAccessCalls            int
	vpcependpointCreateCalls  int
	dnsCreatePrivateZoneCalls int
	dnsCreateRecordSetCalls   int
	sniDeleteCalls            int
	vpcependpointDeleteCalls  int
	dnsDeleteCalls            int
}

func Test_netConnectM3ToM1Resource_Create_Success(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                      string
		plan                      netConnectM3ToM1ResourceModel
		mockM3ToM1SniProxyManager *mockM3ToM1SniProxyManager
		vpcepEndpointMock         *mockVpcepEndpointManager
		mockM3ToM1DnsManager      *mockM3ToM1DnsManager
		wantState                 wantState
		wantCalls                 createWantCalls
	}{
		{
			name: "GIVEN valid plan with domain WHEN Create SHOULD create all resources and set state",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				createPrivateZoneId: testDnsId,
				createRecordSetId:   testLbmDnsRecordId,
			},
			wantState: wantState{
				sniProxyResourceId: testSniProxyId,
				m3VpcepEndpointId:  testVpcepEndpointId,
				m3VpcepEndpointIp:  testVpcepEndpointIp,
				m3DnsPrivateZoneId: testDnsId,
			},
			wantCalls: createWantCalls{
				sniAccessCalls:            1,
				vpcependpointCreateCalls:  1,
				dnsCreatePrivateZoneCalls: 1,
				dnsCreateRecordSetCalls:   1,
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
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantState: wantState{
				sniProxyResourceId: testSniProxyId,
				m3VpcepEndpointId:  testVpcepEndpointId,
				m3VpcepEndpointIp:  testVpcepEndpointIp,
			},
			wantCalls: createWantCalls{
				sniAccessCalls:            1,
				vpcependpointCreateCalls:  1,
				dnsCreatePrivateZoneCalls: 0,
				dnsCreateRecordSetCalls:   0,
			},
		},
		{
			name: "GIVEN empty domain name WHEN Create SHOULD skip DNS creation",
			plan: func() netConnectM3ToM1ResourceModel {
				plan := newM3ToM1ResourceModel()
				plan.M3DnsDomainName = types.StringNull()
				return plan
			}(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantState: wantState{
				sniProxyResourceId: testSniProxyId,
				m3VpcepEndpointId:  testVpcepEndpointId,
				m3VpcepEndpointIp:  testVpcepEndpointIp,
			},
			wantCalls: createWantCalls{
				sniAccessCalls:            1,
				vpcependpointCreateCalls:  1,
				dnsCreatePrivateZoneCalls: 0,
				dnsCreateRecordSetCalls:   0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.vpcepEndpointMock,
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
			assert.Equal(t, tc.wantState.m3VpcepEndpointId, state.M3VpcepEndpointId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcepEndpointIp, state.M3VpcepEndpointIp.ValueString())
			if tc.wantState.m3DnsPrivateZoneId != "" {
				assert.Equal(t, tc.wantState.m3DnsPrivateZoneId, state.M3DnsPrivateZoneId.ValueString())
			} else {
				assert.True(t, state.M3DnsPrivateZoneId.IsUnknown() || state.M3DnsPrivateZoneId.IsNull())
			}

			assert.Equal(t, tc.wantCalls.sniAccessCalls, len(tc.mockM3ToM1SniProxyManager.accessInputs))
			assert.Equal(t, tc.wantCalls.vpcependpointCreateCalls, len(tc.vpcepEndpointMock.createInputs))
			assert.Equal(t, tc.wantCalls.dnsCreatePrivateZoneCalls, len(tc.mockM3ToM1DnsManager.zoneInputs))
			assert.Equal(t, tc.wantCalls.dnsCreateRecordSetCalls, len(tc.mockM3ToM1DnsManager.recordInputs))
		})
	}
}

func Test_netConnectM3ToM1Resource_Create_Failure_And_Rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                      string
		plan                      netConnectM3ToM1ResourceModel
		mockM3ToM1SniProxyManager *mockM3ToM1SniProxyManager
		vpcepEndpointMock         *mockVpcepEndpointManager
		mockM3ToM1DnsManager      *mockM3ToM1DnsManager
		expectedErrContains       string
		expectWarning             bool
	}{
		{
			name: "GIVEN sni proxy creation fails WHEN Create SHOULD return error without creating other resources",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessErr: errors.New("sni proxy access failed"),
			},
			vpcepEndpointMock:    &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrContains:  "create sni-proxy failed",
		},
		{
			name: "GIVEN vpcep endpoint creation fails WHEN Create SHOULD rollback sni proxy",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createErr: errors.New("vpcep endpoint creation failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrContains:  "create vpc-endpoint failed",
		},
		{
			name: "GIVEN dns private zone creation fails WHEN Create SHOULD rollback vpcep endpoint and sni proxy",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
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
				accessResourceId: testSniProxyId,
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createEndpointId: testVpcepEndpointId,
				createEndpointIp: testVpcepEndpointIp,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				createPrivateZoneId: testDnsId,
				createRecordErr:     errors.New("dns record set creation failed"),
			},
			expectedErrContains: "create M3 intranet domain record set failed",
		},
		{
			name: "GIVEN vpcep endpoint creation fails and rollback also fails WHEN Create SHOULD return warning about rollback failure",
			plan: newM3ToM1ResourceModel(),
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				accessResourceId: testSniProxyId,
				deleteErr:        errors.New("sni rollback failed"),
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				createErr: errors.New("vpcep endpoint creation failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrContains:  "create vpc-endpoint failed",
			expectWarning:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.vpcepEndpointMock,
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
		name                      string
		created                   []m3ToM1CreatedChildResource
		mockM3ToM1SniProxyManager *mockM3ToM1SniProxyManager
		vpcepEndpointMock         *mockVpcepEndpointManager
		mockM3ToM1DnsManager      *mockM3ToM1DnsManager
		expectedErrCount          int
		expectedErrContains       string
	}{
		{
			name:                      "GIVEN empty created list WHEN rollback SHOULD return no errors",
			created:                   []m3ToM1CreatedChildResource{},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          0,
		},
		{
			name: "GIVEN all delete success WHEN rollback SHOULD return no errors",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          0,
		},
		{
			name: "GIVEN dns delete fails WHEN rollback SHOULD return dns error",
			created: []m3ToM1CreatedChildResource{
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsPrivateZoneType,
		},
		{
			name: "GIVEN vpcep endpoint delete fails WHEN rollback SHOULD return error",
			created: []m3ToM1CreatedChildResource{
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				deleteErr: errors.New("vpcep endpoint delete failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + vpcepEndpointType,
		},
		{
			name: "GIVEN sni delete fails WHEN rollback SHOULD return sni error",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni delete failed"),
			},
			vpcepEndpointMock:    &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + sniProxyType,
		},
		{
			name: "GIVEN vpcep endpoint delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				deleteErr: errors.New("vpcep endpoint delete failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + vpcepEndpointType,
		},
		{
			name: "GIVEN dns delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsPrivateZoneType,
		},
		{
			name: "GIVEN sni delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni delete failed"),
			},
			vpcepEndpointMock:    &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			expectedErrCount:     1,
			expectedErrContains:  "delete " + sniProxyType,
		},
		{
			name: "GIVEN all three delete fail WHEN rollback SHOULD return all three errors",
			created: []m3ToM1CreatedChildResource{
				{Type: sniProxyType, ID: testSniProxyId},
				{Type: vpcepEndpointType, ID: testVpcepEndpointId},
				{Type: dnsPrivateZoneType, ID: testDnsId},
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				deleteErr: errors.New("sni failed"),
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
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
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectedErrCount:          1,
			expectedErrContains:       "unknown resource type: unknown_type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.vpcepEndpointMock,
				m3DnsManager:           tc.mockM3ToM1DnsManager,
				m3SniProxyManager:      tc.mockM3ToM1SniProxyManager,
			}

			errs := r.rollback(ctx, tc.created)

			assert.Equal(t, tc.expectedErrCount, len(errs))
			if tc.expectedErrContains != "" && len(errs) > 0 {
				assert.Contains(t, errs[0].Error(), tc.expectedErrContains)
			}
			if tc.expectedErrCount > 1 {
				assert.Contains(t, errs[0].Error(), dnsPrivateZoneType)
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

func newStateModel() netConnectM3ToM1ResourceModel {
	return netConnectM3ToM1ResourceModel{
		SniProxyResourceId:    types.StringValue(testSniProxyId),
		M3VpcepEndpointId:     types.StringValue(testVpcepEndpointId),
		M3VpcepEndpointIp:     types.StringValue(testVpcepEndpointIp),
		M3DnsPrivateZoneId:    types.StringValue(testDnsId),
		M3VpcID:               types.StringValue(testM3VpcId),
		M3VpcEndpointSubnetId: types.StringValue(testM3SubnetId),
		SniVpcepServerId:      types.StringValue(testVpcepServiceId),
		M3DnsDomainName:       types.StringValue(testDomainName),
		RegionCode:            types.StringValue(testRegionCode),
		ServiceName:           types.StringValue(testServiceName),
		DomainAccount:         types.StringValue(testDomainAccount),
	}
}

// readWantCalls captures expected service call counts and input IDs.
type readWantCalls struct {
	sniGetCalls           int
	vpcepEndpointGetCalls int
	dnsGetCalls           int
	sniGetId              string
	vpcepEndpointGetId    string
	dnsGetId              string
}

// readWantState captures the expected state after Read completes.
type readWantState struct {
	sniProxyResourceId string
	m3VpcepEndpointId  string
	m3VpcepEndpointIp  string
	m3DnsPrivateZoneId string
	removed            bool
}

func Test_netConnectM3ToM1Resource_Read(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name                      string
		stateModel                *netConnectM3ToM1ResourceModel
		mockM3ToM1SniProxyManager *mockM3ToM1SniProxyManager
		vpcepEndpointMock         *mockVpcepEndpointManager
		mockM3ToM1DnsManager      *mockM3ToM1DnsManager
		invalidState              bool
		wantCalls                 readWantCalls
		wantState                 readWantState
		expectErr                 bool
		expectedErrContains       string
		wantNullFields            []string
	}{
		{
			name: "GIVEN all resources exist WHEN Read SHOULD keep state unchanged",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: testSniProxyId},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: &manager.VpcepEndpointOutput{EndpointId: testVpcepEndpointId, Ip: testVpcepEndpointIp},
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				sniProxyResourceId: testSniProxyId, m3VpcepEndpointId: testVpcepEndpointId,
				m3VpcepEndpointIp: testVpcepEndpointIp, m3DnsPrivateZoneId: testDnsId,
			},
		},
		{
			name: "GIVEN SNI proxy is missing WHEN Read SHOULD mark SNI field as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getAccessServiceOutput: &sniproxyclient.GetAccessServiceResponse{
					Body: sniproxyclient.GetAccessServiceResponseBody{
						BaseResponse: sniproxyclient.BaseResponse{Code: 6082},
					},
				},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: &manager.VpcepEndpointOutput{EndpointId: testVpcepEndpointId, Ip: testVpcepEndpointIp},
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				m3VpcepEndpointId: testVpcepEndpointId, m3VpcepEndpointIp: testVpcepEndpointIp, m3DnsPrivateZoneId: testDnsId,
			},
		},
		{
			name: "GIVEN DNS private zone is missing WHEN Read SHOULD mark DNS field as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: testSniProxyId},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: &manager.VpcepEndpointOutput{EndpointId: testVpcepEndpointId, Ip: testVpcepEndpointIp},
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				sniProxyResourceId: testSniProxyId, m3VpcepEndpointId: testVpcepEndpointId, m3VpcepEndpointIp: testVpcepEndpointIp,
			},
		},
		{
			name: "GIVEN VPCEP endpoint is missing WHEN Read SHOULD mark VPCEP fields as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: testSniProxyId},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: nil,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				sniProxyResourceId: testSniProxyId, m3DnsPrivateZoneId: testDnsId,
			},
		},
		{
			name: "GIVEN SNI and DNS resources are missing WHEN Read SHOULD mark both fields as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getAccessServiceOutput: &sniproxyclient.GetAccessServiceResponse{
					Body: sniproxyclient.GetAccessServiceResponseBody{
						BaseResponse: sniproxyclient.BaseResponse{Code: 6082},
					},
				},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: &manager.VpcepEndpointOutput{EndpointId: testVpcepEndpointId, Ip: testVpcepEndpointIp},
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				m3VpcepEndpointId: testVpcepEndpointId, m3VpcepEndpointIp: testVpcepEndpointIp,
			},
		},
		{
			name: "GIVEN SNI and VPCEP resources are missing WHEN Read SHOULD mark both fields as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getAccessServiceOutput: &sniproxyclient.GetAccessServiceResponse{
					Body: sniproxyclient.GetAccessServiceResponseBody{
						BaseResponse: sniproxyclient.BaseResponse{Code: 6082},
					},
				},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: nil,
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				m3DnsPrivateZoneId: testDnsId,
			},
		},
		{
			name: "GIVEN vpcep and dns resources are missing WHEN Read SHOULD remove resource from state",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getAccessServiceOutput: &sniproxyclient.GetAccessServiceResponse{
					Body: sniproxyclient.GetAccessServiceResponseBody{
						BaseResponse: sniproxyclient.BaseResponse{Code: 6082},
					},
				},
			},
			vpcepEndpointMock:    &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{removed: true},
		},
		{
			name:                      "GIVEN invalid state WHEN Read SHOULD return diagnostics error",
			invalidState:              true,
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{},
			vpcepEndpointMock:         &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager:      &mockM3ToM1DnsManager{},
			expectErr:                 true,
			expectedErrContains:       "Value Conversion Error",
		},
		{
			name: "GIVEN SNI Get returns empty ResourceId WHEN Read SHOULD mark SNI-related fields as null",
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: ""},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getOutput: &manager.VpcepEndpointOutput{EndpointId: testVpcepEndpointId, Ip: testVpcepEndpointIp},
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			wantState: readWantState{
				sniProxyResourceId: "", m3VpcepEndpointId: testVpcepEndpointId,
				m3VpcepEndpointIp: testVpcepEndpointIp, m3DnsPrivateZoneId: testDnsId,
			},
			wantNullFields: []string{"ServiceName", "RegionCode", "DomainAccount"},
		},
		{
			name: "GIVEN Get SNI fails WHEN Read SHOULD return error",
			stateModel: &netConnectM3ToM1ResourceModel{
				SniProxyResourceId: types.StringValue(testSniProxyId),
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getSniProxyErr: errors.New("get sni failed"),
			},
			vpcepEndpointMock:    &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{},
			wantCalls:            readWantCalls{sniGetCalls: 1, sniGetId: testSniProxyId},
			expectErr:            true, expectedErrContains: "Failed to get SNI proxy",
		},
		{
			name: "GIVEN Get DNS fails WHEN Read SHOULD return error",
			stateModel: &netConnectM3ToM1ResourceModel{
				SniProxyResourceId: types.StringValue(testSniProxyId),
				M3DnsPrivateZoneId: types.StringValue(testDnsId),
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: testSniProxyId},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput:  nil,
				getZoneErr: errors.New("get dns failed"),
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 0, dnsGetCalls: 1,
				sniGetId: testSniProxyId, dnsGetId: testDnsId,
			},
			expectErr: true, expectedErrContains: "query intranet domain failed",
		},
		{
			name: "GIVEN Get VPCEP endpoint fails WHEN Read SHOULD return error",
			stateModel: &netConnectM3ToM1ResourceModel{
				SniProxyResourceId: types.StringValue(testSniProxyId),
				M3VpcepEndpointId:  types.StringValue(testVpcepEndpointId),
				M3DnsPrivateZoneId: types.StringValue(testDnsId),
			},
			mockM3ToM1SniProxyManager: &mockM3ToM1SniProxyManager{
				getOutput: &manager.AccessSniProxyOutput{ResourceId: testSniProxyId},
			},
			vpcepEndpointMock: &mockVpcepEndpointManager{
				getErr: errors.New("get vpcep endpoint failed"),
			},
			mockM3ToM1DnsManager: &mockM3ToM1DnsManager{
				getOutput: &manager.DnsZoneOutput{ZoneId: testDnsId},
			},
			wantCalls: readWantCalls{
				sniGetCalls: 1, vpcepEndpointGetCalls: 1, dnsGetCalls: 1,
				sniGetId: testSniProxyId, vpcepEndpointGetId: testVpcepEndpointId, dnsGetId: testDnsId,
			},
			expectErr: true, expectedErrContains: "query vpc-endpoint failed",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepEndpointManager: tc.vpcepEndpointMock,
				m3DnsManager:           tc.mockM3ToM1DnsManager,
				m3SniProxyManager:      tc.mockM3ToM1SniProxyManager,
			}
			var req resource.ReadRequest
			if tc.invalidState {
				req.State = newInvalidM3ToM1ResourceState(t)
			} else if tc.stateModel != nil {
				req.State = newM3ToM1ResourceStateWithModel(t, *tc.stateModel)
			} else {
				req.State = newM3ToM1ResourceStateWithModel(t, newStateModel())
			}
			resp := &resource.ReadResponse{State: newM3ToM1ResourceState(t)}
			r.Read(ctx, req, resp)

			assert.Equal(t, tc.wantCalls.sniGetCalls, tc.mockM3ToM1SniProxyManager.getCalls)
			assert.Equal(t, tc.wantCalls.vpcepEndpointGetCalls, tc.vpcepEndpointMock.getCalls)
			assert.Equal(t, tc.wantCalls.dnsGetCalls, tc.mockM3ToM1DnsManager.getCalls)
			assert.Equal(t, tc.wantCalls.sniGetId, tc.mockM3ToM1SniProxyManager.getId)
			assert.Equal(t, tc.wantCalls.vpcepEndpointGetId, tc.vpcepEndpointMock.getId)
			assert.Equal(t, tc.wantCalls.dnsGetId, tc.mockM3ToM1DnsManager.getId)

			if tc.expectErr {
				require.True(t, resp.Diagnostics.HasError())
				require.NotEmpty(t, resp.Diagnostics.Errors())
				assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), tc.expectedErrContains)
				return
			}

			assert.False(t, resp.Diagnostics.HasError())
			if tc.wantState.removed {
				assert.True(t, resp.State.Raw.IsNull())
				return
			}

			var state netConnectM3ToM1ResourceModel
			diags := resp.State.Get(ctx, &state)
			require.False(t, diags.HasError(), "expected state get without diagnostics, got %v", diags)
			assert.Equal(t, tc.wantState.sniProxyResourceId, state.SniProxyResourceId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcepEndpointId, state.M3VpcepEndpointId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcepEndpointIp, state.M3VpcepEndpointIp.ValueString())
			assert.Equal(t, tc.wantState.m3DnsPrivateZoneId, state.M3DnsPrivateZoneId.ValueString())

			for _, field := range tc.wantNullFields {
				switch field {
				case "ServiceName":
					assert.True(t, state.ServiceName.IsNull(), "ServiceName should be null")
				case "RegionCode":
					assert.True(t, state.RegionCode.IsNull(), "RegionCode should be null")
				case "DomainAccount":
					assert.True(t, state.DomainAccount.IsNull(), "DomainAccount should be null")
				}
			}
		})
	}
}
