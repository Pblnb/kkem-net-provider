---
name: kkem-go-dev-review
description: Use when Codex needs to implement KKEM-style Go development work with disciplined tests and a deep Developer/Reviewer workflow. Supports raw development requests and existing spec/plan files. Use for Go feature work, bug fixes, refactors, test generation, or provider/service/client changes where the Developer Agent must write or update tests following KKEM Go conventions, then a Reviewer Coordinator must run bundled Go review tools, dispatch parallel expert reviewers, aggregate findings, and close every finding by accept, negotiate, reject, coordinator decision, or user decision.
---

# KKEM Go Dev Review

## Overview

Run KKEM-style Go development with deep parallel review. The workflow uses a Developer Agent for implementation, a Reviewer Coordinator for review orchestration, and parallel expert reviewers for safety, data, design, quality, observability, business, naming, and testing.

This skill is self-contained. Do not rely on external `code-review-go`, `kkem-go-test-generate`, or another local `kkem-go-dev-review` skill at execution time.

## Inputs

Accept either form:

- **Plan-driven:** a spec, plan, issue, MR description, or task list file. Read it, extract the required work, and execute it.
- **Raw request:** a direct development request. Inspect the repo, state assumptions, ask only if ambiguity blocks safe implementation, then create a short working plan before development.

If the repo has local instructions such as `AGENTS.md`, follow them as higher-priority project context.

## Workflow

1. **Prepare context**
   - Tell the user in one or two sentences what context you are gathering and what success criteria you will use.
   - Inspect repo state, relevant files, existing tests, and project instructions.
   - Define success criteria and verification commands.
   - For raw requests, write a brief implementation plan. For existing plan files, summarize tasks and constraints.

2. **Dispatch Developer Agent**
   - Tell the user that Developer Agent is implementing the change and what files or packages it owns.
   - Read `references/developer-agent.md`.
   - Provide the task, constraints, owned files or modules, expected tests, and verification commands.
   - Require implementation, KKEM-style tests, relevant `GO_STANDARDS.md` compliance, review-lens self-check, verification, self-review, and a delivery packet.

3. **Run reviewer preflight**
   - Tell the user that Reviewer Coordinator is running tool checks before expert review.
   - Read `references/reviewer-coordinator.md`.
   - Run the bundled scripts on changed Go files when feasible:
     - `scripts/run-go-tools.sh`
     - `scripts/scan-rules.sh`
     - `scripts/analyze-go.sh`
   - Treat tool output as evidence for expert reviewers, not as final judgment.

4. **Dispatch parallel expert reviewers**
   - Tell the user which expert reviewers are being dispatched.
   - Read only the relevant files under `references/expert-reviewers/`.
   - Dispatch independent reviewers in parallel when subagents are available:
     - safety, data, design, quality, observability, business, naming, testing.
   - Each expert reviews the same delivered diff from its lens and returns Chinese findings with stable IDs.

5. **Aggregate review**
   - Reviewer Coordinator deduplicates expert findings, resolves severity conflicts, and produces one canonical Chinese review report.
   - Findings must be P0/P1/P2 with file:line references when possible.

6. **Close review items**
   - Tell the user when findings are being fixed, disputed, escalated, or sent back for targeted re-review.
   - Read `references/review-closure-protocol.md`.
   - Track every finding as `open`, `accepted`, `negotiating`, `rejected-pending-reviewer`, or `closed`.
   - Route each item through accept, negotiate, reject, coordinator decision, coordinator override, or user decision.
   - Re-dispatch the relevant expert reviewer or Reviewer Coordinator for targeted re-review until every item is `closed`.

7. **Final verification**
   - Run narrow tests first, then broader verification when feasible.
   - Do not claim completion until verification output has been checked.
   - Report changed files, commands run, remaining risks or gaps, and every Reviewer finding with final handling result.

## User Updates

Give concise progress updates at important handoff points:

- before Developer Agent starts;
- after Developer Agent returns the delivery packet;
- before Reviewer Coordinator runs tools;
- before parallel expert reviewers start;
- when findings are accepted, disputed, or escalated;
- before final verification.

Keep updates short. One or two sentences is enough. Do not paste full prompts or full review reports unless the user asks.

## Agent Rules

- Use one Developer Agent.
- Use one Reviewer Coordinator.
- Use parallel expert reviewer agents for deep review when subagents are available.
- Keep the main coordinator responsible for user-facing updates, state tracking, and final decisions.
- Keep implementation ownership with Developer Agent. After reviewers raise findings, route accepted or negotiated fixes back to Developer Agent by default.
- Main coordinator must not silently implement reviewer findings. It may take over code edits only when Developer Agent is clearly blocked, off-track, repeatedly unavailable, or the user explicitly asks for coordinator override; announce the override before editing and record it in the closure summary.
- Do not commit or push unless the user explicitly asks.
- Do not skip review because the change is small.
- Do not leave P0 or P1 findings unresolved.
- P2 findings may close by Developer fix, reviewer-accepted rejection, coordinator decision, coordinator override with reviewer confirmation, or explicit user decision.
- If Developer Agent and expert reviewers cannot agree, the main coordinator must make a technical decision when possible. Ask the user only when the coordinator cannot decide safely.

## Superpowers Compatibility

This skill can run after Superpowers brainstorming or planning. Treat Superpowers spec/plan files as plan-driven inputs.

After all items close, you may suggest an optional independent final review with Superpowers, but do not invoke it automatically unless the user asks.

## References

- `references/workflow-overview.md`: Human-readable Mermaid workflow diagrams and role summary.
- `references/developer-agent.md`: Developer Agent prompt rules and KKEM Go test-development standards.
- `references/reviewer-coordinator.md`: Tool preflight, expert dispatch, aggregation, and deduplication rules.
- `references/expert-reviewers/*.md`: Expert reviewer lenses and output contracts.
- `references/review-closure-protocol.md`: Per-finding closure state machine and escalation rules.
- `references/GO_STANDARDS.md`: Complete Go coding standards used as the canonical review reference.

## Bundled Review Tools

- `scripts/run-go-tools.sh`: Runs Go build/vet, optional staticcheck/gocognit, and large-file checks; emits diagnostics JSON.
- `scripts/scan-rules.sh`: Scans changed Go files against `rules/*.yaml`; emits deterministic rule-hit JSON.
- `scripts/analyze-go.sh`: Computes structural file/function metrics such as line count and nesting depth.
- `rules/*.yaml`: Regex-based safety, data, quality, and observability rules used by `scan-rules.sh`.
