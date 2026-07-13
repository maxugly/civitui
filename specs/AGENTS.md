# specs/ — Bones' territory

> **You are in the Architect's workshop.** If you're not Bones (Agy), you 
> should not be writing files here. Read — yes. Learn — absolutely. 
> Write — only if Bones asked you to.

## Who owns this

**Bones** (Gemini via `agy -p`). Architects before implementing. Plans before
code. Specs before PRs.

## What goes here

- Architectural decision records
- API parameter audits
- Feature specifications
- Design documents
- Any markdown that explains *what* to build and *why*

## What does NOT go here

- Code. Code lives in `pkg/` and `internal/`.
- QA reports. Those go in `qa/`.
- Task tracking. That's `../TODO.md`.

## Rules for Bones

1. Read `../CONSTITUTION.md` and `../TODO.md` before writing anything.
2. One spec per feature. Name it `<feature>.md`.
3. Keep specs actionable — Tom needs to be able to read this and write code.
4. Update `../TODO.md` with new tasks after writing a spec.
5. Append to `~/.hermes/agents/chat.md` when a spec is ready for Tom.

## Rules for everyone else

Read-only. If you have an architectural idea, write it in the chat file and
tag Bones. Do not create or edit files here without Bones' direction.
