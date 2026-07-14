# Specification: Multi-Panel Dashboard Layout

This specification defines the final structural layout and interaction model to transition `civitui` from a single-column stacked layout to a multi-panel dashboard. It is designed to match the corrected 4-row mockup from Max, maintaining the existing terminal ANSI color scheme (white text, dim borders, sky-blue selections) with no vertical navigation sidebar, no tabs, and no `FocusPanel` enum.

---

## 1. Visual Wireframe

The dashboard layout is organized into **five rows**, with a thin full-width help line between the form and the queue:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Configure Generation (Form)            │ Image Thumbnails / Preview          │
│ ▶ Prompt: [a cat in a hat___________]  │ ┌──────────┐ ┌──────────┐           │
│   Model:  [air:flux1:schnell________]  │ │  #001    │ │  #002    │           │
│   Width:  [1024]    Height: [1024]     │ │  FLUX.D  │ │  FLUX.D  │           │
│   ...                                  │ │  1024²   │ │  1024²   │           │
│ ℹ Help: Prompt text for generation. [Press → to browse presets.]              │
├──────────────────────────────────────────────────────────────────────────────┤
│ Job Queue                                                                    │
│ ⣾ "a cat in..."  30ⓑ  polling (25%)   ✓ "a dog in..."  30ⓑ  done              │
├────────────────────────────────────────┬─────────────────────────────────────┤
│ Output Image / Opened Image            │ Past Images List (History)          │
│ ┌────────────────────────────────────┐ │ [001] civitai_output_12345678_1.png │
│ │                                    │ │ [002] civitai_output_87654321_1.png │
│ │          (ASCII Art or             │ │                                     │
│ │          Metadata View)            │ │                                     │
│ │                                    │ │                                     │
│ └────────────────────────────────────┘ │                                     │
├────────────────────────────────────────┴─────────────────────────────────────┤
│ Debug Log Output                                                             │
│ [18:22:10] [API] Submitting job...                                           │
│ ctrl+c quit  •  tab cycle panels  •  enter edit/generate                     │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Grid & Layout Math

To maintain alignment and responsiveness, the layout uses precise horizontal splits and cell budget math inside a single-line or double-line outer border.

### 2.1 Width Budget & Two-Column Rows
* **Total Width**: `termWidth`
* **Outer Borders**: 2 cells (left and right edges)
* **Vertical Divider**: 1 cell (`│`)
* **Inner Width**: `termWidth - 2`
* **Left Column (Row 1 Form & Row 3 Output)**:
  - The configuration form requires exactly 67 cells of horizontal space to prevent label/input wrapping:
    $$\text{formColumnWidth}() = 67 \text{ cells}$$
  - The left column width is dynamically calculated to scale on wider terminals:
    $$\text{LeftColWidth} = \max(67, \lfloor 0.60 \times (\text{termWidth} - 2) \rfloor)$$
* **Right Column (Row 1 Thumbnails & Row 3 History)**:
  - The right column occupies the remaining space:
    $$\text{RightColWidth} = (\text{termWidth} - 2) - \text{LeftColWidth} - 1$$
  - The right column displays the thumbnail preview grid and the history list.

### 2.2 Height Budget & Row Sizing
* **Total Height**: `termHeight`
* **Outer Borders**: 2 rows (top and bottom edges)
* **Horizontal Dividers**: 4 rows (dividers separating the five rows)
* **Inner Height**: `termHeight - 6`
* **Row Proportions**:
  - **Row 1 (Config Form | Thumbnails)**: $\\sim 40\\%$ of Inner Height. Needs to be at least 10 rows to display the active form fields.
  - **Row 2 (Help Line)**: Fixed at **1 row**. Displays the contextual help text and preset browse hint.
  - **Row 3 (Job Queue)**: Fixed at $4$ rows (shows headers plus 3-4 running status lines).
  - **Row 4 (Output Image | History)**: $\\sim 35\\%$ of Inner Height.
  - **Row 5 (Debug/Log + Footer)**: Remaining height ($\\sim 20\\%$, minimum 4 rows for readable logging).

### 2.3 Minimum Terminal Size
* **Minimum Width**: **100 cells** (67 for Form + 1 for Divider + 30 for Thumbnails/Previews + 2 for Borders).
* **Minimum Height**: **24 cells** (12 for Form + 4 for Queue + 5 for Output/History + 3 for Debug/Borders).
* *Note: If terminal falls below 100x24, a clear overlay warning is displayed prompting the user to resize.*

---

## 3. Integration of Existing Components

* **Config Form**: Renders in the Left Column of Row 1. All 21 fields, input validation, two-column field pairings, and cursor logic are retained.
* **Help Line**: Renders as a full-width thin bar in Row 2. Displays the active field's help text plus the preset browse hint (`ℹ Help: <text> [Press → to browse presets.]`). Already implemented — just repositions from inside the form to its own row.
* **Job Queue**: Renders across the full width of Row 3. Retains the status icons (spinner, checkmark, cross), prompt truncation, credit costs, and inline confirmation state (e.g. `[y/n]`).
* **Debug Log**: Renders in Row 5. Displays recent log lines from the ring buffer.
* **Global Footer**: Displays at the very bottom of the Debug pane, showing global keyboard mappings (`ctrl+c`, `tab`, `enter`).
* **Floating Preset Popups**: When an input preset is browsed (e.g. Model, Sampler), the popup continues to render as an overlay centered in the terminal window, intercepting the key loop.

---

## 4. Image Thumbnails / Preview Panel (Row 1 Right)

This panel provides a quick visual inventory of images generated during the active session.

### 4.1 Grid Data
* Displays a grid of thumbnail boxes representing downloaded files.
* Each thumbnail card contains:
  - Sequence index (e.g. `#001`)
  - File extension/format (e.g. `PNG` or `JPG`)
  - Dimensions (e.g. `1024²` or `1:1`)
  - Short status indicator (e.g. `done` or `error`)

### 4.2 Pipeline Connection
* Connects to the engine's download thread.
* When `DownloadImage` completes, a message is dispatched to the TUI.
* The TUI appends the newly downloaded image path and metadata to the session inventory, forcing the thumbnail grid to update.

### 4.3 Visual Representation
* Renders 2 or 3 columns of compact boxes depending on `RightColWidth`:
  ```
  ┌──────────┐ ┌──────────┐
  │  #001    │ │  #002    │
  │  FLUX.D  │ │  FLUX.D  │
  │  1024²   │ │  1024²   │
  └──────────┘ └──────────┘
  ```
* Active selection is outlined with a sky-blue border.

---

## 5. Output Image & Past History Panels (Row 4)

### 5.1 Output Image Panel (Row 4 Left)
* Displays the currently selected image in a large framed viewport.
* If the terminal supports graphics protocols (e.g. Kitty, WezTerm), the image is displayed directly.
* If unsupported, it falls back to:
  - A grayscale/braille block representation of the image.
  - An inspector panel detailing all metadata: filename, prompt, base model, steps, CFG, seed, and credit cost.
* Users can toggle between the graphic view and full metadata card.

### 5.2 Past Images List (Row 4 Right)
* A scrollable vertical list showing history of generated/cached files in the output directory.
* Format: `[idx] filename (resolution) [status]`
* Selecting an item in this list automatically updates the large frame viewport on the left.

---

## 6. Keyboard Navigation & Panel Focus

Since there are no navigation tabs or vertical sidebar, navigation follows standard multi-pane conventions (similar to `lazygit`).

### 6.1 Focus Areas
The active focus is tracked via a simple state value indicating the active panel:
1. `FocusConfig` (Config Form inputs)
2. `FocusThumbnails` (Thumbnail Grid)
3. `FocusHistory` (Past Images History)

### 6.2 Key Routing Laws
* **Panel Cycling (`Tab` / `Shift+Tab`)**:
  - Cycles focus through the three active panels:
    $$\text{FocusConfig} \rightleftharpoons \text{FocusThumbnails} \rightleftharpoons \text{FocusHistory}$$
  - The focused panel is drawn with a bright white header and border; unfocused panels remain dim gray.

* **When `FocusConfig` is Focused**:
  - `Up` / `Down` arrows move the selection between the 21 form fields.
  - If a text input field (Prompt, Seed, etc.) is highlighted:
    - Pressing `Enter` focuses the text input (entering text edit mode, turning on cursor blink).
    - Pressing `Esc` de-focuses/blurs the text input (exiting text edit mode).
    - While in text edit mode, typing and standard text navigation keys (arrows, home, end, backspace) are captured by the text field.
  - If a preset field (Model, Sampler, etc.) is highlighted:
    - Pressing `Right Arrow` or `Enter` opens the floating preset popup.
    - Presets are scrolled via `Up`/`Down` and selected with `Enter`. `Esc` closes the popup.

* **When `FocusThumbnails` is Focused**:
  - `Up` / `Down` / `Left` / `Right` arrows navigate the grid of generated images.
  - `Enter` opens the selected image in the system's default image viewer and focuses it in the Output pane.

* **When `FocusHistory` is Focused**:
  - `Up` / `Down` arrows scroll through the history list.
  - Highlighting a history entry automatically loads and previews its details in the Output Image Panel.
  - `Enter` opens the file in the system viewer.

* **Global Actions**:
  - `ctrl+s` / `ctrl+p`: Validates form inputs and submits a new generation job.
  - `q` / `esc` (when not in edit mode or preset popup): Triggers a clean exit confirmation.
