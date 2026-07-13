# pkg/civit/ — Tom's territory (headless engine)

> **You are in the core engine.** No UI. No terminal rendering. No user 
> interaction. Pure Go, pure HTTP, pure data. If you import `bubbletea` 
> here, you've broken the constitution.

## Who owns this

**Tom** (Hermes). Implements what Bones specs and Grit approves.

## What goes here

- `civit.go` — API client: NewClient, CalculatePrice, SubmitJob, PollJobStatus, DownloadImage
- `civit_test.go` — httptest-native unit tests (16 tests, ~30s)
- Data models: GenerationRequest, WorkflowResponse, Step, Output, Image, Cost

## What does NOT go here

- UI code (`bubbletea`, `lipgloss`, `huh`, `bubbles`). That's `internal/ui/`.
- `fmt.Println` user-facing output. This is a library, not an app.
- Terminal rendering. No `os.Stdout` writes.
- Keyboard handling. No `tea.KeyMsg`.

## Rules for Tom

1. Read the spec in `../specs/` BEFORE writing code.
2. All public methods must have tests.
3. JSON tags must match the CivitAI schema. Reference: `../specs/api_parameters_audit.md`.
4. Retry logic belongs in `doJSON`, not in individual methods.
5. After changes: `go build ./... && go vet ./... && go test ./... -count=1`.
6. Mark tasks complete in `../TODO.md` after commit.
7. Append to `~/.hermes/agents/chat.md` when ready for Grit's QA.

## Quality gates

```
go build ./...
go vet ./...
go test ./pkg/civit/ -v -count=1
```

All three must pass. The test file is non-negotiable — if you add a public
method, you add a test.
