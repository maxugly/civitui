# Mockup3 Gap Analysis — civitui

**Date:** 2026-07-13
**Reviewer:** Grit (Grok)
**Mockup:** `sketches/mockup3.png` (781×544, Max's vision)
**Implementation:** `internal/ui/ui.go` (2584 lines, Bubble Tea)

---

## Summary

The mockup depicts a **multi-panel dashboard TUI** with three persistent
columns (left navigation sidebar, center content/previews, right image
library) and 9+ navigable sections. The current implementation is a
**single-column lazygit-style layout** with a config form, inline job queue,
and floating preset popups. The gap between vision and reality is large —
the mockup represents an architectural leap from a form-based tool to a
full dashboard application.

---

## 1. Structural / Architectural Gaps

### 1.1 Multi-Column Layout

| Aspect | Mockup | Code |
|--------|--------|------|
| Panel arrangement | 3 columns side-by-side: left nav, center, right library | Single column, panels stacked vertically |
| Left sidebar | Persistent ~20-char nav column (dark brown #5E3206) | No sidebar — no navigation at all |
| Center area | "Input" tab + "Thumbs/Previews" area (sky blue #6DB2FF) | Config form + queue + debug stacked vertically |
| Right panel | "Images Library" (dark teal #103C3F) | No right panel |
| Bottom panels | "Help" and "Jobs" sections visible | Help line inline in config pane; Jobs as queue panel |

### 1.2 Tab/Section Navigation

The mockup shows **9 distinct navigation sections** in the left sidebar,
implying a tab-based UI where each section swaps the center content:

1. **Input** — config form (currently selected in mockup, highlighted)
2. **Popups** — "menus for models and other settings"
3. **Saved Prompts** — "both negative and positive"
4. **Saved Profiles**
5. **Settings**
6. **Logs**
7. **Detailed Help/Keybinds**
8. **Model Info**
9. **Finder** — "find models, find loras, get metadata for models and loras"

The current code has **zero navigation tabs**. All functionality is
crammed into a single config form page. Switching "modes" (config, queue
view, debug) is done implicitly via panel visibility, not tab selection.

**Gap:** The code needs a left sidebar widget with at least 9 navigable
tabs and content area swapping.

### 1.3 Right "Images Library" Panel

The mockup shows a persistent right panel titled "Images Library" listing:
```
1 image
2 image
3 image (...garbled OCR...)
4 (...garbled...)
5 (...garbled...)
etc...
```

The current implementation shows download paths inline in the queue panel
(`viewQueue` writes `job.dlPaths[0]` as a status line). There is **no
image browsing, no library, no persistent list of generated images**.

**Gap:** Image library panel with scrollable list of generated/saved
images, separate from the job queue.

### 1.4 "Thumbs/Previews" Center Area

The mockup's center area (when "Input" tab is active) shows the label
"Thumbs/Previews" alongside the config form, suggesting inline image
preview thumbnails during generation. The current code has **no image
preview or thumbnail rendering** — images are downloaded as files and
their paths are displayed as text.

**Gap:** Inline image preview/thumbnail area within the center content
pane.

---

## 2. Missing Features

| # | Feature | In Mockup? | In Code? |
|---|---------|-----------|----------|
| 1 | Saved prompts (positive + negative) | ✅ Tab in sidebar | ❌ |
| 2 | Saved profiles/configurations | ✅ Tab in sidebar | ❌ |
| 3 | Settings page | ✅ Tab in sidebar | ❌ |
| 4 | Logs viewer page | ✅ Tab in sidebar | Partial — debug panel inline, not a full page |
| 5 | Detailed help/keybinds page | ✅ Tab in sidebar | Partial — only a 1-line footer (`ctrl+c quit • tab/↑↓ navigate • enter generate`) |
| 6 | Model info display | ✅ Tab in sidebar | ❌ |
| 7 | Model/LoRA finder | ✅ Tab in sidebar | ❌ |
| 8 | Images library browser | ✅ Right panel | ❌ |
| 9 | Image thumbnails/previews | ✅ Center area | ❌ |
| 10 | Popups as navigable section | ✅ Tab in sidebar | Partial — popups exist but are not a "page"; they're overlays |

---

## 3. Visual / Styling Differences

### 3.1 Color Scheme

| Element | Mockup Colors | Code Colors |
|---------|--------------|-------------|
| Background | `#121212` (near-black) | Terminal default (likely black) |
| Left sidebar | `#5E3206` (dark brown) | N/A |
| Center content area | `#6DB2FF` (sky blue) | N/A |
| Right panel | `#103C3F` (dark teal) | N/A |
| Accent colors | Pink (`#FFADCD`), magenta (`#FF90F3`, `#FFA7FF`), greens (`#43BA43`, `#718769`) | Bold green (success), bold red (error), bold yellow (loading/cursor), sky blue (selection) |
| Text | White (`#FFFFFF`) | White/dim white/yellow/cyan |
| Selection highlight | Sky blue background | Sky blue background (`Color("25")`, `selectedStyle`) |

**Gap:** The mockup uses a **color-coded multi-panel scheme** where each
panel has a distinct background color. The code uses terminal ANSI colors
on the default background. The left sidebar, center content, and right
panel each have a different background color in the mockup — the code has
no panel background coloring.

### 3.2 Active Tab Highlighting

The mockup shows "Input" as the **currently selected/active tab** in the
left sidebar, visually distinguished from the other (inactive) nav items.
The code has no tab navigation, so no active tab highlighting exists.

### 3.3 Panel Borders

The mockup uses distinct bordered panels with section separation. The
current code uses lazygit-style `innerPanel` with `┌── ──┐` / `│...│` /
`└──┘` box-drawing characters and titled top borders. The mockup's
precise border style is not fully discernible from OCR/chafa, but the
multi-panel arrangement with color-coded backgrounds is clearly different
from the single-column bordered box layout.

### 3.4 "Input" Main Content

When "Input" is the active tab, the mockup's center area shows:
- "Input" as a label (possibly a tab indicator)
- "Thumbs/Previews" area below/beside it

The code currently shows the form fields directly inside a "Configure
generation" panel with a help line below. There's no tab label, no
preview area, and the form is wrapped in a full-width bordered panel.

---

## 4. Form-Specific Differences

### 4.1 Form Panel Title

| Mockup | Code |
|--------|------|
| "Input" (short tab label) | "── Configure generation ──" (full panel title) |

### 4.2 Helper Text / Popups

The mockup has "Popups" as a **separate tab in the sidebar**, described
as "menus for models and other settings." In the current code, popups are
**inline floating overlays** triggered by pressing → or Enter on a
presets field. The mockup suggests popups might be a browsable reference
page rather than (or in addition to) interactive overlays.

### 4.3 Footer / Status Bar

| Mockup | Code |
|--------|------|
| Not clearly visible in OCR — likely absent or in a different position | `ctrl+c quit • tab/↑↓ navigate • enter generate` (dim white, single line) |

The mockup may have the footer omitted or moved to a status bar within
one of the panels.

---

## 5. What Matches (for completeness)

These elements from the code **are consistent with** or **not contradicted by** the mockup:

- **Floating preset popups**: The mockup lists "Popups" as a menu concept; the code's implementation via `viewPresetsPopup()` + `overlayLine()` is a valid interpretation
- **Job queue**: "Jobs" is referenced in the mockup (position unclear from OCR), and the code has `viewQueue()`
- **Debug/logs**: "logs" is a sidebar tab in the mockup; the code has `debugLog` ring buffer shown in a separate panel
- **Config form fields**: The mockup doesn't detail individual form fields, so no field-level gaps can be identified
- **Form column alignment**: The code's tight column alignment (67 cells) with fixed labels is implementation detail not visible in the mockup
- **Column-major navigation**: Implementation detail not visible in mockup
- **Input masking**: Implementation detail not visible in mockup

---

## 6. Prioritized Gap Summary

### Blockers (architectural — need before anything else fits)
1. **No multi-column layout** — must implement left sidebar + center + right panel
2. **No tab-based navigation** — must implement 9-section sidebar with content swapping
3. **No Images Library panel** — right panel with scrollable image list

### Major Features (missing entirely)
4. Saved prompts (positive + negative)
5. Saved profiles
6. Settings page
7. Model/LoRA finder
8. Model info page
9. Detailed help/keybinds page
10. Image thumbnails/previews

### Visual Polish (style mismatches)
11. Panel background colors (brown sidebar, blue center, teal right)
12. Active tab highlighting in sidebar
13. Multi-panel border arrangement vs single-column boxes
14. "Input" tab label vs "Configure generation" panel title

### Execution Notes
- The mockup file is `sketches/mockup3.png` — there is no corresponding
  spec in `specs/` for this multi-panel dashboard architecture
- The popup-presets spec (`specs/popup-presets.md`) and form-layout spec
  (`specs/form-layout.md`) cover only the current single-column form +
  popup overlay design
- Before implementing any of these gaps, Bones should write an
  architectural spec defining the dashboard layout, panel sizing,
  navigation model, and color scheme
