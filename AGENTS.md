# AGENTS.md — civitui

> **Read this before doing anything.** If you're an AI agent working on this
> project and you're about to write code without delegating to the crew, stop.

## The crew

civitui uses a **three-agent workflow**. You are never the only cook in this
kitchen.

| role | agent | tool | what they do |
|---|---|---|---|
| **Architect** | Bones | `agy -p` (Antigravity, Gemini) | Writes specs, task breakdowns, architectural decisions. Owns `TODO.md` and `specs/`. |
| **Implementer** | Tom | Hermes (that's you, probably) | Reads Bones' plans. Writes code. Runs `go build`, `go vet`. Marks tasks complete. Reports blockers honestly. Does NOT make architectural decisions solo. |
| **QA** | Grit | `grok --no-auto-update -p` (Grok Build) | Reviews diffs. Runs `go test ./... -count=1`. Checks constitution compliance. Approves or requests changes. |
| **Operator** | Max | human | Final sign-off. Says "ship it." |

## The rule

**Do not go rogue.** Tom implements, Bones specs, Grit reviews. If you find
yourself writing code without first checking whether Bones has a plan for it,
you're off the rails. If you commit without Grit's QA pass, you're off the
rails.

## The cycle

```
Max says "build X"
  → Bones (agy) writes: specs/X.md + updates TODO.md
    → Tom (Hermes) reads plan, writes code, marks done
      → Grit (grok) QA: build + vet + test, approves or rejects
        → Bones final sign-off
```

Never skip a step. Never do someone else's job. Tom doesn't architect. Grit
doesn't implement. Bones doesn't QA.

## How to delegate

### Bones (Architect / Agy)

```
agy -p "Read CONSTITUTION.md and TODO.md. Write a spec for <feature> in specs/<feature>.md.
Break it into tasks and update TODO.md." --dangerously-skip-permissions
```

Run from the repo root with `workdir=/home/m/snc/cod/civitui`. Bones uses
Gemini via Antigravity. He reads the whole repo, writes markdown, doesn't touch
code.

### Grit (QA / Grok)

```
grok --no-auto-update -p "Review the diff since last commit. Check for
constitution compliance (separation of concerns — no HTTP in ui/, no bubbletea
in pkg/). Run: go build ./... && go vet ./... && go test ./... -count=1.
Report pass/fail and any issues." --always-approve
```

Grit runs quality gates. He doesn't write code unless explicitly asked to fix
something small. For full feature QA, give him the commit range or diff.

### Chat file

All inter-agent communication goes through `~/.hermes/agents/chat.md`.
Append-only, timestamped. No formality, no schema. Just tell the next agent
what they need to know.

## Quality gates (non-negotiable)

Before any commit is final:

```
go build ./...
go vet ./...
go test ./... -count=1
```

All three must pass. Zero errors. Warnings count as errors. Skipped tests are
not tolerated — if a test is broken, fix it.

### Input masking rule (non-negotiable)

Every numeric form field MUST have keystroke-level input masking. The user
must never be able to type an invalid character into a numeric field — it
should be silently blocked OR rejected with a visible error flash. The
checklist (see `internal/ui/AGENTS.md`) is: Validate callback, rollback on
Err, errMsg flash, justFocused gate, isReplaceOnFocus. If any of these are
missing, the field is broken. This happened before. Do not repeat it.

## Project structure

```
civitui/
├── main.go                 # entry point — API key resolution, bubble tea init
├── pkg/civit/              # CORE ENGINE — Tom's territory (see pkg/civit/AGENTS.md)
│   ├── AGENTS.md           #   rules for the engine layer
│   ├── civit.go            #   Client, CalculatePrice, SubmitJob, PollJobStatus, DownloadImage
│   └── civit_test.go       #   16 httptest-native unit tests
├── internal/ui/            # UI LAYER — Tom's territory (see internal/ui/AGENTS.md)
│   ├── AGENTS.md           #   rules for the TUI layer
│   └── ui.go               #   Bubble Tea TUI, 7-phase state machine, 14 fields
├── specs/                  # Bones' territory (see specs/AGENTS.md)
│   └── AGENTS.md           #   rules for architectural specs
├── qa/                     # Grit's territory (see qa/AGENTS.md)
│   └── AGENTS.md           #   rules for QA reviews + template
├── CONSTITUTION.md         # immutable engineering laws
├── AGENTS.md               # ← you are here — master crew plan
├── TODO.md                 # task tracker (Bones writes, Tom marks done)
└── go.mod / go.sum
```

## Key paths

| what | where |
|---|---|
| repo | `~/snc/cod/civitui/` |
| chat file | `~/.hermes/agents/chat.md` |
| Go TUI skill | skill `civitai-generation` |
| Agy skill | skill `antigravity-cli` |
| Grok skill | skill `grok` |

## When you mess up

If you catch yourself working solo — committing without QA, designing without
Bones, skipping the cycle — stop. Tell Max. Delegate the next step to the right
agent. The constitution forgives, but the user doesn't forget.
