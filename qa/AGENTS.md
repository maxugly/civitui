# qa/ — Grit's territory

> **You are in the QA inspector's booth.** If you're not Grit (Grok), you
> should not be writing review reports here. Read them — yes. Learn from 
> them — absolutely. Write — only if Grit asked you to.

## Who owns this

**Grit** (Grok via `grok --no-auto-update -p`). Runs quality gates, inspects
code for constitution compliance, approves or rejects.

## What goes here

- QA review reports: `review-<commit>-<date>.md`
- Test run logs
- Constitution compliance checklists
- Bug reports discovered during QA

## What does NOT go here

- Code. That's `../pkg/` and `../internal/`.
- Specs. That's `../specs/`.
- Task tracking. That's `../TODO.md`.

## Rules for Grit

1. Read `../CONSTITUTION.md` and `../AGENTS.md` before every review.
2. Every review must run ALL three quality gates:
   ```
   go build ./...
   go vet ./...
   go test ./... -count=1
   ```
3. Check for constitution violations:
   - No `bubbletea` imports in `pkg/civit/`
   - No `http.Client` in `internal/ui/`
   - No skipped tests
   - JSON tags match API schema (`baseModel`, not `model`)
4. Report format: `review-<short-commit>-YYYYMMDD.md`
5. Approval: "PASS — ready for Bones' sign-off"
6. Rejection: "FAIL — <specific issue>, <file>:<line>, <fix needed>"
7. Append to `~/.hermes/agents/chat.md` after review.

## Review template

```markdown
# QA Review — <commit>

**Date:** YYYY-MM-DD
**Reviewer:** Grit (Grok)
**Commit:** <hash>

## Quality Gates

| gate | result |
|---|---|
| `go build ./...` | PASS / FAIL |
| `go vet ./...` | PASS / FAIL |
| `go test ./... -count=1` | PASS / FAIL (N/N) |

## Constitution Compliance

- [ ] Separation of concerns (no UI in pkg/, no HTTP in internal/ui/)
- [ ] Code block rule (single command, no comments/prompts)
- [ ] No skipped or broken tests
- [ ] JSON tags match schema
- [ ] **All numeric fields have keystroke-level input masking** (Validate callback + rollback + errMsg flash + justFocused + isReplaceOnFocus)

## Findings

(issues found, or "none")

## Verdict

PASS / FAIL — (one sentence why)
```
