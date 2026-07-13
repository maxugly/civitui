# Specification: Floating Preset Popups & Single-Line Help

This document specs the replacement of the permanent right-side presets/help pane with **lazygit-style floating preset popups** and a **single-line contextual help bar**. 

---

## 1. Rationale and Design Strategy

1. **Reclaim Horizontal Space**: The permanent right-pane split-pane layout steals critical screen width, forcing the form to squeeze and causing wrap-around clutter.
2. **Modern UX**: Floating centered popups match CLI standards (like `lazygit` branch/preset menus), focusing the user's attention on selection when needed.
3. **Future Right-Side Column (Design Hook)**: Removing the permanent right pane reduces the horizontal footprint of the form to a constant width (~67 cells, per [specs/form-layout.md](file:///home/m/snc/cod/civitui/specs/form-layout.md)). This paves the way for a future phase where the main form occupies the left, and a stacked panel containing the queue + debug log occupies the right:
   ```
   ┌──────────────────────────────────────────────────────────┐
   │ [Form Panel (Width 67)]  │ [Queue + Debug (Remaining)]  │
   │                          │                              │
   └──────────────────────────────────────────────────────────┘
   ```
   *Note: This right-side panel is a design hook only; no right-pane stack is implemented in this phase.*

---

## 2. Layout Wireframes

### 2a. Normal State (Popup Closed, Help as Thin Line)

The configuration form renders as a single, tight column. At the bottom of the config pane, a single dim line displays the active field's help text.

```
┌─ civitui ──────────────────────────────────────────────────────────────────────┐
│                                                                                │
│  Configure generation:                                                         │
│                                                                                │
│  ▶ Prompt:         [car_____________________________]                          │
│    Negative Prompt: [optional - what to avoid________]                         │
│    Model:           [air:flux1:checkpoint:civitai...]                          │
│    Flux Mode:       [urn:air:flux1:checkpoint:civ...]                          │
│    Scheduler:       [EulerA_________________________]                          │
│                                                                                │
│  ℹ Help: Model reference (AIR) or custom hash. Press → to browse presets.     │
│                                                                                │
│  ── Queue ──────────────────────────────────────────────────────────────────── │
│  ⣾  "car"                                     30ⓑ  submitting...               │
│                                                                                │
│  ctrl+c quit  •  tab/↑↓ navigate  •  enter generate                            │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 2b. Popup Open State (Floating Centered Overlay)

When the user activates a preset field, a bordered popup window is drawn centered over the base UI. The background UI remains visible behind it.

```
┌─ civitui ──────────────────────────────────────────────────────────────────────┐
│                                                                                │
│  Configure generation:                                                         │
│                                                                                │
│  ▶ Prompt:         [car_____________________________]                          │
│    ┌── Model Presets ──────────────────────────────┐                           │
│    │  ▶ Flux.1 Dev                                 │                           │
│    │      FLUX.1-dev model checkpoint              │                           │
│    │    Pony V6                                    │                           │
│    │      Pony Diffusion V6 XL                     │                           │
│    │    SDXL 1.0                                   │                           │
│    │      Stable Diffusion XL 1.0                  │                           │
│    │                                               │                           │
│    │  ↑↓ select  •  enter apply  •  ←/esc back     │                           │
│    └───────────────────────────────────────────────┘                           │
│  ── Queue ──────────────────────────────────────────────────────────────────── │
│  ⣾  "car"                                     30ⓑ  submitting...               │
│                                                                                │
│  ctrl+c quit  •  tab/↑↓ navigate  •  enter generate                            │
└────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Popup Overlay Rendering Law

To render the popup, the application compiles the base UI view string, splits it into lines, and overlays the popup box lines on top of it.

### 3a. Dimensions and Centering Math

- **Popup Width**: Fixed at **48 cells** (including borders).
- **Popup Height**: Determined dynamically based on the preset list length:
  $$\text{height} = \text{number of items} \times 2 \text{ (name + desc)} + 4 \text{ (borders + title + footer)}$$
  Capped at a maximum of **15 lines** (or $\text{terminalHeight} - 6$).
- **Positioning**: Center-aligned both horizontally and vertically relative to the terminal dimensions.
  - Let $W_{\text{term}}$ and $H_{\text{term}}$ be the terminal width and height.
  - Horizontal start position: $X_{\text{start}} = (W_{\text{term}} - W_{\text{popup}}) / 2$
  - Vertical start position: $Y_{\text{start}} = (H_{\text{term}} - H_{\text{popup}}) / 2$

### 3b. ANSI-Safe Overlay Helper

Because strings in Bubble Tea contain escape sequences for colors (e.g., `\x1b[38;5;12m`), simple byte-slicing will corrupt the terminal state. 
Tom must implement a helper that overlays lines using character-by-character visual width tracking, preserving ANSI sequences:

```go
// overlayLine replaces a portion of the base line with the overlay line,
// starting at visual character index startX, ensuring ANSI codes are preserved.
func overlayLine(base, overlay string, startX int) string {
	baseRunes := []rune(base)
	overlayWidth := displayWidth(overlay) // visual length of overlay
	
	// We reconstruct the line by iterating through visual characters.
	var result strings.Builder
	
	inEscape := false
	visualCol := 0
	
	// Write base characters before startX, copying escape codes verbatim.
	for i := 0; i < len(baseRunes); i++ {
		r := baseRunes[i]
		if inEscape {
			result.WriteRune(r)
			if r == 'm' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		
		// If we've reached the overlay start position, inject the overlay string
		// and skip the corresponding visual columns of the base string.
		if visualCol == startX {
			result.WriteString(overlay)
			visualCol += overlayWidth
			// Skip base characters that were overwritten
			for visualCol > startX && i < len(baseRunes) {
				// We must parse ahead in the base to skip only visual characters
				i++
				if i >= len(baseRunes) {
					break
				}
				r2 := baseRunes[i]
				if r2 == '\x1b' {
					// Don't skip ANSI codes! Preserve them.
					result.WriteRune(r2)
					for i+1 < len(baseRunes) {
						i++
						r3 := baseRunes[i]
						result.WriteRune(r3)
						if r3 == 'm' || (r3 >= 'A' && r3 <= 'Z') || (r3 >= 'a' && r3 <= 'z') {
							break
						}
					}
				} else {
					startX++ // Shift visual skip window
				}
			}
			i-- // Adjust loop increment
			continue
		}
		
		result.WriteRune(r)
		visualCol++
	}
	return result.String()
}
```

---

## 4. Key Mapping & Focus Routing

The existing state machine variables (`inPresetsPane` and `activePreset`) are reused.

### 4a. While Popup is Open (`inPresetsPane == true`)

- **Up/Down / k/j**: Increment/decrement `activePreset` within the current preset list limits (with wraparound).
- **Enter**: Apply the selected preset (copy name/value/AIR to the text input model), set `inPresetsPane = false`, and call `m.inputs[m.activeInput].Focus()`.
- **Left / Esc / Tab**: Close the popup without applying changes, set `inPresetsPane = false`, and restore focus to the input field.
- **Other Inputs**: Completely blocked (no keystrokes pass to underlying text input fields).

### 4b. While Popup is Closed (`inPresetsPane == false`)

- **Right Arrow (→) or Enter** on any field where `isPresetsField(activeInput)` is `true`: Opens the preset popup for that field (`inPresetsPane = true`, `activePreset` set to matching active value index or `0`).

---

## 5. Help Line Specification

The permanent right-side help column is removed. Instead:
- Under the last form row, render one blank line.
- Render a thin single line:
  `  ℹ Help: <Active Field Help Text>` (cyan or gray styling)
- When the active field is a preset field, append: ` Press → to browse presets.`
- Line wrapping: Truncate or wrap to the width of the terminal minus margins.

---

## 6. Non-Goals for Version 1

- No incremental search/filtering of presets inside the popup.
- No scrollbar rendering inside the popup (lists are short and will fit the capped height).
- No mouse integration.

---

## 7. Tom Task Breakdown (Implementation Plan)

### Step 1: Remove Old Split-Pane Rendering
- Modify `viewConfig` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go) to delete all right-column preset rendering, preset headers, and combined side-by-side assembly loops.
- Configure form inputs to stretch/render using the standard `formColumnWidth()` width. Do not stretch inputs to the edge of the terminal.

### Step 2: Implement Thin Help Line
- In `viewConfig`, render a single help line at the bottom of the config pane.
- Retrieve description text from the `fieldHelpText` slice using the current `activeInput` index.

### Step 3: Implement Centered Popup Render
- Write a helper `viewPresetsPopup() string` that compiles a bordered panel with the current active field's presets, highlighting the row at `activePreset`.
- At the bottom of the popup, render a short instruction guide: `↑↓ select  •  enter apply  •  ←/esc back`.

### Step 4: Integrate the Overlay Logic
- In the main `View()` function, compile the full base view (Header + Config Panel + Queue + Footer).
- If `inPresetsPane` is true:
  1. Generate the popup string from `viewPresetsPopup()`.
  2. Compute vertical/horizontal centered coordinates.
  3. Split base view and popup into lines.
  4. Overlay the popup lines on top of the base view lines using the ANSI-safe `overlayLine` helper.
  5. Join the lines back with newlines.

### Step 5: Verify Transitions and Quality Gates
- Verify keyboard behavior: popup opens, blocks input, applies on enter, exits on escape/arrow left.
- Re-run all quality gates:
  ```bash
  go build ./...
  go vet ./...
  go test ./... -count=1
  go test -tags=integration -run Integration ./pkg/civit/ -v
  ```
