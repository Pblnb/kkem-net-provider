---
name: kkem-go-dev-review
description: Use when Codex needs to implement KKEM-style Go development work with disciplined tests and an explicit Developer/Reviewer closure loop. Supports both raw development requests and existing spec/plan files. Use for Go feature work, bug fixes, refactors, test generation, or provider/service/client changes where the Developer Agent must write or update tests following KKEM Go test conventions, run verification, then hand off to a Reviewer Agent that applies KKEM Go review lenses and closes every finding by accept, negotiate, or reject.
---

# KKEM Go Dev Review

## Overview

Run KKEM-style Go development with two roles: a Developer Agent that implements and verifies the change, and a Reviewer Agent that reviews the result using embedded Go review and test-quality standards. The coordinator keeps ownership of the workflow, routes messages between agents, and does not finish until every review item is closed.

This skill is self-contained. Do not rely on external `code-review-go` or `kkem-go-test-generate` skills at execution time.

## Inputs

Accept either form:

- **Plan-driven:** a spec, plan, issue, MR description, or task list file. Read it, extract the required work, and execute it.
- **Raw request:** a direct development request. Inspect the repo, state assumptions, ask only if ambiguity blocks safe implementation, then create a short working plan before development.

If the repo already has local instructions such as `AGENTS.md`, follow them as higher-priority project context.

## Workflow

1. **Prepare context**
   - Tell the user in one or two sentences what context you are gathering and what success criteria you will use.
   - Inspect repo state, relevant files, existing tests, and project instructions.
   - Define success criteria and verification commands.
   - For raw requests, write a brief implementation plan in the conversation. For existing plan files, summarize the tasks and constraints.

2. **Dispatch Developer Agent**
   - Tell the user that Developer Agent is implementing the change and what files or packages it owns.
   - Read `references/developer-agent.md`.
   - Provide the Developer Agent the concrete task, relevant constraints, owned files or modules, expected tests, and verification commands.
   - Require the Developer Agent to edit code, write or update tests using KKEM Go test conventions, apply relevant `GO_STANDARDS.md` rules and review lenses during development, run verification, self-review, and return a delivery packet.

3. **Review the delivery**
   - Tell the user that Reviewer Agent is reviewing the delivered diff, including which tool checks or review lenses are being used.
   - Read `references/reviewer-agent.md`.
   - Dispatch the Reviewer Agent with the delivery packet, requirements, git diff range or changed files, test output, and project constraints.
   - Require the Reviewer Agent to run or evaluate the bundled review tools when feasible, then produce a Chinese review report with P0/P1/P2 findings and file:line references.

4. **Close review items**
   - Tell the user when review findings are being fixed, disputed, or sent back for targeted re-review.
   - Read `references/review-closure-protocol.md`.
   - Track every finding as an item with status `open`, `accepted`, `negotiating`, `rejected-pending-reviewer`, or `closed`.
   - Route each item through the accept, negotiate, reject, or coordinator override path.
   - Re-dispatch Reviewer Agent for targeted re-review until every item is `closed`.

5. **Final verification**
   - Run the narrowest meaningful tests first, then broader verification when feasible.
   - Do not claim completion until verification output has been checked.
   - Report changed files, commands run, remaining risks or gaps, and every Reviewer finding with its final handling result.

## User Updates

Give the user concise progress updates at important handoff points:

- before Developer Agent starts;
- after Developer Agent returns the delivery packet;
- before Reviewer Agent starts;
- when Reviewer findings are accepted, disputed, or escalated;
- before final verification.

Keep updates short. One or two sentences is enough. Do not paste full prompts or full review reports unless the user asks.

## Agent Rules

- Use exactly one Developer Agent and one Reviewer Agent by default.
- The Reviewer Agent may use multiple review lenses internally, but must not spawn seven expert subagents in this skill.
- Keep the coordinator responsible for decisions, state tracking, and user-facing updates.
- Keep implementation ownership with Developer Agent. After Reviewer raises findings, route accepted or negotiated fixes back to Developer Agent by default.
- The coordinator must not silently implement Reviewer findings. It may take over code edits only when Developer Agent is clearly blocked, off-track, repeatedly unavailable, or the user explicitly asks for coordinator override; announce the override before editing and record it in the closure summary.
- Do not commit or push unless the user explicitly asks.
- Do not skip review because the change is small.
- Do not leave P0 or P1 findings unresolved.
- P2 findings may close by Developer fix, reviewer-accepted rejection, coordinator override with Reviewer confirmation, or explicit user decision.
- If Developer Agent and Reviewer Agent cannot agree, the coordinator must make a technical decision when possible. Ask the user only when the coordinator cannot decide safely.

## Superpowers Compatibility

This skill can run after Superpowers brainstorming or planning. Treat Superpowers spec/plan files as plan-driven inputs.

After all items close, you may suggest an optional independent final review with Superpowers, but do not invoke it automatically unless the user asks.

## References

- `references/workflow-overview.md`: Human-readable Mermaid workflow diagrams and role summary.
- `references/developer-agent.md`: Developer Agent prompt rules and KKEM Go test-development standards.
- `references/reviewer-agent.md`: Reviewer Agent prompt rules, Go review lenses, and testing lens.
- `references/review-closure-protocol.md`: Per-finding closure state machine and response rules.
- `references/GO_STANDARDS.md`: Complete Go coding standards used as the canonical review reference.

## Bundled Review Tools

- `scripts/run-go-tools.sh`: Runs Go build/vet, optional staticcheck/gocognit, and large-file checks; emits diagnostics JSON.
- `scripts/scan-rules.sh`: Scans changed Go files against `rules/*.yaml`; emits deterministic rule-hit JSON.
- `scripts/analyze-go.sh`: Computes structural file/function metrics such as line count and nesting depth.
- `rules/*.yaml`: Regex-based safety, data, quality, and observability rules used by `scan-rules.sh`.
