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

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/service"
)

func m3ToM1Schema(t *testing.T) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	(&netConnectM3ToM1Resource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	assert.False(t, resp.Diagnostics.HasError())
	return resp.Schema
}

func newM3ToM1Plan(t *testing.T, model netConnectM3ToM1Model) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: m3ToM1Schema(t)}
	diags := plan.Set(context.Background(), &model)
	assert.False(t, diags.HasError(), "failed to set plan: %v", diags)
	return plan
}

func newM3ToM1State(t *testing.T) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: m3ToM1Schema(t)}
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
					assert.NotNil(t, r.m3VpcepService)
					assert.NotNil(t, r.m3DnsService)
					assert.NotNil(t, r.m3SniProxyService)
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

func TestNetConnectM3ToM1Resource_Create_InvalidPlan(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		plan     tfsdk.Plan
		expected string
	}{
		{
			name: "GIVEN invalid plan WHEN Create SHOULD return diagnostics error",
			plan: tfsdk.Plan{
				Schema: m3ToM1Schema(t),
				Raw:    tftypes.NewValue(m3ToM1Schema(t).Type().TerraformType(ctx), tftypes.UnknownValue),
			},
			expected: "Value Conversion Error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepService:    &mockM3ToM1VpcepEndpointService{},
				m3DnsService:      &mockM3ToM1DnsService{},
				m3SniProxyService: &mockM3ToM1SniProxyService{},
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

func TestNetConnectM3ToM1Resource_Create_Success(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name      string
		plan      netConnectM3ToM1Model
		sniMock   *mockM3ToM1SniProxyService
		vpcepMock *mockM3ToM1VpcepEndpointService
		dnsMock   *mockM3ToM1DnsService
		wantState wantState
		wantCalls wantCalls
	}{
		{
			name: "GIVEN valid plan with domain WHEN Create SHOULD create all resources and set state",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createIp:         testVpcepEndpointIp,
			},
			dnsMock: &mockM3ToM1DnsService{
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
			plan: func() netConnectM3ToM1Model {
				plan := newTestPlan()
				plan.M3DnsDomainName = types.StringNull()
				return plan
			}(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createIp:         testVpcepEndpointIp,
			},
			dnsMock: &mockM3ToM1DnsService{},
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
			plan: func() netConnectM3ToM1Model {
				plan := newTestPlan()
				plan.M3DnsDomainName = types.StringValue("")
				return plan
			}(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createIp:         testVpcepEndpointIp,
			},
			dnsMock: &mockM3ToM1DnsService{},
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
				m3VpcepService:    tc.vpcepMock,
				m3DnsService:      tc.dnsMock,
				m3SniProxyService: tc.sniMock,
			}

			req := resource.CreateRequest{Plan: newM3ToM1Plan(t, tc.plan)}
			resp := &resource.CreateResponse{State: newM3ToM1State(t)}

			r.Create(ctx, req, resp)

			assert.False(t, resp.Diagnostics.HasError())

			var state netConnectM3ToM1Model
			resp.State.Get(ctx, &state)

			assert.Equal(t, tc.wantState.sniProxyResourceId, state.SniProxyResourceId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcEndpointId, state.M3VpcEndpointId.ValueString())
			assert.Equal(t, tc.wantState.m3VpcEndpointIp, state.M3VpcEndpointIp.ValueString())
			if tc.wantState.m3DnsPrivateZoneId != "" {
				assert.Equal(t, tc.wantState.m3DnsPrivateZoneId, state.M3DnsPrivateZoneId.ValueString())
			} else {
				assert.True(t, state.M3DnsPrivateZoneId.IsUnknown() || state.M3DnsPrivateZoneId.IsNull())
			}

			assert.Equal(t, tc.wantCalls.sniAccessCalls, len(tc.sniMock.accessInputs))
			assert.Equal(t, tc.wantCalls.vpcepCreateCalls, len(tc.vpcepMock.createInputs))
			assert.Equal(t, tc.wantCalls.dnsZoneCalls, len(tc.dnsMock.zoneInputs))
			assert.Equal(t, tc.wantCalls.dnsRecordCalls, len(tc.dnsMock.recordInputs))
		})
	}
}

func TestNetConnectM3ToM1Resource_Create_Failure_And_Rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		plan                netConnectM3ToM1Model
		sniMock             *mockM3ToM1SniProxyService
		vpcepMock           *mockM3ToM1VpcepEndpointService
		dnsMock             *mockM3ToM1DnsService
		expectedErrContains string
		expectWarning       bool
	}{
		{
			name: "GIVEN sni proxy creation fails WHEN Create SHOULD return error without creating other resources",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessErr: errors.New("sni proxy access failed"),
			},
			vpcepMock:           &mockM3ToM1VpcepEndpointService{},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrContains: "create sni-proxy failed",
		},
		{
			name: "GIVEN vpcep endpoint creation fails WHEN Create SHOULD rollback sni proxy",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createErr: errors.New("vpcep endpoint creation failed"),
			},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrContains: "create vpc-endpoint failed",
		},
		{
			name: "GIVEN dns private zone creation fails WHEN Create SHOULD rollback vpcep and sni proxy",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createIp:         testVpcepEndpointIp,
			},
			dnsMock: &mockM3ToM1DnsService{
				createZoneErr: errors.New("dns private zone creation failed"),
			},
			expectedErrContains: "create M3 intranet domain failed",
		},
		{
			name: "GIVEN dns record set creation fails WHEN Create SHOULD rollback all created resources",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createEndpointId: testVpcepEndpointId,
				createIp:         testVpcepEndpointIp,
			},
			dnsMock: &mockM3ToM1DnsService{
				createPrivateZoneId: testDnsID,
				createRecordErr:     errors.New("dns record set creation failed"),
			},
			expectedErrContains: "create M3 intranet domain record set failed",
		},
		{
			name: "GIVEN vpcep creation fails and rollback also fails WHEN Create SHOULD return warning about rollback failure",
			plan: newTestPlan(),
			sniMock: &mockM3ToM1SniProxyService{
				accessResourceId: testSniProxyID,
				deleteErr:        errors.New("sni rollback failed"),
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				createErr: errors.New("vpcep creation failed"),
			},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrContains: "create vpc-endpoint failed",
			expectWarning:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepService:    tc.vpcepMock,
				m3DnsService:      tc.dnsMock,
				m3SniProxyService: tc.sniMock,
			}

			req := resource.CreateRequest{Plan: newM3ToM1Plan(t, tc.plan)}
			resp := &resource.CreateResponse{State: newM3ToM1State(t)}

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
		name                string
		created             []createdResource
		sniMock             *mockM3ToM1SniProxyService
		vpcepMock           *mockM3ToM1VpcepEndpointService
		dnsMock             *mockM3ToM1DnsService
		expectedErrCount    int
		expectedErrContains string
	}{
		{
			name:             "GIVEN empty created list WHEN rollback SHOULD return no errors",
			created:          []createdResource{},
			sniMock:          &mockM3ToM1SniProxyService{},
			vpcepMock:        &mockM3ToM1VpcepEndpointService{},
			dnsMock:          &mockM3ToM1DnsService{},
			expectedErrCount: 0,
		},
		{
			name: "GIVEN all delete success WHEN rollback SHOULD return no errors",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			sniMock:          &mockM3ToM1SniProxyService{},
			vpcepMock:        &mockM3ToM1VpcepEndpointService{},
			dnsMock:          &mockM3ToM1DnsService{},
			expectedErrCount: 0,
		},
		{
			name: "GIVEN dns delete fails WHEN rollback SHOULD return dns error",
			created: []createdResource{
				{Type: dnsType, ID: testDnsID},
			},
			sniMock:   &mockM3ToM1SniProxyService{},
			vpcepMock: &mockM3ToM1VpcepEndpointService{},
			dnsMock: &mockM3ToM1DnsService{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsType,
		},
		{
			name: "GIVEN vpcep delete fails WHEN rollback SHOULD return vpcep error",
			created: []createdResource{
				{Type: vpcepType, ID: testVpcepID},
			},
			sniMock: &mockM3ToM1SniProxyService{},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				deleteErr: errors.New("vpcep delete failed"),
			},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrCount:    1,
			expectedErrContains: "delete " + vpcepType,
		},
		{
			name: "GIVEN sni delete fails WHEN rollback SHOULD return sni error",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
			},
			sniMock: &mockM3ToM1SniProxyService{
				deleteErr: errors.New("sni delete failed"),
			},
			vpcepMock:           &mockM3ToM1VpcepEndpointService{},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrCount:    1,
			expectedErrContains: "delete " + sniProxyType,
		},
		{
			name: "GIVEN vpcep delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			sniMock: &mockM3ToM1SniProxyService{},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				deleteErr: errors.New("vpcep delete failed"),
			},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrCount:    1,
			expectedErrContains: "delete " + vpcepType,
		},
		{
			name: "GIVEN dns delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			sniMock:   &mockM3ToM1SniProxyService{},
			vpcepMock: &mockM3ToM1VpcepEndpointService{},
			dnsMock: &mockM3ToM1DnsService{
				deletePrivateErr: errors.New("dns delete failed"),
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsType,
		},
		{
			name: "GIVEN sni delete fails others succeed WHEN rollback SHOULD return one error and continue deleting",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			sniMock: &mockM3ToM1SniProxyService{
				deleteErr: errors.New("sni delete failed"),
			},
			vpcepMock:           &mockM3ToM1VpcepEndpointService{},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrCount:    1,
			expectedErrContains: "delete " + sniProxyType,
		},
		{
			name: "GIVEN all three delete fail WHEN rollback SHOULD return all three errors",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			sniMock: &mockM3ToM1SniProxyService{
				deleteErr: errors.New("sni failed"),
			},
			vpcepMock: &mockM3ToM1VpcepEndpointService{
				deleteErr: errors.New("vpcep failed"),
			},
			dnsMock: &mockM3ToM1DnsService{
				deletePrivateErr: errors.New("dns failed"),
			},
			expectedErrCount: 3,
		},
		{
			name: "GIVEN unknown resource type WHEN rollback SHOULD return error",
			created: []createdResource{
				{Type: "unknown_type", ID: "unknown-id"},
			},
			sniMock:             &mockM3ToM1SniProxyService{},
			vpcepMock:           &mockM3ToM1VpcepEndpointService{},
			dnsMock:             &mockM3ToM1DnsService{},
			expectedErrCount:    1,
			expectedErrContains: "unknown resource type: unknown_type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				m3VpcepService:    tc.vpcepMock,
				m3DnsService:      tc.dnsMock,
				m3SniProxyService: tc.sniMock,
			}

			errs := r.rollback(ctx, tc.created)

			assert.Equal(t, tc.expectedErrCount, len(errs))
			if tc.expectedErrContains != "" && len(errs) > 0 {
				assert.Contains(t, errs[0].Error(), tc.expectedErrContains)
			}
			if tc.expectedErrCount > 1 {
				assert.Contains(t, errs[0].Error(), dnsType)
				assert.Contains(t, errs[1].Error(), vpcepType)
				assert.Contains(t, errs[2].Error(), sniProxyType)
			}
		})
	}
}

type mockM3ToM1VpcepEndpointService struct {
	createEndpointId string
	createIp         string
	createErr        error
	deleteErr        error
	createInputs     []service.VpcEndpointInput
	deleteIds        []string
}

func (m *mockM3ToM1VpcepEndpointService) Create(ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
	m.createInputs = append(m.createInputs, input)
	return m.createEndpointId, m.createIp, m.createErr
}

func (m *mockM3ToM1VpcepEndpointService) Delete(ctx context.Context, endpointId string) error {
	m.deleteIds = append(m.deleteIds, endpointId)
	return m.deleteErr
}

func (m *mockM3ToM1VpcepEndpointService) Get(ctx context.Context, endpointId string) (*service.VpcepEndpointOutput, error) {
	return nil, nil
}

type mockM3ToM1DnsService struct {
	createPrivateZoneId string
	createRecordSetId   string
	createZoneErr       error
	createRecordErr     error
	deletePrivateErr    error
	zoneInputs          []service.DnsZoneInput
	recordInputs        []service.DnsRecordSetInput
	deleteZoneIds       []string
}

func (m *mockM3ToM1DnsService) CreatePrivateZone(ctx context.Context, input service.DnsZoneInput) (string, error) {
	m.zoneInputs = append(m.zoneInputs, input)
	return m.createPrivateZoneId, m.createZoneErr
}

func (m *mockM3ToM1DnsService) CreateRecordSet(ctx context.Context, input service.DnsRecordSetInput) (string, error) {
	m.recordInputs = append(m.recordInputs, input)
	return m.createRecordSetId, m.createRecordErr
}

func (m *mockM3ToM1DnsService) DeletePrivateZone(ctx context.Context, zoneId string) error {
	m.deleteZoneIds = append(m.deleteZoneIds, zoneId)
	return m.deletePrivateErr
}

func (m *mockM3ToM1DnsService) GetPrivateZone(ctx context.Context, zoneId string) (*service.DnsZoneOutput, error) {
	return nil, nil
}

type mockM3ToM1SniProxyService struct {
	accessResourceId string
	accessErr        error
	deleteErr        error
	accessInputs     []service.AccessSniProxyInput
	deleteIds        []string
}

func (m *mockM3ToM1SniProxyService) AccessSniProxy(ctx context.Context, input service.AccessSniProxyInput) (string, error) {
	m.accessInputs = append(m.accessInputs, input)
	return m.accessResourceId, m.accessErr
}

func (m *mockM3ToM1SniProxyService) DeleteSniProxy(ctx context.Context, resourceId string) error {
	m.deleteIds = append(m.deleteIds, resourceId)
	return m.deleteErr
}

func (m *mockM3ToM1SniProxyService) GetSniProxy(ctx context.Context, resourceId string) (*service.AccessSniProxyOutput, *sniproxyclient.GetAccessServiceResponse, error) {
	return nil, nil, nil
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
