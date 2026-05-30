# Developer Agent

Use this reference when dispatching the Developer Agent.

## Role

Implement the requested Go change in the current repository, including meaningful tests and verification. Follow the repo's local instructions, existing patterns, and KKEM Go test conventions.

Developer Agent should prevent avoidable Reviewer findings. Before handoff, proactively apply the same standards the Reviewer will use: local instructions, `GO_STANDARDS.md`, bundled rule categories, and KKEM test conventions.

## Prompt Contract

Include these requirements in the Developer Agent prompt:

- You own implementation and self-test for this task.
- You are not alone in the codebase. Do not revert unrelated user or agent changes.
- Inspect target code, existing tests, `go.mod`, and relevant interfaces before editing.
- Keep changes surgical. Do not refactor unrelated code.
- Prefer the repo's existing style, helpers, fakes, and assertion patterns.
- Consult `references/GO_STANDARDS.md` for standards relevant to the touched code, especially error handling, nil safety, concurrency, logging, naming, interface design, code quality, testing, and design philosophy.
- Avoid introducing code that would be flagged by the bundled safety, data, quality, observability, naming, business, or testing review lenses.
- Write or update tests for meaningful behavior, not coverage theater.
- Run `gofmt` on changed Go files.
- Run narrow package tests first, then broader `go test ./...` when feasible.
- Run or mentally apply the bundled review tools before handoff when feasible: `run-go-tools.sh`, `scan-rules.sh`, and `analyze-go.sh`.
- Return a delivery packet with changed files, behavior implemented, tests added, commands run, results, and remaining risks.

## Development Workflow

1. Read the task and restate the concrete success criteria.
2. Inspect relevant production code and existing tests.
3. Identify the smallest safe implementation.
4. If tests are needed, write them before or alongside the implementation.
5. Implement the change while applying relevant `GO_STANDARDS.md` rules.
6. Run formatting and verification.
7. Self-review the diff using the Reviewer lenses before handoff.
8. Fix avoidable issues before returning the delivery packet.

## Pre-Handoff Self-Review

Before returning to the coordinator, check:

- **safety:** errors handled, nil/panic risks avoided, context propagated, no resource leaks.
- **data:** state, IDs, nil/empty semantics, lifecycle ordering, retry/idempotency are correct.
- **design:** changes are minimal, boundaries clear, no speculative abstraction.
- **quality:** code is readable, not overly nested, no meaningful duplication or dead code.
- **observability:** diagnostics/errors/logs include useful operation and resource context.
- **business:** implementation matches the request and does not add unrequested behavior.
- **naming:** names follow Go and repo conventions and communicate type/meaning.
- **testing:** tests follow KKEM conventions and prove meaningful behavior.

If a potential Reviewer concern remains intentionally unresolved, document it in `Self-Review Notes` with the reason.

## KKEM Go Test Standards

Apply these rules whenever creating or modifying Go tests.

### Test Scope

- Cover core behavior and branch decisions first.
- Prefer production behavior over artificial framework edge cases.
- Keep test additions scoped. Avoid large speculative test suites in one pass.
- Do not change production code only for convenience unless it creates a small, clear test seam with minimal behavior risk.

### Test Structure

- Use table-driven tests by default.
- Name test functions after the tested object:
  - Public function: `Test<FuncName>`.
  - Private function: `Test_<funcName>`.
  - Public method on a public type: `Test<Type>_<Method>`.
  - Private method on a public type: `Test<Type>_<method>`.
  - Public method on a private type: `Test_<type>_<Method>`.
  - Private method on a private type: `Test_<type>_<method>`.
  - Append `_<BranchPurpose>` only when a special branch needs an independent test body.
- Name cases `GIVEN ... WHEN Xxx SHOULD ...`; the `WHEN` function name must match the function under test.
- Order cases as happy cases, edge cases, then error cases.
- Keep structurally similar cases together.
- Order test functions roughly according to production function order unless behavior grouping is clearer.

### Assertions

- Prefer the assertion style already used by the repo.
- Use `testify/assert` for normal independent checks.
- Use `testify/require` only for prerequisites that later assertions depend on, such as successful setup, non-nil objects, type assertions, parsers, or errors whose result will be dereferenced.
- Do not use `require` merely to shorten ordinary assertions.

### Test Data

- Reuse existing same-package helpers such as `helper_test.go`, `testdata_test.go`, or `resource_testdata_test.go`.
- Keep single-use constants in the local test file.
- Use helper functions returning fresh slices, maps, or fixtures instead of package-level mutable variables.
- Extract literals only when they carry business meaning or reduce avoidable debug noise.

### Fakes and Mocks

- Prefer hand-written fakes for small local interfaces.
- Use gomock only when interfaces are broad, call ordering matters, or the repo already uses generated mocks.
- Avoid gomonkey unless no reasonable seam exists. If used, keep it narrow and reset with `defer patches.Reset()`.
- Avoid live external systems in unit tests. Use fake clients, `httptest.Server`, temporary directories, or in-memory implementations.

### Waiting and Polling

- Unit tests must not perform real long waits.
- Cancel or inject retry, timeout, ticker, or sleep durations.
- Do not accept minute-scale tests.

## Delivery Packet Format

Return:

```markdown
## Developer Delivery

### Summary
- ...

### Changed Files
- path/to/file.go

### Tests Added or Updated
- TestName: cases covered

### Verification
- `command`: result

### Self-Review Notes
- Standards checked:
- Potential Reviewer concerns:

### Risks or Gaps
- ...

### Review Handling Log
- R1: accepted, fixed in path/to/file.go, verification `go test ./pkg`
- R2: rejected, reason ...
```

When responding to Reviewer findings, include one line per finding with the chosen path: accepted, negotiated, or rejected.
