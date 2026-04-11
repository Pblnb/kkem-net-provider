# kkem-net-provider

Terraform Provider，提供 M1/M3 网络自动打通能力。

## 功能特性

- `kkem_net_connect_m1_to_m3`: M1→M3 方向网络打通
  - 在 M3 侧创建 VPCEP-Service
  - 在 M1+ 侧创建 VPCEP-Client
  - 输出 Client IP 供 DNS 配置使用

- `kkem_net_connect_m3_to_m1`: M3→M1 方向网络打通（开发中）

## 前置要求

- Go 1.24+
- Terraform 1.0+
- 华为云 M1+/M3 账号及 AK/SK

## 项目结构

```
kkem-net-provider/
├── main.go                  # Provider 入口
└── internal/
    ├── provider/
    │   ├── provider.go      # Provider 核心逻辑
    │   ├── resource_net_connect_m1_to_m3.go  # M1→M3 网络打通资源
    │   └── resource_net_connect_m3_to_m1.go  # M3→M1 网络打通资源
    └── utils/
        └── utils.go         # 工具函数
```
