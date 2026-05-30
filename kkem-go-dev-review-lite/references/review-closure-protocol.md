# Review Closure Protocol

Use this reference after the Reviewer Agent returns findings.

## Coordinator Responsibilities

Maintain a review table. Do not finish until each finding is closed.

The coordinator owns routing, decisions, user updates, and closure state. Developer Agent owns code changes. Reviewer Agent owns review judgment. Do not collapse these roles during normal flow.

Track:

- ID: reviewer finding ID, such as `R1`.
- Severity: P0, P1, or P2.
- Summary.
- Decision: accept, negotiate, reject, or coordinator override.
- Developer response.
- Reviewer recheck result.
- Final handling result: Developer fixed and Reviewer accepted, Developer rejected and Reviewer accepted, negotiated fix accepted, negotiated rejection accepted, coordinator override, or user decision.
- Status.

## Status Values

- `open`: Reviewer raised the item and Developer has not responded.
- `accepted`: Developer agrees and is implementing or has implemented a fix.
- `negotiating`: Developer partially disagrees or proposes an alternate fix.
- `rejected-pending-reviewer`: Developer rejects with technical reasoning; Reviewer has not accepted the rejection yet.
- `closed`: Reviewer confirms the fix or accepts the rejection.

## Accept Path

Use when Developer agrees with the finding.

1. Ask Developer Agent to fix the item.
2. Require a short fix summary, changed files, and verification command output.
3. Send the targeted diff and fix summary to Reviewer Agent.
4. Close only when Reviewer says the item is resolved.

## Negotiate Path

Use when Developer agrees with the problem but not the exact recommendation, or the finding needs design clarification.

1. Ask Developer Agent to explain the concern and propose one or two concrete options.
2. Send the options and relevant code context to Reviewer Agent.
3. If Reviewer accepts an option, ask Developer to implement it and then request re-review.
4. If Reviewer agrees the original finding should be dropped, close it as reviewer-accepted rejection.
5. If they still disagree, the coordinator evaluates both arguments and makes a technical decision when the codebase evidence is sufficient.
6. Ask the user to decide only when the coordinator cannot decide safely, such as product behavior, architecture ownership, or risk acceptance.

## Reject Path

Use when Developer believes the finding is technically wrong or out of scope.

1. Require Developer Agent to provide a concise technical rejection:
   - why the finding does not apply,
   - what code or test proves current behavior,
   - whether any follow-up risk remains.
2. Send the rejection to Reviewer Agent.
3. Close only when Reviewer accepts the rejection.
4. If Reviewer does not accept the rejection, move to the negotiate path.
5. If negotiation stalls, the coordinator evaluates both arguments and makes a technical decision when the codebase evidence is sufficient.
6. Ask the user to decide only when the coordinator cannot decide safely.

## Escalation

Use this escalation order for unresolved disagreements:

1. Developer Agent and Reviewer Agent exchange concise technical positions through the coordinator.
2. Coordinator reviews the code, tests, requirements, and risk; then decides if the evidence is sufficient.
3. If the coordinator can decide safely, record the decision and route the item to fix or close.
4. If the coordinator cannot decide safely, ask the user with the smallest decision question possible.

Do not let agents argue indefinitely. Two focused exchanges per disputed item is usually enough before coordinator decision.

## Coordinator Override

Use coordinator override only when continuing through Developer Agent is worse for correctness or momentum:

- Developer Agent is blocked on the same issue after repeated guidance.
- Developer Agent is off-track, widening scope, or changing unrelated files.
- Developer Agent is unavailable for an unreasonable window relative to task size and has not produced a useful intermediate result.
- The user explicitly asks the coordinator to take over.

Large tasks may legitimately take longer. Treat active progress as not timed out when Developer Agent produces useful diffs, verification output, concrete blockers, or clear next steps. Before overriding, send a focused convergence instruction when that is likely to recover the flow.

When override is used:

1. Announce the override and why it is safer than waiting.
2. Make the smallest necessary edit.
3. Run narrow verification.
4. Send the targeted diff to Reviewer Agent for confirmation.
5. Record the final result as `coordinator override` in the closure summary.

## Batch Handling

- Process independent simple accepts together when they touch disjoint code.
- Do not batch unclear, architectural, or disputed findings.
- Re-run narrow tests after each meaningful fix batch.
- Re-run broader tests before final completion when feasible.

## Closure Summary Format

Use this format in the final response:

```markdown
## Review Closure

| ID | Severity | Decision | Result |
|----|----------|----------|--------|
| R1 | P1 | accepted | Developer fixed, Reviewer accepted |
| R2 | P2 | rejected | Developer rejected, Reviewer accepted |
| R3 | P1 | negotiated | Alternative fix implemented, Reviewer accepted |
| R4 | P2 | coordinator decision | Coordinator accepted Developer rejection |
| R5 | P2 | coordinator override | Coordinator fixed after Developer Agent became blocked, Reviewer accepted |
```

Also include a short bullet list below the table for any item that required coordinator or user decision.

## Rules

- Never silently ignore a finding.
- Never mark a finding closed based only on Developer self-review.
- P0 and P1 require Reviewer confirmation or explicit user override.
- P2 may close by Reviewer confirmation, accepted rejection, coordinator override with Reviewer confirmation, or explicit user decision.
- Keep responses technical. Avoid performative agreement.
