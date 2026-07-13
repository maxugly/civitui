# CONSTITUTION.md — civitui project

This document defines the immutable engineering laws and workflow rules for the civitui project. All agents, human or otherwise, are bound by these principles.

---

## 1. Immutable Engineering Laws

### I. Strict Separation of Concerns

The project has exactly two layers. They must never bleed into each other.

| layer | directory | responsibility | must NOT contain |
|---|---|---|---|
| **Core Engine** | `pkg/civit/` | Headless API client. The 5-step transaction chain: `NewClient` → `CalculatePrice` → `SubmitJob` → `PollJobStatus` → `DownloadImage`. JSON serialization, HTTP retry logic, context propagation. | No UI imports. No `bubbletea`, `lipgloss`, `huh`, or any terminal rendering library. No `fmt.Println` user-facing output. No keyboard handling. |
| **UI Layer** | `internal/ui/` | Bubble Tea state machine. Form input, spinner animations, canvas/viewport rendering, key event handling. Consumes `pkg/civit` as a dependency. | No direct HTTP calls. No `http.Client` construction. No JSON marshalling of API payloads. No retry loops — those are the engine's job. |

**Only violation allowed**: `manage_imports.go` at the project root, which exists solely to pin Go toolchain dependencies. It imports both layers but contains no logic.

### II. Code Block Rule

Any markdown code block containing terminal commands must:
1. Contain only the executable command — no comments, no `#` annotations, no shell prompts (`$`, `>`)
2. Be limited to a single command per block

```bash
go build ./...
```

Not:
```bash
# Build the project
$ go build ./...  # ensure clean compilation
```

This rule exists for instant copy-paste usability. Implementation agents must not waste time stripping decorations.

### III. Environment Policy

- Dependencies must be minimal, self-contained, and high-performance
- No systemd units. No heavy system frameworks.
- Go stdlib preferred over external packages where practical
- External dependencies require explicit justification
- The project must build on a fresh Go toolchain with zero system-level setup beyond `go mod download`

---

## 2. File-Driven Workflow

### Roles

| role | agent | responsibility |
|---|---|---|
| **Architect** | Bones (via Antigravity) | Drafts architectural plans, task breakdowns, and specifications into local markdown files (`TODO.md`, `specs/`). Performs final sign-off. |
| **Implementer** | Tom/Hermes | Reads architect-written plans, executes code modifications, writes tests, and marks task status as pending review. |
| **QA / Reviewer** | Grok (via Grok CLI) | Runs quality gates (`go build`, `go vet`, `go test`), inspects code for rule compliance, and either approves or requests changes. |
| **Operator** | Max | Human oversight. Final authority on design decisions, acceptance criteria, and deployment. |

### Workflow Cycle

```
User (Max) triggers Architect (Bones)
    → Bones drafts plan in TODO.md / specs/
        → Implementer (Hermes) executes code changes
            → QA/Reviewer (Grok) runs tests & verifies compliance
                → If Fail: Hermes fixes code (loop back)
                → If Pass: Architect (Bones) does final checkoff
```

### Status Markers

Implementation agents append status to task files upon completion:

```markdown
- [x] Task description — completed in commit `abc1234`
```


Never delete a task file without explicit instruction. Mark it done, don't erase it.

---

## 3. Quality Gates

Before any commit is considered complete:

```bash
go build ./...
```

```bash
go vet ./...
```

```bash
go test ./... -count=1
```

All three must return zero errors. No warnings tolerated. No skipped tests.

---

## 4. Communication

Inter-agent communication happens via `~/.hermes/agents/chat.md`.

Format: append-only, timestamped headers. No schema enforcement. Direct, honest, no corporate sugar.

---

*Ratified 2026-07-12 by Tom, on behalf of Bones and Max.*
