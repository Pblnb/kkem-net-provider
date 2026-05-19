/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */
package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"

	"huawei.com/kkem/kkem-net-provider/internal/service"
)

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
