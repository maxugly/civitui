# Specification: Form Layout and Navigation Law

This document defines the layout rules, field alignment, and navigation laws for the configuration form in `civitui`.

---

## 1. Field Ordering and Two-Column Pairing (`fieldOrder`)

Form fields are laid out vertically. To optimize space, some fields are rendered side-by-side as pairs (two columns), while others occupy the full width of the form column.

The physical ordering and pairing are defined by the `fieldOrder` slice in `internal/ui/ui.go`:
- **Positive Index**: Renders as a full-width row alone.
- **Negative Index**: Renders as a paired row. The negative value represents the index of the first (left) field, and the next element in the slice is the second (right) field of the pair.

Example slice structure:
```go
var fieldOrder = []int{
	fiPrompt,           // alone (full width)
	fiNegativePrompt,   // alone
	fiModel,            // alone
	fiFluxMode,         // alone
	fiSampler,          // alone
	-fiScheduler,       // pair start (left: Scheduler)
	fiAspectRatio,      // pair end   (right: AspectRatio)
	-fiWidth,           // pair start (left: Width)
	fiHeight,           // pair end   (right: Height)
	// ...
}
```

---

## 2. Tight Form Column Alignment

To prevent large dead zones and mismatched layout widths, the form column is aligned tightly according to predefined size constants:

```go
const (
	formCursorWidth = 2  // Column for current input selection marker (e.g., "> ")
	formLabelWidth  = 16 // Padding width for the field label (fits "Negative Prompt:")
	formLabelGap    = 1  // Spaces between label and input field
	formPairGap     = 2  // Gutter spaces between left and right paired inputs
	formSoloInputW  = 48 // Content width of a single full-width text/preset input
	formPairInputW  = 10 // Content width of each text input in a pair (e.g., CFG scale, steps)
	formColGap      = 2  // Gap between the form column and the right-side help/presets pane
)
```

### Layout Grid Calculations

1. **Solo Row Width**:
   $$\text{width} = \text{formCursorWidth} + \text{formLabelWidth} + \text{formLabelGap} + \text{formSoloInputW}$$
   $$2 + 16 + 1 + 48 = 67 \text{ cells}$$

2. **Pair Half Width (One Cell)**:
   $$\text{halfWidth} = \text{formCursorWidth} + \text{formLabelWidth} + \text{formLabelGap} + \text{formPairInputW}$$
   $$2 + 16 + 1 + 10 = 29 \text{ cells}$$

3. **Combined Pair Row Width**:
   $$\text{pairWidth} = (\text{halfWidth} \times 2) + \text{formPairGap}$$
   $$(29 \times 2) + 2 = 60 \text{ cells}$$

4. **Total Left Column Width (`formColumnWidth()`)**:
   Max of Solo Row Width and Combined Pair Row Width, which is **67 cells**.

The permanent right-side help and presets pane is replaced by floating popups and a thin help line (see [specs/popup-presets.md](file:///home/m/snc/cod/civitui/specs/popup-presets.md)). The remaining panel width is temporarily left empty or unused in normal state, serving as a layout hook for a future right-side pane (such as a stacked queue and debug panel).

---

## 3. Column-Major Navigation (`navOrder`)

Tab and Arrow key navigation must follow a column-major traversal pattern, where focus travels down the left column first, then down the right column, and finally wraps around.

1. **Left Column Traversal**: Includes all full-width rows plus the left input of every paired row.
2. **Right Column Traversal**: Includes the right input of every paired row.

The `navOrder()` function dynamically computes this path from `fieldOrder`:
```go
func navOrder() []int {
	var left, right []int
	for i := 0; i < len(fieldOrder); i++ {
		idx := fieldOrder[i]
		if idx < 0 {
			left = append(left, -idx)
			i++
			if i < len(fieldOrder) {
				right = append(right, fieldOrder[i])
			}
			continue
		}
		left = append(left, idx)
	}
	out := make([]int, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}
```

### Focus Movement Guidelines

- **Tab / Down Arrow**: Advance forward (+1) along the computed `navOrder`.
- **Shift+Tab / Up Arrow**: Retreat backward (-1) along the computed `navOrder`.
- **Dynamic Exit/Limit**: Loops that advance or retreat focus must have a hard boundary (maximum steps equal to the number of fields) to prevent infinite loops when all fields are hidden.
- **Skipping Hidden Fields**: Focus must automatically skip any field where `isFieldVisible(idx)` returns `false`.

---

## 4. Dynamic Visibility and Focus Recovery

Form fields are conditionally shown or hidden based on the active model ecosystem (e.g., Flux, SDXL, SD 1.5). When fields are dynamically toggled, the application must manage focus transitions gracefully:

1. **Ecosystem-Specific Adjustments**: When a model preset is chosen or custom AIR is typed, the model's ecosystem is determined. Sampler, scheduler, and Flux-specific options are auto-adjusted to valid presets.
2. **Refocusing Hidden Inputs**: If the user changes a model and the currently focused input field gets dynamically hidden (e.g., leaving a Flux model while focused on `fiFluxUltraRaw`), the application must immediately run `refocusIfHidden()`. This helper shifts focus to the next visible field in `navOrder` and blurs the hidden field.

---

## 5. Visual Rendering and Row Highlights

- **Left Column Padding**: All rendered left-column rows are padded (or truncated) to exactly `formColumnWidth()` to keep the layout aligned.
- **Highlighting**: Selected inputs are highlighted with a sky-blue styling. If a row contains a pair, only the active half of the pair gets the selection decorator (`> ` and colored input background), while the inactive half renders normally.
