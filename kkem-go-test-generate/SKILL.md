---
name: kkem-go-test-generate
description: Generate or refactor Go tests for kkem-style provider work with disciplined table-driven cases, testify assertions, small phased changes, mocks/fakes, and coverage-oriented planning. Use when Codex is asked to create Golang unit/integration tests, improve existing Go tests, split test work into phases, or make production code minimally more testable.
---

# KKEM Go Test Generate

## Workflow

1. Inspect before writing tests: read target code, existing tests, `go.mod`, and relevant interfaces. Do not ask for facts that the repo can answer.
2. Plan test scope by risk: cover core behavior and branch decisions first, then helpers and low-risk wrappers. Keep each implementation phase or PR around 500 lines of added/changed test code unless the user overrides it. Do not split test files solely to keep a single file under 500 lines unless the user explicitly asks for per-file limits.
3. Prefer production behavior over coverage theater. Add tests for meaningful scenarios; avoid artificial inputs that only exercise unreachable framework errors.
4. Implement one phase at a time. Run `gofmt`, package tests, and then broader `go test ./...` when feasible.
5. Report files changed, functions covered, cases covered, commands run, coverage, and known remaining gaps.

## Test Structure Rules

- Use table-driven tests by default.
- Name test functions according to the tested object:
  - Public function: `Test<FuncName>`, for example `TestNewLbmDnsService`.
  - Private function: `Test_<funcName>`, for example `Test_isLbmDnsNoChanges`.
  - Public method on a public type: `Test<Type>_<Method>`, for example `TestLbmDnsService_CreateIntranetDnsDomain`.
  - Private method on a public type: `Test<Type>_<method>`, for example `TestLbmDnsService_waitForTaskCompleted`.
  - Public method on a private type: `Test_<type>_<Method>`, for example `Test_lbmDnsService_CreateIntranetDnsDomain`.
  - Private method on a private type: `Test_<type>_<method>`, for example `Test_lbmDnsService_waitForTaskCompleted`.
  - If one tested function needs an independent test body for a special branch, append `_<BranchPurpose>`, for example `Test_lbmDnsService_waitForTaskCompleted_mockCases`.
- Name cases `GIVEN ... WHEN Xxx SHOULD ...`; the `WHEN` function name must match the function under test.
- Keep case order: happy cases, edge cases, error cases.
- Within each happy/edge/error group, place structurally similar cases together for easier comparison.
- Order test functions roughly according to the order of functions in the target file, unless grouping by behavior makes the test easier to review.
- Prefer DAMP over DRY. Use small helpers only for repeated setup/decoding/assertion that would obscure the test.
- Prefer the assertion style already used by the repo/team. `testify/assert` and `testify/require` are both allowed:
  - Use `assert` for normal value checks so one failing assertion does not hide later independent assertions.
  - Use `require` only for prerequisite checks that later assertions depend on, such as type assertions, non-nil objects, successful builders/parsers, or errors whose result is dereferenced. This avoids noisy follow-up panics or misleading failures.
  - Do not introduce `require` just to shorten ordinary assertions; keep it limited to cases where continuing the test would be unsafe or confusing.

Example:

```go
func TestNormalizePorts(t *testing.T) {
    testCases := []struct {
        name     string
        input    []portBlock
        expected []portBlock
    }{
        {
            name: "GIVEN unsorted ports WHEN normalizePorts SHOULD sort by client port then server port",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            actual := normalizePorts(tc.input)
            assert.Equal(t, tc.expected, actual)
        })
    }
}
```

## Test Data Placement

- If the package already has a unified same-package test helper such as `helper_test.go`, `testdata_test.go`, or `resource_testdata_test.go` for package test data/helpers, prefer placing extracted constants, variables, and helper functions there when they represent package-level or domain-level defaults that should be discoverable, even if the first use is in one test file.
- In such unified helper files, use clear names that identify the owning domain, keep constants, variables, and helper functions from the same domain together, and separate different domains with short block comments.
- If the package has no unified helper file and constants are used by one test file only, keep them in that file.
- If multiple test files in the same package reuse constants and no unified helper file exists yet, move them to a same-package `*_test.go` helper such as `resource_testdata_test.go`.
- Only create `internal/testutils` after multiple packages need substantial shared builders or fixtures.
- Avoid shared packages for a few string constants.
- Extract literal constants only when they carry useful business meaning or are reused enough to improve readability. Repeated structure builders, Terraform values, slices, maps, and complex fixtures usually have more extraction value than bare strings.
- When a test string contains the same literal value as an extracted variable or constant, reuse that variable or constant instead of repeating the literal, so future default-value changes do not create avoidable debug noise. For example: `expectedErr: fmt.Sprintf("vpcep-endpoint %s is accepted but has no IP", testVpcepEndpointId)`.
- For reusable slice/map fixtures, prefer helper functions that return fresh values instead of package-level mutable variables.

## Testability Refactors

- Do not change production code just for convenience unless it creates a clear test seam with minimal behavior risk.
- If a resource directly depends on concrete services and lifecycle tests need fakes, define small consumer-side interfaces in the resource package.
- If a service consumes an unmodifiable SDK client, define the SDK-client interface in the service package.
- Keep interfaces minimal: include only methods used by the consumer.
- Explain any production-code testability refactor before or alongside the test change.

## Mocks and Fakes

- Prefer simple hand-written fakes for small local interfaces.
- Use gomock when interfaces are broad, call ordering matters, or the repo already uses generated mocks.
- Avoid gomonkey unless there is no reasonable interface seam; it is more fragile and should be reset with `defer patches.Reset()`.
- Do not introduce production global variables or test hooks only to force rare framework errors. If such branches must be covered and no clean seam exists, use `gomonkey` in a narrowly scoped test, always `defer patches.Reset()`, and keep the case isolated from normal behavior tests.
- Avoid live external systems in unit tests. Use local fake clients, `httptest.Server`, temporary directories, or in-memory implementations.

## Validation

Run the narrowest useful command first, then a broader one:

```bash
go test ./path/to/package -cover
go test ./...
```

If Go cache permissions fail in sandboxed environments, use a writable cache path such as:

```bash
GOCACHE=/private/tmp/project-go-build-cache go test ./...
```
