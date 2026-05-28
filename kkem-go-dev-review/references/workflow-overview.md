# Workflow Overview

## 整体流程

```mermaid
flowchart TD
    A[用户需求或计划文件] --> B[主 Agent 准备上下文]
    B --> C[Developer Agent 开发代码与测试]
    C --> D[Developer 预防性自检]
    D --> E[Reviewer Coordinator 运行工具预检]
    E --> F[并行专家 Reviewer 审查]
    F --> G[Reviewer Coordinator 聚合去重]
    G --> H{是否发现问题}
    H -->|否| I[最终验证与交付]
    H -->|是| J[主 Agent 建立 Review 问题表]
    J --> K{逐条处理}
    K -->|接受| L[Developer 修改]
    K -->|协商| M[Developer 与相关专家讨论方案]
    K -->|驳回| N[Developer 给出技术理由]
    L --> O[相关专家或 Reviewer Coordinator 定向复审]
    M --> P{达成一致?}
    N --> O
    P -->|修改| L
    P -->|驳回| O
    P -->|无法一致| Q[主 Agent 决策]
    Q -->|可安全决策| L
    Q -->|无法安全决策| R[用户决策]
    R --> L
    O --> S{问题是否闭环}
    S -->|否| K
    S -->|是| T{是否还有问题}
    T -->|有| K
    T -->|无| I
```

## 角色分工

- **主 Agent**：准备上下文、派发 Developer/Reviewer、维护 review 状态表、处理争议升级、向用户简短汇报关键环节。
- **Developer Agent**：按需求开发代码和测试，主动遵循 `GO_STANDARDS.md`、KKEM 测试规范和 review lenses，交付前尽量修掉可预防问题。
- **Reviewer Coordinator**：运行工具预检，派发并行专家，聚合去重，形成统一中文审查报告。
- **专家 Reviewer**：从 safety/data/design/quality/observability/business/naming/testing 各自视角独立审查。

## 闭环结果

最终交付时必须列出每个 Reviewer 问题的处理结果：

- Developer 修改后 Reviewer 接受。
- Developer 直接驳回后 Reviewer 接受。
- 双方讨论后修改并被 Reviewer 接受。
- 双方讨论后驳回并被 Reviewer 接受。
- 主 Agent 决策后闭环。
- 用户决策后闭环。
