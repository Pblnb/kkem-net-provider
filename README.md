# kkem-net-provider

Terraform Provider，提供 M1/M3 网络自动打通能力。

## 功能特性

- `kkem_net_connect_m1_to_m3`: M1→M3 方向网络打通
    - 在 M3 侧创建 VPCEP-Service
    - 在 M1+ 侧创建 VPCEP-Endpoint
    - 输出 Client IP 供 DNS 配置使用

- `kkem_net_connect_m3_to_m1`: M3→M1 方向网络打通（开发中）

## 前置要求

- Go 1.24+
- Terraform 1.0+
- 华为云 M1+/M3 账号及 AK/SK

## 项目结构

本项目采用 **三层架构（Resource → Service → Client）** 组织代码，各层职责分离如下：

```
internal/
├── provider/                         # Resource 层：Terraform 资源生命周期管理
│   ├── provider.go                   # Provider 核心逻辑（Schema、Configure、Client 初始化）
│   ├── resource_net_connect_m1_to_m3.go  # M1→M3 网络打通资源（VPCEP-Service + VPCEP-Client + LBM-DNS）
│   └── resource_net_connect_m3_to_m1.go  # M3→M1 网络打通资源（VPCEP-Client + 标准 DNS）
├── service/                          # Service 层：业务逻辑封装
│   ├── const.go                      # 共享常量（轮询间隔/超时/错误容忍）
│   ├── vpcep_service.go              # VPCEP Service 业务封装（创建/删除/权限/状态同步）
│   ├── vpcep_endpoint.go             # VPCEP Endpoint 业务封装（原子化创建+等待/删除/查询）
│   ├── dns.go                        # 标准 DNS 业务封装（PrivateZone/RecordSet）
│   ├── lbm_dns.go                    # LBM DNS 业务封装（创建/删除/更新/查询）
│   └── utils.go                      # Service 层工具函数（重试/404判断/指针包装）
└── client/                           # Client 层：外部 API 客户端封装
    └── lbmdnsclient/                 # LBM-DNS 客户端
        ├── dns_client.go             # LBM-DNS API 调用封装（异步任务发起/轮询）
        ├── http_client.go            # HTTP 基础封装（请求构造/响应解析）
        └── types.go                  # LBM-DNS 请求/响应结构体定义
```

### 分层职责

| 层级           | 目录                       | 职责                                                                                                        |
|--------------|--------------------------|-----------------------------------------------------------------------------------------------------------|
| **Resource** | `internal/provider/`     | 对接 Terraform Plugin Framework，处理 Schema 定义、State 读写、Diagnostics 上报，**不直接调用 SDK**                          |
| **Service**  | `internal/service/`      | 封装云资源业务逻辑（创建顺序、轮询等待、错误处理、404 语义化），通过**细粒度接口**隔离具体 SDK                                                     |
| **Client**   | `internal/client/`       | 封装底层 API 调用细节。当前包含 `lbmdnsclient/`（LBM DNS 客户端），后续可扩展 `sniproxyclient/` 等 |

### 关键设计

- **接口隔离**：`VpcepServiceClient` / `VpcepEndpointClient` / `DnsServiceClient` 等接口仅暴露该 Service 所需的 SDK 方法，便于测试时 Mock
- **原子化操作**：`VpcepEndpointService.Create` 将"创建 + 轮询等待 Ready"合并为单次调用，Resource 层无需关心轮询细节
- **共享常量**：`const.go` 统一管理所有轮询参数（interval / timeout / errTolerance），避免各文件重复声明

## 开发指南

### 📦 模型字段类型选型：Framework 封装类型 vs. Go 原生类型

在定义 Provider、Resource 或 Data Source 的数据模型（Model）时，我们可以选择使用 Terraform Plugin Framework 提供的封装类型（如 `types.String`）或 Go 的原生类型（如 `string`）。

正确选择字段类型，不仅能提升代码的可读性，还能避免运行时的状态解析错误。

#### 🧭 核心差异：状态表达能力
Terraform 的状态管理非常严格，一个字段通常有三种状态：**已知值（Value）**、**未配置（Null）** 和 **未知（Unknown，通常在 Apply 后才可知）**。

- **Framework 封装类型（如 `types.String`）**：
  原生支持 Terraform 的完整状态语义。它可以明确区分“用户传入了空字符串 `""`”和“用户根本没有配置该字段（`null`）”。
- **Go 原生类型（如 `string`）**：
  Go 的基础类型不具备指针特性，无法表达 `null` 或 `unknown` 状态（未赋值时仅为默认的零值，如 `""`）。如果 Terraform 引擎向一个原生类型字段注入 `null` 值，会导致严重的运行时报错。

#### ⚖️ 开发体验权衡
虽然封装类型在状态表达上更安全、功能更强大，但在实际编码中，访问其内部值需要调用特定的方法（例如 `plan.Name.ValueString()`），这在一定程度上增加了代码的繁琐度。相比之下，原生类型可以直接读写，代码更加简洁直观。

#### 💡 最佳实践选型指南

综合考虑**运行安全性**与**代码简洁度**，建议按照以下 Schema 属性来决定模型字段的类型：

| **Schema 字段属性** | **推荐数据类型** | **适用场景与原因** |
| :--- | :--- | :--- |
| **`Required: true`** | **Go 原生类型**<br/>*(如 `string`, `int64`)* | **必填参数**。由于 Terraform 引擎会强制校验必填项，该字段永远不可能为 `null` 或 `unknown`，使用原生类型可最大化提升代码可读性。 |
| **`Optional: true`** | **Framework 封装类型**<br/>*(如 `types.String`)* | **可选参数**。用户可能未配置该字段，必须使用封装类型来安全接收并处理潜在的 `null` 状态。 |
| **`Computed: true`** | **Framework 封装类型**<br/>*(如 `types.String`)* | **计算参数**。资源创建前状态通常为 `unknown`，且可能由远端 API 返回 `null`，必须使用封装类型以保证状态同步的准确性。 |

> **⚠️ 注意**：如果一个字段同时具备 `Optional: true` 和 `Computed: true`，请务必使用 Framework 封装类型。

### 测试函数命名规范：
- 公开函数：Test<FuncName>
  示例：TestNewLbmDnsService

- 私有函数：Test_<funcName>
  示例：Test_isLbmDnsNoChanges

- 公开类型的公开方法：Test<Type>_<Method>
  示例：TestLbmDnsService_CreateIntranetDnsDomain

- 公开类型的私有方法：Test<Type>_<method>
  示例：TestLbmDnsService_waitForTaskCompleted

- 私有类型的公开方法：Test_<type>_<Method>
  示例：Test_lbmDnsService_CreateIntranetDnsDomain

- 私有类型的私有方法：Test_<type>_<method>
  示例：Test_lbmDnsService_waitForTaskCompleted

- 对于需要对一个被测函数使用多个测试函数覆盖特定分支，额外添加 _<BranchPurpose> 后缀，例如
  示例：Test_lbmDnsService_waitForTaskCompleted_errorCases
