/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */
package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"huawei.com/kkem/kkem-net-provider/internal/service"
)

func setupCreateRequest(t *testing.T, ctx context.Context, plan netConnectM3ToM1Model) (resource.CreateRequest, *resource.CreateResponse) {
	t.Helper()

	r := NewNetConnectM3ToM1Resource()
	schemaReq := resource.SchemaRequest{}
	schemaResp := &resource.SchemaResponse{}

	r.Schema(ctx, schemaReq, schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	schemaType := schemaResp.Schema.Type()
	objType, ok := schemaType.(types.ObjectType)
	if !ok {
		t.Fatalf("schema type is not an object type")
	}
	attrTypes := objType.AttrTypes

	dnsDomainValue := plan.M3DnsDomainName
	if plan.M3DnsDomainName.IsNull() || plan.M3DnsDomainName.ValueString() == "" {
		dnsDomainValue = types.StringNull()
	}

	planObj, diag := types.ObjectValue(attrTypes, map[string]attr.Value{
		"m3_vpcep_id":           types.StringUnknown(),
		"m3_vpc_id":             plan.M3VpcID,
		"m3_vpcep_ip":           types.StringUnknown(),
		"m3_vpcep_subnet_id":    plan.M3VpcEndpointSubnetId,
		"sni_vpcep_server_id":   plan.SniVpcepServerId,
		"m3_dns_domain_name":    dnsDomainValue,
		"m3_dns_privatezone_id": types.StringUnknown(),
		"sni_proxy_resource_id": types.StringUnknown(),
		"region_code":           plan.RegionCode,
		"service_name":          plan.ServiceName,
		"m3_iam_domain_account": plan.DomainAccount,
	})
	if diag.HasError() {
		t.Fatalf("failed to create plan object: %v", diag)
	}

	rawValue, err := planObj.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("failed to convert to terraform value: %v", err)
	}

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Raw:    rawValue,
			Schema: schemaResp.Schema,
		},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{
			Raw:    rawValue.Copy(),
			Schema: schemaResp.Schema,
		},
	}

	return req, resp
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

			expectedAttributes := []string{
				"m3_vpcep_id",
				"m3_vpc_id",
				"m3_vpcep_ip",
				"m3_vpcep_subnet_id",
				"sni_vpcep_server_id",
				"m3_dns_domain_name",
				"m3_dns_privatezone_id",
				"sni_proxy_resource_id",
				"region_code",
				"service_name",
				"m3_iam_domain_account",
			}

			for _, attrName := range expectedAttributes {
				attr, ok := resp.Schema.Attributes[attrName]
				assert.True(t, ok, "schema should contain attribute: %s", attrName)

				switch attrName {
				case "m3_vpc_id", "m3_vpcep_subnet_id", "sni_vpcep_server_id",
					"region_code", "service_name", "m3_iam_domain_account", "m3_dns_domain_name":
					assert.True(t, attr.IsRequired(), "%s should be required", attrName)
				case "m3_vpcep_id", "m3_vpcep_ip", "m3_dns_privatezone_id", "sni_proxy_resource_id":
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
					assert.NotNil(t, r.vpcepEndpoint)
					assert.NotNil(t, r.dnsService)
					assert.NotNil(t, r.sniProxyService)
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

func TestNetConnectM3ToM1Resource_Create_Success(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		plan         netConnectM3ToM1Model
		setupPatches func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches
		verifyState  func(t *testing.T, state netConnectM3ToM1Model)
	}{
		{
			name: "GIVEN valid plan with domain WHEN Create SHOULD create all resources and set state",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						assert.Equal(t, testRegionCode, input.RegionCode)
						assert.Equal(t, testServiceName, input.ServiceName)
						assert.Equal(t, []string{testDomainAccount}, input.IamDomainAccount)
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						assert.Equal(t, testVpcepServiceId, input.EndpointServiceId)
						assert.Equal(t, testM3VpcId, input.VpcId)
						assert.Equal(t, testM3SubnetId, input.SubnetId)
						return "vpcep-endpoint-id", "10.0.0.1", nil
					})

				patches.ApplyMethod(&service.DnsService{}, "CreatePrivateZone",
					func(_ *service.DnsService, ctx context.Context, input service.DnsZoneInput) (string, error) {
						assert.Equal(t, testDomainName, input.DomainName)
						assert.Equal(t, testM3VpcId, input.RouterId)
						return "dns-zone-id", nil
					})

				patches.ApplyMethod(&service.DnsService{}, "CreateRecordSet",
					func(_ *service.DnsService, ctx context.Context, input service.DnsRecordSetInput) (string, error) {
						assert.Equal(t, "dns-zone-id", input.ZoneId)
						assert.Equal(t, testDomainName, input.Name)
						assert.Equal(t, []string{"10.0.0.1"}, input.Records)
						return "record-set-id", nil
					})

				return patches
			},
			verifyState: func(t *testing.T, state netConnectM3ToM1Model) {
				assert.Equal(t, "sni-proxy-res-id", state.SniProxyResourceId.ValueString())
				assert.Equal(t, "vpcep-endpoint-id", state.M3VpcEndpointId.ValueString())
				assert.Equal(t, "10.0.0.1", state.M3VpcEndpointIp.ValueString())
				assert.Equal(t, "dns-zone-id", state.M3DnsPrivateZoneId.ValueString())
			},
		},
		{
			name: "GIVEN plan without domain name WHEN Create SHOULD skip DNS creation",
			plan: netConnectM3ToM1Model{
				M3VpcID:               types.StringValue(testM3VpcId),
				M3VpcEndpointSubnetId: types.StringValue(testM3SubnetId),
				SniVpcepServerId:      types.StringValue(testVpcepServiceId),
				M3DnsDomainName:       types.StringNull(),
				RegionCode:            types.StringValue(testRegionCode),
				ServiceName:           types.StringValue(testServiceName),
				DomainAccount:         types.StringValue(testDomainAccount),
			},
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "vpcep-endpoint-id", "10.0.0.1", nil
					})

				return patches
			},
			verifyState: func(t *testing.T, state netConnectM3ToM1Model) {
				assert.Equal(t, "sni-proxy-res-id", state.SniProxyResourceId.ValueString())
				assert.Equal(t, "vpcep-endpoint-id", state.M3VpcEndpointId.ValueString())
				assert.Equal(t, "10.0.0.1", state.M3VpcEndpointIp.ValueString())
				assert.True(t, state.M3DnsPrivateZoneId.IsUnknown() || state.M3DnsPrivateZoneId.IsNull())
			},
		},
		{
			name: "GIVEN empty domain name WHEN Create SHOULD skip DNS creation",
			plan: netConnectM3ToM1Model{
				M3VpcID:               types.StringValue(testM3VpcId),
				M3VpcEndpointSubnetId: types.StringValue(testM3SubnetId),
				SniVpcepServerId:      types.StringValue(testVpcepServiceId),
				M3DnsDomainName:       types.StringValue(""),
				RegionCode:            types.StringValue(testRegionCode),
				ServiceName:           types.StringValue(testServiceName),
				DomainAccount:         types.StringValue(testDomainAccount),
			},
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "vpcep-endpoint-id", "10.0.0.1", nil
					})

				return patches
			},
			verifyState: func(t *testing.T, state netConnectM3ToM1Model) {
				assert.Equal(t, "sni-proxy-res-id", state.SniProxyResourceId.ValueString())
				assert.Equal(t, "vpcep-endpoint-id", state.M3VpcEndpointId.ValueString())
				assert.Equal(t, "10.0.0.1", state.M3VpcEndpointIp.ValueString())
				// When empty domain is provided, M3DnsPrivateZoneId should remain unknown (not set)
				assert.True(t, state.M3DnsPrivateZoneId.IsUnknown() || state.M3DnsPrivateZoneId.IsNull())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				vpcepEndpoint:   service.NewVpcepEndpointService(nil),
				dnsService:      service.NewDnsService(nil),
				sniProxyService: service.NewSniProxyService(nil),
			}

			patches := tc.setupPatches(t, r)
			defer patches.Reset()

			req, resp := setupCreateRequest(t, ctx, tc.plan)

			r.Create(ctx, req, resp)

			assert.False(t, resp.Diagnostics.HasError())

			var state netConnectM3ToM1Model
			resp.State.Get(ctx, &state)
			if tc.verifyState != nil {
				tc.verifyState(t, state)
			}
		})
	}
}

func TestNetConnectM3ToM1Resource_Create_Failure_And_Rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		plan                netConnectM3ToM1Model
		setupPatches        func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches
		expectedErrContains string
	}{
		{
			name: "GIVEN sni proxy creation fails WHEN Create SHOULD return error without creating other resources",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "", errors.New("sni proxy access failed")
					})

				return patches
			},
			expectedErrContains: "create sni-proxy failed",
		},
		{
			name: "GIVEN vpcep endpoint creation fails WHEN Create SHOULD rollback sni proxy",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						assert.Equal(t, "sni-proxy-res-id", resourceId)
						return nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "", "", errors.New("vpcep endpoint creation failed")
					})

				return patches
			},
			expectedErrContains: "create vpc-endpoint failed",
		},
		{
			name: "GIVEN dns private zone creation fails WHEN Create SHOULD rollback vpcep and sni proxy",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						assert.Equal(t, "sni-proxy-res-id", resourceId)
						return nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "vpcep-endpoint-id", "10.0.0.1", nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						assert.Equal(t, "vpcep-endpoint-id", endpointId)
						return nil
					})

				patches.ApplyMethod(&service.DnsService{}, "CreatePrivateZone",
					func(_ *service.DnsService, ctx context.Context, input service.DnsZoneInput) (string, error) {
						return "", errors.New("dns private zone creation failed")
					})

				return patches
			},
			expectedErrContains: "create M3 intranet domain failed",
		},
		{
			name: "GIVEN dns record set creation fails WHEN Create SHOULD rollback all created resources",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return "sni-proxy-res-id", nil
					})

				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						assert.Equal(t, "sni-proxy-res-id", resourceId)
						return nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "vpcep-endpoint-id", "10.0.0.1", nil
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						assert.Equal(t, "vpcep-endpoint-id", endpointId)
						return nil
					})

				patches.ApplyMethod(&service.DnsService{}, "CreatePrivateZone",
					func(_ *service.DnsService, ctx context.Context, input service.DnsZoneInput) (string, error) {
						return "dns-zone-id", nil
					})

				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						assert.Equal(t, "dns-zone-id", zoneId)
						return nil
					})

				patches.ApplyMethod(&service.DnsService{}, "CreateRecordSet",
					func(_ *service.DnsService, ctx context.Context, input service.DnsRecordSetInput) (string, error) {
						return "", errors.New("dns record set creation failed")
					})

				return patches
			},
			expectedErrContains: "create M3 intranet domain record set failed",
		},
		{
			name: "GIVEN vpcep creation fails and rollback also fails WHEN Create SHOULD return warning about rollback failure",
			plan: newTestPlan(),
			setupPatches: func(t *testing.T, r *netConnectM3ToM1Resource) *gomonkey.Patches {
				patches := gomonkey.NewPatches()

				patches.ApplyMethod(&service.SniProxyService{}, "AccessSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, input service.AccessSniProxyInput) (string, error) {
						return testSniProxyID, nil
					})

				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						assert.Equal(t, testSniProxyID, resourceId)
						return errors.New("sni rollback failed")
					})

				patches.ApplyMethod(&service.VpcepEndpointService{}, "Create",
					func(_ *service.VpcepEndpointService, ctx context.Context, input service.VpcEndpointInput) (string, string, error) {
						return "", "", errors.New("vpcep creation failed")
					})

				return patches
			},
			expectedErrContains: "create vpc-endpoint failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				vpcepEndpoint:   service.NewVpcepEndpointService(nil),
				dnsService:      service.NewDnsService(nil),
				sniProxyService: service.NewSniProxyService(nil),
			}

			patches := tc.setupPatches(t, r)
			defer patches.Reset()

			req, resp := setupCreateRequest(t, ctx, tc.plan)

			r.Create(ctx, req, resp)

			assert.True(t, resp.Diagnostics.HasError())
			errFound := false
			for _, diagErr := range resp.Diagnostics.Errors() {
				if strings.Contains(diagErr.Summary(), tc.expectedErrContains) || strings.Contains(diagErr.Detail(), tc.expectedErrContains) {
					errFound = true
					break
				}
			}
			assert.True(t, errFound, "expected error containing '%s' but got: %v", tc.expectedErrContains, resp.Diagnostics.Errors())

			if tc.name == "GIVEN vpcep creation fails and rollback also fails WHEN Create SHOULD return warning about rollback failure" {
				assert.Len(t, resp.Diagnostics.Warnings(), 1)
				assert.Contains(t, resp.Diagnostics.Warnings()[0].Detail(), "sni rollback failed")
			}
		})
	}
}

func Test_netConnectM3ToM1Resource_rollback(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		created             []createdResource
		setupPatches        func(t *testing.T) (*gomonkey.Patches, *[]string)
		expectedErrCount    int
		expectedErrContains string
	}{
		{
			name:    "GIVEN empty created list WHEN rollback SHOULD return no errors",
			created: []createdResource{},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				return gomonkey.NewPatches(), nil
			},
			expectedErrCount: 0,
		},
		{
			name: "GIVEN all delete success WHEN rollback SHOULD return no errors",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error { return nil })
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error { return nil })
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error { return nil })
				return patches, nil
			},
			expectedErrCount: 0,
		},
		{
			name: "GIVEN dns delete fails WHEN rollback SHOULD return dns error",
			created: []createdResource{
				{Type: dnsType, ID: testDnsID},
			},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						return errors.New("dns delete failed")
					})
				return patches, nil
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + dnsType,
		},
		{
			name: "GIVEN vpcep delete fails WHEN rollback SHOULD return vpcep error",
			created: []createdResource{
				{Type: vpcepType, ID: testVpcepID},
			},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						return errors.New("vpcep delete failed")
					})
				return patches, nil
			},
			expectedErrCount:    1,
			expectedErrContains: "delete " + vpcepType,
		},
		{
			name: "GIVEN sni delete fails WHEN rollback SHOULD return sni error",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
			},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						return errors.New("sni delete failed")
					})
				return patches, nil
			},
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
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				callOrder := []string{}
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						callOrder = append(callOrder, "dns")
						return nil
					})
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						callOrder = append(callOrder, "vpcep")
						return errors.New("vpcep delete failed")
					})
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						callOrder = append(callOrder, "sni")
						return nil
					})
				return patches, &callOrder
			},
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
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				callOrder := []string{}
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						callOrder = append(callOrder, "dns")
						return errors.New("dns delete failed")
					})
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						callOrder = append(callOrder, "vpcep")
						return nil
					})
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						callOrder = append(callOrder, "sni")
						return nil
					})
				return patches, &callOrder
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
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				callOrder := []string{}
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						callOrder = append(callOrder, "dns")
						return nil
					})
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						callOrder = append(callOrder, "vpcep")
						return nil
					})
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						callOrder = append(callOrder, "sni")
						return errors.New("sni delete failed")
					})
				return patches, &callOrder
			},
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
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						return errors.New("dns failed")
					})
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						return errors.New("vpcep failed")
					})
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						return errors.New("sni failed")
					})
				return patches, nil
			},
			expectedErrCount: 3,
		},
		{
			name: "GIVEN reverse delete order WHEN rollback SHOULD delete dns first, then vpcep, then sni",
			created: []createdResource{
				{Type: sniProxyType, ID: testSniProxyID},
				{Type: vpcepType, ID: testVpcepID},
				{Type: dnsType, ID: testDnsID},
			},
			setupPatches: func(t *testing.T) (*gomonkey.Patches, *[]string) {
				callOrder := []string{}
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&service.DnsService{}, "DeletePrivateZone",
					func(_ *service.DnsService, ctx context.Context, zoneId string) error {
						callOrder = append(callOrder, "dns")
						return nil
					})
				patches.ApplyMethod(&service.VpcepEndpointService{}, "Delete",
					func(_ *service.VpcepEndpointService, ctx context.Context, endpointId string) error {
						callOrder = append(callOrder, "vpcep")
						return nil
					})
				patches.ApplyMethod(&service.SniProxyService{}, "DeleteSniProxy",
					func(_ *service.SniProxyService, ctx context.Context, resourceId string) error {
						callOrder = append(callOrder, "sni")
						return nil
					})
				return patches, &callOrder
			},
			expectedErrCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &netConnectM3ToM1Resource{
				vpcepEndpoint:   service.NewVpcepEndpointService(nil),
				dnsService:      service.NewDnsService(nil),
				sniProxyService: service.NewSniProxyService(nil),
			}

			patches, callOrder := tc.setupPatches(t)
			defer patches.Reset()

			errs := r.rollback(ctx, tc.created)

			assert.Equal(t, tc.expectedErrCount, len(errs))
			if tc.expectedErrContains != "" && len(errs) > 0 {
				assert.Contains(t, errs[0].Error(), tc.expectedErrContains)
			}
			if callOrder != nil {
				assert.Equal(t, []string{"dns", "vpcep", "sni"}, *callOrder)
			}
			if tc.expectedErrCount > 1 {
				assert.Contains(t, errs[0].Error(), dnsType)
				assert.Contains(t, errs[1].Error(), vpcepType)
				assert.Contains(t, errs[2].Error(), sniProxyType)
			}
		})
	}
}
