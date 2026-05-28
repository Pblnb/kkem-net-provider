# Review Closure Protocol

Use this reference after Reviewer Coordinator returns canonical findings.

## Coordinator Responsibilities

Maintain a review table. Do not finish until each finding is closed.

The main coordinator owns routing, decisions, user updates, and closure state. Developer Agent owns code changes. Reviewer Coordinator and expert reviewers own review judgment. Do not collapse these roles during normal flow.

Track:

- ID.
- Severity.
- Summary.
- Source expert or tool.
- Decision: accept, negotiate, reject, coordinator decision, coordinator override, or user decision.
- Developer response.
- Reviewer recheck result.
- Final handling result: Developer fixed and Reviewer accepted, Developer rejected and Reviewer accepted, negotiated fix accepted, negotiated rejection accepted, coordinator decision, coordinator override, or user decision.
- Status.

## Status Values

- `open`: Reviewer raised the item and Developer has not responded.
- `accepted`: Developer agrees and is implementing or has implemented a fix.
- `negotiating`: Developer partially disagrees or proposes an alternate fix.
- `rejected-pending-reviewer`: Developer rejects with technical reasoning; Reviewer has not accepted the rejection yet.
- `closed`: Reviewer confirms the fix or accepts the rejection.

## Accept Path

1. Ask Developer Agent to fix the item.
2. Require fix summary, changed files, and verification output.
3. Send the targeted diff and summary to the source expert or Reviewer Coordinator.
4. Close only when the reviewer says the item is resolved.

## Negotiate Path

1. Ask Developer Agent to explain the concern and propose one or two concrete options.
2. Send options and relevant code context to the source expert or Reviewer Coordinator.
3. If reviewer accepts an option, ask Developer to implement it and request re-review.
4. If reviewer agrees the original finding should be dropped, close it as reviewer-accepted rejection.
5. If disagreement remains, the main coordinator evaluates both arguments and makes a technical decision when codebase evidence is sufficient.
6. Ask the user only when the main coordinator cannot decide safely.

## Reject Path

1. Require Developer Agent to provide concise technical rejection:
   - why the finding does not apply;
   - what code or test proves current behavior;
   - whether follow-up risk remains.
2. Send the rejection to the source expert or Reviewer Coordinator.
3. Close only when reviewer accepts the rejection.
4. If reviewer does not accept it, move to negotiate path.
5. If negotiation stalls, the main coordinator decides when safe, otherwise asks the user.

## Escalation

Use this order:

1. Developer Agent and reviewer exchange concise technical positions through the main coordinator.
2. Main coordinator reviews code, tests, requirements, and risk.
3. If evidence is sufficient, main coordinator decides and records the result.
4. If evidence is insufficient or the decision is product/architecture ownership, ask the user the smallest decision question possible.

Do not let agents argue indefinitely. Two focused exchanges per disputed item is usually enough before coordinator decision.

## Coordinator Override

Use coordinator override only when continuing through Developer Agent is worse for correctness or momentum:

- Developer Agent is blocked on the same issue after repeated guidance.
- Developer Agent is off-track, widening scope, or changing unrelated files.
- Developer Agent is unavailable for an unreasonable window relative to task size and has not produced a useful intermediate result.
- The user explicitly asks the main coordinator to take over.

Large tasks may legitimately take longer. Treat active progress as not timed out when Developer Agent produces useful diffs, verification output, concrete blockers, or clear next steps. Before overriding, send a focused convergence instruction when that is likely to recover the flow.

When override is used:

1. Announce the override and why it is safer than waiting.
2. Make the smallest necessary edit.
3. Run narrow verification.
4. Send the targeted diff to the reviewer for confirmation.
5. Record the final result as `coordinator override` in the closure summary.

## Final Closure Summary

Use this format:

```markdown
## Review Closure

| ID | Severity | Source | Decision | Result |
|----|----------|--------|----------|--------|
| R1 | P1 | safety | accepted | Developer fixed, Reviewer accepted |
| R2 | P2 | naming | rejected | Developer rejected, Reviewer accepted |
| R3 | P1 | testing | negotiated | Alternative fix implemented, Reviewer accepted |
| R4 | P2 | design | coordinator decision | Coordinator accepted Developer rejection |
| R5 | P2 | testing | coordinator override | Coordinator fixed after Developer Agent became blocked, Reviewer accepted |
```

Include a short bullet list below the table for any item requiring coordinator or user decision.

## Rules

- Never silently ignore a finding.
- Never mark a finding closed based only on Developer self-review.
- P0 and P1 require reviewer confirmation or explicit user override.
- P2 may close by reviewer confirmation, accepted rejection, coordinator decision, coordinator override with reviewer confirmation, or explicit user decision.
- Keep responses technical. Avoid performative agreement.
