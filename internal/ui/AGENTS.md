# internal/ui/ — Tom's territory (Bubble Tea TUI)

> **You are in the UI layer.** No HTTP. No JSON marshalling. No retry 
> loops. You consume `pkg/civit` — you don't talk to the internet.

## Who owns this

**Tom** (Hermes). Implements the terminal interface that Max actually sees.

## What goes here

- `ui.go` — Bubble Tea state machine, 7-phase pipeline, 14 form fields, 6 preset selectors
- View rendering, key event handling, input masking, validation
- Async commands wrapping `pkg/civit.Client` methods

## What does NOT go here

- HTTP calls. No `http.Client`, no `http.NewRequest`. That's `pkg/civit/`.
- JSON serialization of API payloads. Use `pkg/civit.GenerationRequest`.
- Retry loops. The engine handles that.
- API key management. The engine owns auth.

## Rules for Tom

1. Read the spec in `../specs/` AND `pkg/civit/` docs BEFORE touching this file.
2. All API calls go through `pkg/civit.Client`. Never bypass it.
3. **EVERY numeric field MUST have keystroke-level input masking.** This is non-negotiable.
4. New form fields: add `fi*` constant, increment `numFormFields`, add input + validate + label + help.
5. Input masking: validate at keystroke level, flash `errMsg` on rejection (invisible masking is broken).
6. Presets-only fields: block free-form typing, right arrow to browse, enter to apply.
7. After changes: `go build ./...` must pass.
8. Mark tasks complete in `../TODO.md`.

### Input masking checklist (every numeric field)

When adding a new numeric field, ALL of these must be done:

- [ ] `inputs[fiXxx].Validate` — regex gate + range check
- [ ] `handleConfigKey` rollback block — capture oldValue/oldPos, restore on Err
- [ ] `m.errMsg = input.Err.Error()` — USER MUST SEE why their keystroke was rejected
- [ ] `justFocused` gate — first keystroke clears old value for numeric fields
- [ ] `isReplaceOnFocus` returns true for this index (if numeric)

If any of these are missing, the field accepts junk silently. The user will
notice and you will look incompetent. This happened before. Don't let it
happen again.

## UI architecture (don't break this)

```
7-phase state machine:
  config → pricing → confirm → submitting → polling → downloading → done

Async pattern:
  tea.Cmd → typed message struct → Update() switch → next phase

Split pane:
  left: config form (14 fields)
  right: presets browser (models, samplers, schedulers, aspect ratios, etc.)
```

## Input masking pattern

```go
// capture old state
oldValue := input.Value()
oldPos := input.Position()
// let bubbletea process the keystroke
input, _ = input.Update(msg)
// if validation failed, roll back AND show error
if input.Err != nil {
    input.SetValue(oldValue)
    input.SetCursor(oldPos)
    input.Err = nil
    m.errMsg = "reason it was rejected"  // USER MUST SEE THIS
}
```
