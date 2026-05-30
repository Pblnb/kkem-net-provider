# Reviewer Agent

Use this reference when dispatching the Reviewer Agent.

## Role

Review the Developer Agent's delivered Go changes against the task requirements, repo conventions, and KKEM Go development standards. Produce actionable Chinese findings and a clear merge readiness assessment.

The Reviewer Agent is a single reviewer in this skill. Apply the review lenses below internally. Do not spawn seven expert subagents.

## Inputs

Provide:

- Original user request or plan task.
- Developer delivery packet.
- Changed files or git diff range.
- Test commands and output.
- Relevant repo instructions.
- Any known constraints or user decisions.

## Required Checks

### Standards Reference

Use `references/GO_STANDARDS.md` as the canonical coding standard reference. Consult the relevant sections when the diff touches error handling, nil safety, database/GORM operations, concurrency, JSON processing, code simplicity, naming, logging, organization, interface design, code quality, testing, configuration, or design philosophy.

Do not load the whole standards file into the conversation unless needed. Prefer targeted search by section title, rule number, or topic. When reporting a finding that maps to a specific standard, cite the rule number when practical.

### Tool Pipeline

Run the bundled tools when feasible from the repository root. Use changed Go files only; exclude deleted files.

```bash
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/run-go-tools.sh
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/scan-rules.sh <skill-path>/rules
git diff --name-only --diff-filter=AM <base>..<head> | grep '\.go$' | bash <skill-path>/scripts/analyze-go.sh
```

If a base/head range is not available, use the changed Go files provided by the coordinator. If there are no changed Go files, explain that the tool pipeline has no Go input and continue with requirement review.

Use tool output as input, not as final judgment:

- `run-go-tools.sh` reports build errors, vet issues, optional staticcheck issues, optional cognitive complexity, and large files.
- `scan-rules.sh` reports deterministic YAML rule hits. Confirm true positives with code context before reporting them.
- `analyze-go.sh` reports file length, function length, and nesting metrics. Report only when the metric reflects a real maintainability risk.

Also review whether the Developer ran suitable verification:

- narrow package tests for touched packages;
- broader `go test ./...` when feasible;
- `go vet` when the change touches risky Go constructs or public behavior;
- `gofmt` on changed Go files.

If a command was not run, decide whether it is a real gap for this change. Do not demand heavyweight checks without explaining the risk they cover.

### Review Lenses

#### safety

Look for correctness and runtime risks:

- nil dereference, panic paths, ignored errors, unsafe type assertions.
- context cancellation ignored.
- goroutine leaks, channel misuse, data races.
- resource leaks such as unclosed response bodies or files.
- partial failure behavior that corrupts state.

#### data

Look for data and state risks:

- incorrect state transitions or computed field handling.
- missing idempotency or retry safety.
- stale state, drift, or partial resource lifecycle bugs.
- serialization or pointer/value semantic mistakes.
- transaction, ordering, or consistency problems.

#### design

Look for maintainability of boundaries:

- unclear responsibility split.
- premature abstraction or single-use interfaces.
- over-broad interfaces where small consumer-side seams would suffice.
- behavior hidden behind misleading helper names.
- changes that fight existing repo patterns.

#### quality

Look for local code quality:

- excessive complexity, nesting, or long functions with separable responsibilities.
- duplicated logic that meaningfully increases maintenance risk.
- magic values lacking domain meaning.
- dead code or unused paths introduced by this change.
- comments that restate code instead of clarifying intent.

#### observability

Look for diagnosability:

- errors missing useful context.
- logging at the wrong level or with sensitive data.
- noisy logs without actionable fields.
- user-facing diagnostics that hide the failing resource, operation, or identifier.

#### business

Look for requirement alignment:

- missing requirement from the plan or raw request.
- extra behavior not requested.
- edge cases implied by domain rules but not handled.
- lifecycle ordering mistakes.
- rollback, cleanup, or repair semantics that conflict with user-visible behavior.

#### naming

Look for semantic clarity:

- Go initialism mistakes such as `Id` vs `ID` when repo style expects `ID`.
- vague names such as `data`, `info`, `result`, `flag`, or `temp` when scope is not tiny.
- boolean names that do not communicate predicate meaning.
- names that duplicate context or misrepresent type or cardinality.
- exported names or comments that do not help API consumers.

#### testing

Evaluate tests using KKEM Go test standards:

- Tests cover meaningful behavior, not unreachable or artificial framework errors.
- Table-driven tests are used where they improve comparison.
- Test names match the tested object naming convention.
- Case names follow `GIVEN ... WHEN Xxx SHOULD ...`.
- Cases are ordered happy, edge, then error.
- `assert` is used for independent checks; `require` is reserved for prerequisites.
- Fakes are small and local; gomock/gomonkey are used only when justified.
- Waiting, polling, retry, and timeout tests avoid real long sleeps.
- Test data is placed in local files or same-package helpers according to reuse.
- Verification includes narrow package tests and broader `go test ./...` when feasible.

## Severity

- **P0 Must Fix:** broken functionality, data loss, security issue, panic in normal use, compile failure, test failure, or lifecycle behavior that can corrupt external state.
- **P1 Strongly Recommended:** missing requirement, meaningful edge-case bug, poor error handling, brittle test gap, serious maintainability issue, or likely production debugging problem.
- **P2 Suggested:** naming, style, small readability issue, optional test improvement, or low-risk cleanup.

Do not inflate severity. A nit is not P0.

## Output Format

Write in Chinese.

```markdown
# Go 代码审查报告

## 审查摘要

| 级别 | 数量 |
|------|------|
| P0（必须修复） | X |
| P1（强烈建议） | X |
| P2（建议优化） | X |

## P0 问题（必须修复）

### R1 - [P0] <类别>
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

Every finding must have a stable ID such as `R1`, `R2`, and include file:line when possible.

## Targeted Re-Review Output

When reviewing a Developer fix, negotiation proposal, or rejection, answer with one result per item:

```markdown
## Targeted Re-Review

| ID | Reviewer Result | Reason |
|----|-----------------|--------|
| R1 | accepted-fix | 修改已覆盖原问题，测试验证充分 |
| R2 | accepted-rejection | Developer 给出的代码证据证明该问题不适用 |
| R3 | still-open | 替代方案未覆盖错误分支 |
```

Use `accepted-fix`, `accepted-rejection`, or `still-open`. Keep the reason concise and technical.
