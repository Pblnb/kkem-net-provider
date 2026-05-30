---
name: address-review-comments
description: "Use when addressing inline review comments marked with cmt:, review comments, MR comments, or reviewer feedback embedded in code comments."
---

# Address Review Comments

## Workflow

1. Locate review comments precisely. Use `rg -n --fixed-strings "cmt:"` in the user-specified files or directories. If no scope is specified, run it from the current workspace root and then read only matching files plus nearby context. Do not scan or load every repository file.
2. Treat only real code comments as review comments. Supported prefixes include `// cmt:`, `# cmt:`, and `<!-- cmt: ... -->`. Ignore matches inside string literals, generated output, vendored code, or unrelated data unless the user explicitly asks for them.
3. For each `cmt:` block, inspect adjacent comments and the same continuous comment block for `think:` entries. Treat `think:` as the user's suggested handling or reasoning, not as an instruction to follow blindly.
4. Judge the comment before editing. Accept comments that improve correctness, maintainability, tests, or clarity. Reject comments that are wrong, unnecessary, too broad for the request, conflict with existing design, or create worse tradeoffs.
5. When accepting a comment, make the smallest code change that fully addresses it. Remove the handled `cmt:` and related `think:` comments after the change.
6. When rejecting a comment, do not change unrelated code. Remove the handled `cmt:` and related `think:` comments, then explain the rejection in the final response.
7. Look for similar issues after understanding the accepted comment's underlying pattern. Prefer targeted searches based on symbols, literals, error patterns, or nearby logic. If repository-wide repair is too broad, at minimum inspect and fix matching issues in files changed on the current branch, using `git diff --name-only` or the best available branch-base comparison.
8. Verify with the narrowest meaningful formatter/tests/checks for the changed files, then broaden only when useful.

## Response Format

Always reply for every review comment handled:

- `Accepted`: summarize the implemented change and where it was made.
- `Rejected`: explain the technical reason for not adopting the suggestion.
- `Comment replies`: provide one concise reply for each individual review comment, suitable for the user to paste back to the reviewer. For accepted comments, state the adopted fix; for rejected comments, state the rejection reason.
- `Similar issues`: list any analogous fixes made, or say none were found in the searched scope.
- `Verification`: list commands run and whether they passed; if not run, explain why.

## Guardrails

- Do not modify code unrelated to the review comment or its proven analogous issue pattern.
- Do not ask before rejecting clearly bad advice; state the reasoning in the final response.
- Ask for confirmation only when the comment intent is ambiguous or the necessary fix would substantially expand scope.
- Preserve existing style, formatting, and ownership boundaries.
