# kkem-net-provider

`kkem-net-provider` 是一个自研 Terraform Provider，用于自动化打通 M1/M3 网络链路。

## 功能特性

### `kkem_net_connect_m1_to_m3`

M1 到 M3 方向网络打通，当前已接入真实 VPCEP 与 LBM-DNS API。

- 在 M3 侧创建 VPCEP-Service，并配置服务白名单
- 在 M1+ 侧创建 VPCEP-Endpoint，获取私网访问 IP
- 调用 LBM-DNS 创建 IntranetDnsDomain 解析记录
- Read 阶段同步 VPCEP-Service、VPCEP-Endpoint、LBM-DNS 远端真实状态
- Update 阶段支持缺失子资源修复、VPCEP-Service 原地更新或替换、VPCEP-Endpoint 替换、LBM-DNS 记录创建/替换/更新
- Delete 阶段按 `DNS -> VPCEP-Endpoint -> VPCEP-Service` 顺序清理

### `kkem_net_connect_m3_to_m1`

M3 到 M1 方向网络打通，当前已接入 M3 VPCEP-Endpoint 与华为云 DNS Private Zone 相关 API，SNI Proxy 接入仍是 TODO。

- 在 M3 侧创建 VPCEP-Endpoint，对接已有 SNI Proxy VPCEP-Service
- 可选创建 DNS Private Zone，并创建 A 记录指向 VPCEP-Endpoint IP
- Read 阶段同步 VPCEP-Endpoint 和 DNS Private Zone 存在性
- Delete 阶段按 `DNS Private Zone -> VPCEP-Endpoint` 顺序清理，SNI Proxy 删除待补齐

## 前置要求

- Go 1.24+
- Terraform 1.0+
- 华为云 M1+/M3 账号及 AK/SK
- VPCEP、DNS、LBM-DNS 服务 Endpoint
- LBM-DNS `x-open-token`

## Provider 配置

```hcl
provider "kkem" {
  m1_plus {
    ak         = "..."
    sk         = "..."
    project_id = "..."
  }

  m3 {
    ak         = "..."
    sk         = "..."
    project_id = "..."
  }

  vpcep_endpoint   = "https://vpcep.cn-north-7.myhuaweicloud.com"
  lbm_dns_endpoint = "https://lbm-app-api.myhuaweicloud.com"
  dns_endpoint     = "https://dns.cn-north-7.myhuaweicloud.com"
  x_open_token     = "..."
}
```

## 项目结构

```text
kkem-net-provider/
├── main.go
├── go.mod
├── go.sum
├── README.md
├── huaweicloud_sdk_demo.md
├── dev/
└── internal/
    ├── client/
    │   └── lbmdnsclient/
    │       ├── dns_client.go
    │       ├── http_client.go
    │       └── types.go
    ├── provider/
    │   ├── provider.go
    │   ├── resource_net_connect_m1_to_m3.go
    │   └── resource_net_connect_m3_to_m1.go
    └── service/
        ├── const.go
        ├── dns.go
        ├── lbm_dns.go
        ├── utils.go
        ├── vpcep_endpoint.go
        └── vpcep_service.go
```

## 分层说明

- `internal/provider`: Terraform Provider 和 Resource 层，负责 schema、state、diagnostics 与多子资源编排
- `internal/service`: 业务服务层，封装 VPCEP-Service、VPCEP-Endpoint、DNS、LBM-DNS 的创建、查询、更新、删除、轮询与重试
- `internal/client/lbmdnsclient`: LBM-DNS HTTP 传输层，负责请求构造与响应结构定义

## 开发命令

```bash
gofmt -w .
go test ./...
```

`dev/` 目录用于本地联调配置，包含真实凭证时禁止提交。
