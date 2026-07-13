# Architectural Decision: Input Field Strategy

This document outlines the choice of input handling libraries for the `CONFIG` phase parameters in `civitui`.

## 1. Options Compared

| Strategy | Pros | Cons | Decision |
|---|---|---|---|
| **Raw Key Appending** | Zero external dependencies. Complete layout control. | No cursor movement, home/end navigation, select-to-delete, or edit-in-place. Custom validation is tedious. | **Rejected** (Current implementation; lacks essential text editor ergonomics). |
| **Charm `huh` Forms** | Automated layouts, built-in validation, excellent out-of-the-box styling. | Takes over the full terminal display rendering lifecycle. Highly structured, making it hard to render side-by-side with a progressive image canvas panel on the right. | **Rejected** (Impedes custom split-pane layouts). |
| **Charm `bubbles/textinput`** | Ergonomic text editing (home, end, arrows, backspace). Lightweight. Can be rendered directly within any custom Lipgloss layout (like side-by-side columns). | Requires manual focus-routing logic in `Update` and `Init`. | **Selected** (Best balance of text editing capabilities and layout flexibility). |

---

## 2. Implementation Specification

### I. Model Updates
We will structure our form inputs using standard `github.com/charmbracelet/bubbles/textinput` models.

Update the `Model` struct in `internal/ui/ui.go` to hold an indexable list of inputs:

```go
import "github.com/charmbracelet/bubbles/textinput"

type Model struct {
    client     *civit.Client
    phase      Phase
    inputs     []textinput.Model
    activeInput int // 0 to len(inputs)-1
    // ... pricing, polling, results states
}
```

### II. Fields Mapping
We will initialize the inputs array with 9 fields in this index order:
1. `Prompt` (Text input, focused by default)
2. `Negative Prompt` (Text input)
3. `Model AIR` (Text input)
4. `Width` (Numeric text input)
5. `Height` (Numeric text input)
6. `Steps` (Numeric text input)
7. `CFG Scale` (Numeric text input)
8. `Quantity` (Numeric text input)
9. `Seed` (Numeric text input)

### III. Event Routing in `Update`
When in `PhaseConfig`:
* `tab` / `down`: Blur active input, increment `activeInput`, Focus new input.
* `shift+tab` / `up`: Blur active input, decrement `activeInput`, Focus new input.
* `enter`: Read text values, validate them, transition to `PhasePricing`.
* Any other keystroke: Pass `tea.Msg` to `inputs[activeInput].Update(msg)` to handle typing/cursor movement.

### IV. Rendering in `View`
In `viewConfig`:
Render the fields as list items:
```go
for i, input := range m.inputs {
    if i == m.activeInput {
        // Render with cursor style and active field styling
    } else {
        // Render normal field styling
    }
}
```
