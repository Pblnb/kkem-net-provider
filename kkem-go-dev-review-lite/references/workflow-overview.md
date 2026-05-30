# Workflow Overview

## 整体流程

```mermaid
flowchart TD
    A[用户需求或计划文件] --> B[主 Agent 准备上下文]
    B --> C[Developer Agent 开发代码与测试]
    C --> D[Developer 预防性自检]
    D --> E[Reviewer Agent 审查交付代码]
    E --> F{是否发现问题}
    F -->|否| G[最终验证与交付]
    F -->|是| H[主 Agent 建立 Review 问题表]
    H --> I{逐条处理}
    I -->|接受| J[Developer 修改]
    I -->|协商| K[Developer 与 Reviewer 讨论方案]
    I -->|驳回| L[Developer 给出技术理由]
    J --> M[Reviewer 定向复审]
    K --> N{达成一致?}
    L --> M
    N -->|修改| J
    N -->|驳回| M
    N -->|无法一致| O[主 Agent 决策]
    O -->|可安全决策| J
    O -->|无法安全决策| P[用户决策]
    P --> J
    M --> Q{问题是否闭环}
    Q -->|否| I
    Q -->|是| R{是否还有问题}
    R -->|有| I
    R -->|无| G
```

## 角色分工

- **主 Agent**：准备上下文、派发 Developer/Reviewer、维护 review 状态表、处理争议升级、向用户简短汇报关键环节。
- **Developer Agent**：按需求开发代码和测试，主动遵循 `GO_STANDARDS.md`、KKEM 测试规范和 review lenses，交付前尽量修掉可预防问题。
- **Reviewer Agent**：运行或参考内置工具链，按 safety/data/design/quality/observability/business/naming/testing 维度审查，并输出中文 P0/P1/P2 问题。

## 闭环结果

最终交付时必须列出每个 Reviewer 问题的处理结果：

- Developer 修改后 Reviewer 接受。
- Developer 直接驳回后 Reviewer 接受。
- 双方讨论后修改并被 Reviewer 接受。
- 双方讨论后驳回并被 Reviewer 接受。
- 主 Agent 决策后闭环。
- 用户决策后闭环。
