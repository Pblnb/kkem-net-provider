---
name: write-mr-description
description: "Use when drafting a Merge Request or Pull Request Description section from the current branch diff or change summary."
---

# Write MR Description

## Workflow

1. Inspect the current branch changes before writing. Prefer `git diff --stat`, `git diff --name-only`, and focused `git diff` reads. If the user provides a diff or summary, use that as the source of truth.
2. Infer the MR's core goal from the diff. Separate the delivered capability or fix from implementation details needed to achieve it.
3. Write for reviewers who may not know this MR's local context. Prefer plain product or engineering language over branch-specific shorthand, internal nicknames, or implementation-only labels.
4. Output only the `## Description` section. Do not include LLD links, Test, Impacted Service, TODO, self-review checklist, screenshots, or placeholders unless the user explicitly asks.
5. Return raw Markdown inside a fenced `markdown` code block and do not add extra explanation outside the block.

## Output Shape

```markdown
## Description
[One concise paragraph describing the MR goal.]

实现内容：
- [Delivered capability or bug fix.]

实现细节：
- [Required implementation detail.]
- [Required implementation detail.]

顺手完成：
- [Incidental cleanup or improvement, or `无`.]
```

## Writing Rules

- Put required implementation details under `实现细节`, not `顺手完成`.
- Use `顺手完成：
- 无` when there are no incidental changes.
- Keep bullets concrete and tied to the diff.
- Make the description easy to understand without opening the code first: explain the user-visible or operational purpose before naming implementation mechanics.
- Avoid MR-local jargon, uncommon abbreviations, branch names, commit nicknames, and narrow helper/function names unless they are necessary to explain the change.
- If a domain-specific term is necessary, define it briefly on first use or replace it with a more general phrase.
- Do not make the paragraph read like a file-by-file diff summary; group details by behavior and intent.
- Do not invent tests, rollout notes, impacted services, or TODOs in this skill's output.
