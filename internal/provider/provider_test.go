/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTypeName         = "kkem"
	testVersion          = "0.1.0"
	testVpcepEndpoint    = "https://vpcep.cn-north-7.myhuaweicloud.com"
	testLbmDnsEndpoint   = "https://lbm-app-api.myhuaweicloud.com"
	testDnsEndpoint      = "https://dns.cn-north-7.myhuaweicloud.com"
	testSniProxyEndpoint = "https://linksniproxy-test.myhuaweicloud.com"
	testXOpenToken       = "token-1"
	testM1PlusAk         = "m1-ak"
	testM1PlusSk         = "m1-sk"
	testM1PlusProjectId  = "m1-project-id"
	testM3Ak             = "m3-ak"
	testM3Sk             = "m3-sk"
	testM3ProjectId      = "m3-project-id"
)

func TestNewKKEMProvider(t *testing.T) {
	testCases := []struct {
		name    string
		version string
	}{
		{
			name:    "GIVEN provider version WHEN NewKKEMProvider SHOULD return provider factory",
			version: testVersion,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := NewKKEMProvider(tc.version)

			require.NotNil(t, actual)
			providerInstance, ok := actual().(*KkemProvider)
			require.True(t, ok)
			assert.Equal(t, tc.version, providerInstance.version)
		})
	}
}

func TestKkemProvider_Metadata(t *testing.T) {
	testCases := []struct {
		name            string
		provider        *KkemProvider
		expectedName    string
		expectedVersion string
	}{
		{
			name:            "GIVEN provider version WHEN Metadata SHOULD set provider name and version",
			provider:        &KkemProvider{version: testVersion},
			expectedName:    testTypeName,
			expectedVersion: testVersion,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &provider.MetadataResponse{}

			tc.provider.Metadata(context.Background(), provider.MetadataRequest{}, resp)

			assert.Equal(t, tc.expectedName, resp.TypeName)
			assert.Equal(t, tc.expectedVersion, resp.Version)
		})
	}
}

func TestKkemProvider_Schema(t *testing.T) {
	const (
		expectedProviderSchemaAttributeCount = 5
		expectedProviderSchemaBlockCount     = 2
	)

	testCases := []struct {
		name     string
		provider *KkemProvider
	}{
		{
			name:     "GIVEN provider WHEN Schema SHOULD return all required attributes and blocks",
			provider: &KkemProvider{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &provider.SchemaResponse{}

			tc.provider.Schema(context.Background(), provider.SchemaRequest{}, resp)

			assert.False(t, resp.Diagnostics.HasError())
			assert.Len(t, resp.Schema.Attributes, expectedProviderSchemaAttributeCount)
			assert.Len(t, resp.Schema.Blocks, expectedProviderSchemaBlockCount)
			assert.NotContains(t, resp.Schema.Attributes, "nonexistent_field")
			assertStringAttribute(t, resp.Schema.Attributes, "vpcep_endpoint", true, false)
			assertStringAttribute(t, resp.Schema.Attributes, "lbm_dns_endpoint", true, false)
			assertStringAttribute(t, resp.Schema.Attributes, "dns_endpoint", true, false)
			assertStringAttribute(t, resp.Schema.Attributes, "sni_proxy_endpoint", true, false)
			assertStringAttribute(t, resp.Schema.Attributes, "x_open_token", true, true)
			assertCredentialBlock(t, resp.Schema.Blocks, "m1_plus")
			assertCredentialBlock(t, resp.Schema.Blocks, "m3")
		})
	}
}

func TestKkemProvider_buildVpcepClient(t *testing.T) {
	testCases := []struct {
		name        string
		ak          string
		sk          string
		projectId   string
		endpoint    string
		setupPatch  func(t *testing.T) *gomonkey.Patches
		expectedErr string
	}{
		{
			name:      "GIVEN valid config WHEN buildVpcepClient SHOULD return client",
			ak:        testM1PlusAk,
			sk:        testM1PlusSk,
			projectId: testM1PlusProjectId,
			endpoint:  testVpcepEndpoint,
		},
		{
			name:        "GIVEN empty ak WHEN buildVpcepClient SHOULD return error",
			sk:          testM1PlusSk,
			projectId:   testM1PlusProjectId,
			endpoint:    testVpcepEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty sk WHEN buildVpcepClient SHOULD return error",
			ak:          testM1PlusAk,
			projectId:   testM1PlusProjectId,
			endpoint:    testVpcepEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty project id WHEN buildVpcepClient SHOULD return error",
			ak:          testM1PlusAk,
			sk:          testM1PlusSk,
			endpoint:    testVpcepEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty endpoint WHEN buildVpcepClient SHOULD return error",
			ak:          testM1PlusAk,
			sk:          testM1PlusSk,
			projectId:   testM1PlusProjectId,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:      "GIVEN credential build fails WHEN buildVpcepClient SHOULD return credential error",
			ak:        testM1PlusAk,
			sk:        testM1PlusSk,
			projectId: testM1PlusProjectId,
			endpoint:  testVpcepEndpoint,
			setupPatch: func(t *testing.T) *gomonkey.Patches {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&auth.BasicCredentialsBuilder{}, "SafeBuild",
					func(_ *auth.BasicCredentialsBuilder) (*auth.BasicCredentials, error) {
						return nil, errors.New("credential build failed")
					})
				return patches
			},
			expectedErr: "failed to init credential with ak/sk: credential build failed",
		},
		{
			name:      "GIVEN http client build fails WHEN buildVpcepClient SHOULD return vpcep client error",
			ak:        testM1PlusAk,
			sk:        testM1PlusSk,
			projectId: testM1PlusProjectId,
			endpoint:  testVpcepEndpoint,
			setupPatch: func(t *testing.T) *gomonkey.Patches {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&core.HcHttpClientBuilder{}, "SafeBuild",
					func(_ *core.HcHttpClientBuilder) (*core.HcHttpClient, error) {
						return nil, errors.New("http client build failed")
					})
				return patches
			},
			expectedErr: fmt.Sprintf("failed to init vpcep client with endpoint %s: http client build failed",
				testVpcepEndpoint),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupPatch != nil {
				patches := tc.setupPatch(t)
				defer patches.Reset()
			}

			actual, err := (&KkemProvider{}).buildVpcepClient(context.Background(), "M1+", tc.ak, tc.sk, tc.projectId,
				tc.endpoint)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.NotNil(t, actual)
			} else {
				assert.Nil(t, actual)
				assert.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func TestKkemProvider_buildDnsClient(t *testing.T) {
	testCases := []struct {
		name        string
		ak          string
		sk          string
		projectId   string
		endpoint    string
		setupPatch  func(t *testing.T) *gomonkey.Patches
		expectedErr string
	}{
		{
			name:      "GIVEN valid config WHEN buildDnsClient SHOULD return client",
			ak:        testM3Ak,
			sk:        testM3Sk,
			projectId: testM3ProjectId,
			endpoint:  testDnsEndpoint,
		},
		{
			name:        "GIVEN empty ak WHEN buildDnsClient SHOULD return error",
			sk:          testM3Sk,
			projectId:   testM3ProjectId,
			endpoint:    testDnsEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty sk WHEN buildDnsClient SHOULD return error",
			ak:          testM3Ak,
			projectId:   testM3ProjectId,
			endpoint:    testDnsEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty project id WHEN buildDnsClient SHOULD return error",
			ak:          testM3Ak,
			sk:          testM3Sk,
			endpoint:    testDnsEndpoint,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:        "GIVEN empty endpoint WHEN buildDnsClient SHOULD return error",
			ak:          testM3Ak,
			sk:          testM3Sk,
			projectId:   testM3ProjectId,
			expectedErr: "ak, sk, projectId, endpoint must not be empty",
		},
		{
			name:      "GIVEN credential build fails WHEN buildDnsClient SHOULD return credential error",
			ak:        testM3Ak,
			sk:        testM3Sk,
			projectId: testM3ProjectId,
			endpoint:  testDnsEndpoint,
			setupPatch: func(t *testing.T) *gomonkey.Patches {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&auth.BasicCredentialsBuilder{}, "SafeBuild",
					func(_ *auth.BasicCredentialsBuilder) (*auth.BasicCredentials, error) {
						return nil, errors.New("credential build failed")
					})
				return patches
			},
			expectedErr: "failed to init credential with ak/sk: credential build failed",
		},
		{
			name:      "GIVEN http client build fails WHEN buildDnsClient SHOULD return dns client error",
			ak:        testM3Ak,
			sk:        testM3Sk,
			projectId: testM3ProjectId,
			endpoint:  testDnsEndpoint,
			setupPatch: func(t *testing.T) *gomonkey.Patches {
				patches := gomonkey.NewPatches()
				patches.ApplyMethod(&core.HcHttpClientBuilder{}, "SafeBuild",
					func(_ *core.HcHttpClientBuilder) (*core.HcHttpClient, error) {
						return nil, errors.New("http client build failed")
					})
				return patches
			},
			expectedErr: fmt.Sprintf("failed to init dns client with endpoint %s: http client build failed",
				testDnsEndpoint),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupPatch != nil {
				patches := tc.setupPatch(t)
				defer patches.Reset()
			}

			actual, err := (&KkemProvider{}).buildDnsClient(context.Background(), "M3", tc.ak, tc.sk, tc.projectId,
				tc.endpoint)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.NotNil(t, actual)
			} else {
				assert.Nil(t, actual)
				assert.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func TestKkemProvider_Configure(t *testing.T) {
	const expectedEmptyClientEndpoint = "https://"

	testCases := []struct {
		name                     string
		config                   func(t *testing.T, ctx context.Context) tfsdk.Config
		expectedErrSummary       string
		expectedLbmDnsEndpoint   string
		expectedSniProxyEndpoint string
		expectedClientToken      string
	}{
		{
			name: "GIVEN valid config WHEN Configure SHOULD initialize all clients",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				return buildProviderConfig(t, ctx, validProviderModel())
			},
			expectedLbmDnsEndpoint:   testLbmDnsEndpoint,
			expectedSniProxyEndpoint: testSniProxyEndpoint,
			expectedClientToken:      testXOpenToken,
		},
		{
			name: "GIVEN empty lbm dns endpoint WHEN Configure SHOULD create lbm dns client with empty endpoint",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.LbmDnsEndpoint = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedLbmDnsEndpoint:   expectedEmptyClientEndpoint,
			expectedSniProxyEndpoint: testSniProxyEndpoint,
			expectedClientToken:      testXOpenToken,
		},
		{
			name: "GIVEN empty sni proxy endpoint WHEN Configure SHOULD create sni proxy client with empty endpoint",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.SniProxyEndpoint = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedLbmDnsEndpoint:   testLbmDnsEndpoint,
			expectedSniProxyEndpoint: expectedEmptyClientEndpoint,
			expectedClientToken:      testXOpenToken,
		},
		{
			name: "GIVEN empty x open token WHEN Configure SHOULD create HTTP clients with empty token",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.XOpenToken = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedLbmDnsEndpoint:   testLbmDnsEndpoint,
			expectedSniProxyEndpoint: testSniProxyEndpoint,
			expectedClientToken:      "",
		},
		{
			name: "GIVEN malformed config WHEN Configure SHOULD return config diagnostics",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				return buildMalformedProviderConfig(t, ctx)
			},
			expectedErrSummary: "Value Conversion Error",
		},
		{
			name: "GIVEN empty M1 plus ak WHEN Configure SHOULD return M1 plus vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M1Plus.Ak = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M1+ VPCEP client failed",
		},
		{
			name: "GIVEN empty M1 plus sk WHEN Configure SHOULD return M1 plus vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M1Plus.Sk = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M1+ VPCEP client failed",
		},
		{
			name: "GIVEN empty M1 plus project id WHEN Configure SHOULD return M1 plus vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M1Plus.ProjectId = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M1+ VPCEP client failed",
		},
		{
			name: "GIVEN empty vpcep endpoint WHEN Configure SHOULD return M1 plus vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.VpcepEndpoint = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M1+ VPCEP client failed",
		},
		{
			name: "GIVEN empty M3 ak WHEN Configure SHOULD return M3 vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M3.Ak = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M3 VPCEP client failed",
		},
		{
			name: "GIVEN empty M3 sk WHEN Configure SHOULD return M3 vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M3.Sk = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M3 VPCEP client failed",
		},
		{
			name: "GIVEN empty M3 project id WHEN Configure SHOULD return M3 vpcep client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.M3.ProjectId = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M3 VPCEP client failed",
		},
		{
			name: "GIVEN empty DNS endpoint WHEN Configure SHOULD return M3 DNS client error",
			config: func(t *testing.T, ctx context.Context) tfsdk.Config {
				config := validProviderModel()
				config.DnsEndpoint = ""
				return buildProviderConfig(t, ctx, config)
			},
			expectedErrSummary: "create M3 DNS client failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req := provider.ConfigureRequest{Config: tc.config(t, ctx)}
			resp := &provider.ConfigureResponse{}

			(&KkemProvider{}).Configure(ctx, req, resp)

			if tc.expectedErrSummary == "" {
				require.False(t, resp.Diagnostics.HasError())
				resourceClients, ok := resp.ResourceData.(*clients)
				require.True(t, ok)
				dataSourceClients, ok := resp.DataSourceData.(*clients)
				require.True(t, ok)
				assert.Same(t, resourceClients, dataSourceClients)
				assert.NotNil(t, resourceClients.m1PlusVpcepClient)
				assert.NotNil(t, resourceClients.m3VpcepClient)
				assert.NotNil(t, resourceClients.m3DnsClient)
				assert.NotNil(t, resourceClients.lbmDnsClient)
				assert.NotNil(t, resourceClients.sniProxyClient)
				assert.Equal(t, tc.expectedLbmDnsEndpoint,
					commonClientStringField(t, resourceClients.lbmDnsClient.Client,
						"endpoint"))
				assert.Equal(t, tc.expectedClientToken,
					commonClientStringField(t, resourceClients.lbmDnsClient.Client,
						"token"))
				assert.Equal(t, tc.expectedSniProxyEndpoint,
					commonClientStringField(t, resourceClients.sniProxyClient.Client,
						"endpoint"))
				assert.Equal(t, tc.expectedClientToken,
					commonClientStringField(t, resourceClients.sniProxyClient.Client,
						"token"))
			} else {
				require.True(t, resp.Diagnostics.HasError())
				require.NotEmpty(t, resp.Diagnostics.Errors())
				assert.Equal(t, tc.expectedErrSummary, resp.Diagnostics.Errors()[0].Summary())
				assert.Nil(t, resp.ResourceData)
				assert.Nil(t, resp.DataSourceData)
			}
		})
	}
}

func TestKkemProvider_DataSources(t *testing.T) {
	testCases := []struct {
		name     string
		provider *KkemProvider
	}{
		{
			name:     "GIVEN provider WHEN DataSources SHOULD return nil",
			provider: &KkemProvider{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.provider.DataSources(context.Background())

			assert.Nil(t, actual)
		})
	}
}

func TestKkemProvider_Resources(t *testing.T) {
	const (
		testM1ToM3ResourceTypeName = "_net_connect_m1_to_m3"
		testM3ToM1ResourceTypeName = "_net_connect_m3_to_m1"
	)

	testCases := []struct {
		name                  string
		provider              *KkemProvider
		expectedResourceNames []string
	}{
		{
			name:     "GIVEN provider WHEN Resources SHOULD return network resources",
			provider: &KkemProvider{},
			expectedResourceNames: []string{testTypeName + testM1ToM3ResourceTypeName,
				testTypeName + testM3ToM1ResourceTypeName},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.provider.Resources(context.Background())

			require.Len(t, actual, len(tc.expectedResourceNames))
			remainingNames := make(map[string]struct{}, len(tc.expectedResourceNames))
			for _, expectedName := range tc.expectedResourceNames {
				remainingNames[expectedName] = struct{}{}
			}
			for _, resourceFactory := range actual {
				resourceInstance := resourceFactory()
				resp := &resource.MetadataResponse{}
				resourceInstance.Metadata(context.Background(),
					resource.MetadataRequest{ProviderTypeName: testTypeName}, resp)

				_, ok := remainingNames[resp.TypeName]
				require.True(t, ok, "unexpected or duplicate resource type: %s", resp.TypeName)
				delete(remainingNames, resp.TypeName)
				switch resp.TypeName {
				case testTypeName + testM1ToM3ResourceTypeName:
					_, ok = resourceInstance.(*netConnectM1ToM3Resource)
					assert.True(t, ok)
				case testTypeName + testM3ToM1ResourceTypeName:
					_, ok = resourceInstance.(*netConnectM3ToM1Resource)
					assert.True(t, ok)
				}
			}
			assert.Empty(t, remainingNames)
		})
	}
}

func assertStringAttribute(t *testing.T, attrs map[string]providerschema.Attribute, name string,
	required, sensitive bool) {
	t.Helper()

	attribute, ok := attrs[name]
	require.True(t, ok, "schema should contain attribute %s", name)
	stringAttr, ok := attribute.(providerschema.StringAttribute)
	require.True(t, ok, "%s should be string attribute", name)
	assert.Equal(t, required, stringAttr.IsRequired())
	assert.Equal(t, sensitive, stringAttr.IsSensitive())
	assert.NotEmpty(t, stringAttr.Description)
}

func assertCredentialBlock(t *testing.T, blocks map[string]providerschema.Block, name string) {
	t.Helper()

	block, ok := blocks[name]
	require.True(t, ok, "schema should contain block %s", name)
	nestedBlock, ok := block.(providerschema.SingleNestedBlock)
	require.True(t, ok, "%s should be single nested block", name)
	assert.NotEmpty(t, nestedBlock.Description)
	assertStringAttribute(t, nestedBlock.Attributes, "ak", true, true)
	assertStringAttribute(t, nestedBlock.Attributes, "sk", true, true)
	assertStringAttribute(t, nestedBlock.Attributes, "project_id", true, false)
}

func buildProviderConfig(t *testing.T, ctx context.Context, data kkemNetProviderModel) tfsdk.Config {
	t.Helper()

	providerSchema := buildProviderSchema(t, ctx)
	schemaType, ok := providerSchema.Type().(types.ObjectType)
	require.True(t, ok)
	attrTypes := schemaType.AttrTypes

	m1PlusValue := buildProviderCredentialValue(t, attrTypes, "m1_plus", data.M1Plus)
	m3Value := buildProviderCredentialValue(t, attrTypes, "m3", data.M3)
	configValue, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
		"vpcep_endpoint":     types.StringValue(data.VpcepEndpoint),
		"lbm_dns_endpoint":   types.StringValue(data.LbmDnsEndpoint),
		"dns_endpoint":       types.StringValue(data.DnsEndpoint),
		"sni_proxy_endpoint": types.StringValue(data.SniProxyEndpoint),
		"x_open_token":       types.StringValue(data.XOpenToken),
		"m1_plus":            m1PlusValue,
		"m3":                 m3Value,
	})
	require.False(t, diags.HasError(), fmt.Sprintf("failed to create provider config value: %v", diags))

	rawValue, err := configValue.ToTerraformValue(ctx)
	require.NoError(t, err)
	return tfsdk.Config{
		Raw:    rawValue,
		Schema: providerSchema,
	}
}

func buildMalformedProviderConfig(t *testing.T, ctx context.Context) tfsdk.Config {
	t.Helper()

	providerSchema := buildProviderSchema(t, ctx)
	schemaType, ok := providerSchema.Type().(types.ObjectType)
	require.True(t, ok)
	rawValue := tftypes.NewValue(schemaType.TerraformType(ctx), nil)
	return tfsdk.Config{
		Raw:    rawValue,
		Schema: providerSchema,
	}
}

func buildProviderSchema(t *testing.T, ctx context.Context) providerschema.Schema {
	t.Helper()

	resp := &provider.SchemaResponse{}
	(&KkemProvider{}).Schema(ctx, provider.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError())
	return resp.Schema
}

func buildProviderCredentialValue(t *testing.T, attrTypes map[string]attr.Type, blockName string,
	credentials cloudCredentials) attr.Value {
	t.Helper()

	blockType, ok := attrTypes[blockName].(types.ObjectType)
	require.True(t, ok, "%s should be object type", blockName)
	value, diags := types.ObjectValue(blockType.AttrTypes, map[string]attr.Value{
		"ak":         types.StringValue(credentials.Ak),
		"sk":         types.StringValue(credentials.Sk),
		"project_id": types.StringValue(credentials.ProjectId),
	})
	require.False(t, diags.HasError(), fmt.Sprintf("failed to create %s block value: %v", blockName, diags))
	return value
}

func validProviderModel() kkemNetProviderModel {
	return kkemNetProviderModel{
		M1Plus: cloudCredentials{
			Ak:        testM1PlusAk,
			Sk:        testM1PlusSk,
			ProjectId: testM1PlusProjectId,
		},
		M3: cloudCredentials{
			Ak:        testM3Ak,
			Sk:        testM3Sk,
			ProjectId: testM3ProjectId,
		},
		VpcepEndpoint:    testVpcepEndpoint,
		LbmDnsEndpoint:   testLbmDnsEndpoint,
		DnsEndpoint:      testDnsEndpoint,
		SniProxyEndpoint: testSniProxyEndpoint,
		XOpenToken:       testXOpenToken,
	}
}

func commonClientStringField(t *testing.T, client any, fieldName string) string {
	t.Helper()

	// common.Client 未导出 endpoint/token，这里通过反射只校验 Configure 传入的配置是否被保留下来。
	value := reflect.ValueOf(client)
	require.Equal(t, reflect.Pointer, value.Kind())
	field := value.Elem().FieldByName(fieldName)
	require.True(t, field.IsValid(), "common client should contain %s", fieldName)
	require.Equal(t, reflect.String, field.Kind())
	return field.String()
}
