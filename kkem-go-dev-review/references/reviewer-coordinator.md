# Reviewer Coordinator

Use this reference after Developer Agent returns a delivery packet.

## Role

Run review preflight tools, dispatch parallel expert reviewers, aggregate findings, deduplicate issues, and produce the canonical review report.

## Tool Preflight

Run from the repository root on changed Go files only, excluding deleted files:

```bash
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/run-go-tools.sh > /tmp/diagnostics.json
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/scan-rules.sh <skill-path>/rules > /tmp/rule-hits.json
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/analyze-go.sh > /tmp/go-structure.json
```

If a base/head range is unavailable, use changed Go files supplied by the main coordinator. If there are no changed Go files, explain that the tool pipeline has no Go input and continue with requirement review.

Tool outputs:

- `run-go-tools.sh`: build errors, vet issues, optional staticcheck, optional cognitive complexity, large files.
- `scan-rules.sh`: deterministic YAML rule hits. Expert reviewers must confirm true positives.
- `analyze-go.sh`: file/function metrics. Expert reviewers report only real maintainability risk.

Pass `/tmp/diagnostics.json`, `/tmp/rule-hits.json`, and `/tmp/go-structure.json` contents or paths to expert reviewers. If tools cannot run, include the error and let experts continue with code review rather than inventing tool results.

## Standards Reference

Use `references/GO_STANDARDS.md` as the canonical coding standard reference. Give expert reviewers the relevant rule sections or tell them which topics to search when their lens touches error handling, nil safety, database/GORM operations, concurrency, JSON processing, simplicity, naming, logging, organization, interface design, testing, configuration, or design philosophy.

Do not load the whole standards file into every expert prompt unless the diff is broad enough to justify it. Prefer targeted section lookup by rule number, topic, or table of contents heading. When aggregating a finding that maps to a specific standard, cite the rule number when practical.

## Expert Dispatch

Dispatch expert reviewers in parallel when possible:

- `expert-reviewers/safety.md`
- `expert-reviewers/data.md`
- `expert-reviewers/design.md`
- `expert-reviewers/quality.md`
- `expert-reviewers/observability.md`
- `expert-reviewers/business.md`
- `expert-reviewers/naming.md`
- `expert-reviewers/testing.md`

Give each expert:

- original request or plan task;
- Developer delivery packet;
- changed files or git diff;
- tool outputs relevant to the expert;
- local project constraints.

## Aggregation

Merge expert outputs into one canonical report:

- Deduplicate same-location or same-root-cause findings.
- Keep the highest justified severity when experts disagree.
- Downgrade inflated severity when impact does not match P0/P1.
- Prefer actionable findings with file:line references.
- Drop tool false positives after code-context confirmation.
- Preserve expert attribution in the issue title, such as `safety/tool`, `testing`, or `business`.

## Canonical Review Output

Write in Chinese:

```markdown
# Go 代码审查报告

## 审查摘要

| 级别 | 数量 |
|------|------|
| P0（必须修复） | X |
| P1（强烈建议） | X |
| P2（建议优化） | X |

## P0 问题（必须修复）

### R1 - [P0] <类别>（来源：<expert/tool>）
**位置**: path/to/file.go:行号
**问题**: ...
**影响**: ...
**建议**: ...

## P1 问题（强烈建议）
...

## P2 问题（建议优化）
...

## 验证评价
- 已看到的验证: ...
- 缺失但建议补充的验证: ...

## 结论
**是否可合入**: Yes | No | With fixes
**原因**: ...
```

Every finding must have a stable ID such as `R1`, `R2`.

## Targeted Re-Review

For fixes, negotiations, or rejections, route the item back to the expert that raised it when possible. If several experts raised the same canonical item, route to Reviewer Coordinator.

Use:

```markdown
## Targeted Re-Review

| ID | Reviewer Result | Reason |
|----|-----------------|--------|
| R1 | accepted-fix | 修改已覆盖原问题，测试验证充分 |
| R2 | accepted-rejection | Developer 给出的代码证据证明该问题不适用 |
| R3 | still-open | 替代方案未覆盖错误分支 |
```

Valid results: `accepted-fix`, `accepted-rejection`, `still-open`.
