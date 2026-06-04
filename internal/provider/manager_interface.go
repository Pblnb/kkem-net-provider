/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package provider

import (
	"context"

	"huawei.com/kkem/kkem-net-provider/internal/client/sniproxyclient"
	"huawei.com/kkem/kkem-net-provider/internal/manager"
)

type vpcepEndpointManager interface {
	Create(ctx context.Context, input manager.VpcEndpointInput) (string, string, error)
	Delete(ctx context.Context, endpointId string) error
	Get(ctx context.Context, endpointId string) (*manager.VpcepEndpointOutput, error)
}

// vpcepServiceManager 封装 VPCEP-Service 的生命周期、权限和配置更新操作。
type vpcepServiceManager interface {
	Create(ctx context.Context, input manager.VpcepServiceInput) (string, error)
	Delete(ctx context.Context, serviceId string) error
	Get(ctx context.Context, serviceId string) (*manager.VpcepServiceOutput, error)
	AddPermissions(ctx context.Context, serviceId string, permissions []manager.PermissionInput) error
	GetPermissions(ctx context.Context, serviceId string) (map[string]string, error)
	UpdateConfig(ctx context.Context, serviceId string, input manager.VpcepServiceInput) error
	ReconcilePermissions(ctx context.Context, serviceId string, desired []manager.PermissionInput) error
}

type lbmDnsManager interface {
	CreateIntranetDnsDomain(ctx context.Context, input manager.CreateLbmDnsInput) (*manager.CreateLbmDnsOutput, error)
	DeleteIntranetDnsDomain(ctx context.Context, recordId string) error
	UpdateRecordValue(ctx context.Context, recordId, endpointIp string) error
	GetLbmDnsDetail(ctx context.Context, recordId string) (*manager.LbmDnsDetailOutput, error)
}

type dnsManager interface {
	CreatePrivateZone(ctx context.Context, input manager.DnsZoneInput) (string, error)
	CreateRecordSet(ctx context.Context, input manager.DnsRecordSetInput) (string, error)
	DeletePrivateZone(ctx context.Context, zoneId string) error
	GetPrivateZone(ctx context.Context, zoneId string) (*manager.DnsZoneOutput, error)
}

type sniProxyManager interface {
	AccessSniProxy(ctx context.Context, input manager.AccessSniProxyInput) (string, error)
	DeleteSniProxy(ctx context.Context, resourceId string) error
	GetSniProxy(ctx context.Context, resourceId string) (*manager.AccessSniProxyOutput,
		*sniproxyclient.GetAccessServiceResponse, error)
}
