# Mockup3 Layout Analysis — Blueprint for Final TUI

> Analyzed from `/home/m/snc/cod/civitui/sketches/mockup3.png` (781×544, PNG)
> by pixel-level structural decomposition.
> This is the **target layout** for the civitui single-window dashboard.

---

## 1. Color Palette

| Role | Color | Notes |
|---|---|---|
| Background | `#111E1E` | Very dark teal-black. Uniform across the entire canvas. Terminal default background. |
| Content / Borders / Text | `#0000A0`–`#0000E6` | Pure blue, varying brightness levels. Brighter for headers/borders, dimmer for body text. Single-hue monochrome — no other accent colors appear in the mockup. |
| Highlight/Selection | N/A in mockup | Not depicted. Will be sky-blue per `form-layout.md` (implementation detail). |

**Key takeaway**: The mockup is a **monochrome blue terminal UI**. All structural elements — borders, section headers, field labels, text — are rendered in blue against the dark background. No green/pink/orange/white accents are present; those were quantization artifacts.

---

## 2. Outer Frame

The entire application is enclosed in a **single-line rectangular border**:

- **Top edge**: A full-width horizontal blue line spanning the entire terminal width (row 0 in the mockup). Thin, single-pixel/character height.
- **Bottom edge**: Matching full-width horizontal line at the bottom.
- **Left edge**: Continuous vertical line from top to bottom.
- **Right edge**: Continuous vertical line, slightly inset from the left edge (there's a 1-pixel gap between left edge and frame, but right edge aligns with frame edge).
- **Corner joints**: The top two corners show `▓▓▓▓...` (dense blue), suggesting **rounded or double-thick corners** at the frame joints. This is characteristic of terminal box-drawing characters (e.g., `╭─╮│╰─╯` with Unicode box-drawing glyphs).

**Implementation note**: Use lipgloss `Border` with `NormalBorder()`, rounded corners, or Unicode box-drawing characters for the outer frame.

---

## 3. Left Sidebar (Navigation Panel)

**Position**: Full-height along the left edge, from row 3 to row 133 (in 200×139 scale). Approximately 7-8% of total width.

**Structure**:
- The sidebar is a **vertical stack of labeled navigation tabs**.
- Each tab is a **horizontal bar** of blue text on the dark background, separated by thin spacers.
- Tabs have a **pressed/selected state** indicated by bright blue fill (denser blue pixels).

**Tab order (top to bottom)**, based on OCR and structural analysis:

| Row (approx) | Tab Label | Description |
|---|---|---|
| ~5-7 | **Input** | The configuration form. First/default tab. |
| ~8-10 | **Help** | Help / keybindings reference. |
| ~15-17 | **Jobs** | Job queue and status view. |
| ~18-20 | **Thumbs / Previews** | Generated image thumbnails. |
| ~25+ | **Images Library** | Scrollable list of saved/downloaded images with numbered entries (1, 2, 3, ...). |
| ~40+ | **Popups** | Sub-items: menus for models and settings. |
| | **Saved Prompts** | Positive and negative prompt presets. |
| | **Saved Profiles** | Settings profiles. |
| | **Logs** | Application log output. |
| | **Model Info** | Detailed help, keybinds, model metadata. |
| | **Later: Finder** | Placeholder for future: find models, find LoRAs, get metadata. |

**Visual style**:
- Active/selected tab: **Bright blue background fill** (dense `█` pixels, e.g., rgb 0,0,200-230).
- Inactive tabs: Thin blue text outline (sparse pixels, rgb 0,0,100-160).
- Separator lines: Thin single-pixel horizontal rules between tab groups.
- Each tab has consistent height (~3-4 character rows).

**Scroll behavior**: The sidebar appears to be scrollable — the Images Library has numbered entries that continue past the visible area.

---

## 4. Main Content Area — Three Vertical Panes

The main content area fills the remaining ~93% of width to the right of the sidebar. It is divided into **three stacked panes**, separated by horizontal divider bars:

### 4.1 Top Pane: Configuration Form (rows ~5–47)

**Header Bar**:
- Full-width bright blue bar at the very top of the content area.
- Contains the section title (likely "Generate" or "Config").
- Single row, bright blue fill, white/light text (inferred from density).

**Form Body (rows ~6–47)**:
- The form uses a **two-column layout** for most fields.
- Structure is similar to the existing `fieldOrder` system in `form-layout.md`:
  - **Full-width rows**: Single fields spanning the entire form column width (e.g., Prompt, Negative Prompt, Model).
  - **Paired rows**: Two fields side-by-side (e.g., Width × Height, CFG Scale × Steps).
- A **vertical separator** at approximately column 70 (in the 200-column scaled view) divides the left and right field columns within paired rows.

**Field arrangement** (inferred from pixel density patterns and existing `fieldOrder`):

| Position | Field(s) | Layout |
|---|---|---|
| Row ~6-8 | Prompt | Full-width (multi-line textarea) |
| Row ~9-10 | Negative Prompt | Full-width (multi-line textarea) |
| Row ~11-12 | Model | Full-width (dropdown/preset selector) |
| Row ~13-14 | Flux Mode | Full-width (dropdown) |
| Row ~15-16 | Sampler | Left of pair |
| Row ~15-16 | Scheduler | Right of pair |
| Row ~19-20 | Aspect Ratio | Left of pair |
| Row ~19-20 | Width | Right of pair (paired with Height below?) |
| Row ~21-22 | Height | Left of pair |
| Row ~21-22 | CFG Scale | Right of pair |
| Row ~25-26 | Steps | Left of pair |
| Row ~25-26 | Seed | Right of pair |
| Row ~27-28 | Quantity | Full-width |
| Row ~29-31 | Output Format | Full-width (dropdown) |
| Row ~33-36 | Toggles row | Experimental, Flux Ultra Raw, Draft, Denoise |
| Row ~38-40 | Clip Skip | Left of pair |
| Row ~38-40 | Upscale Width | Right of pair |
| Row ~42-43 | Upscale Height | Full-width? |

> **Note**: The exact field pairing is inferred from the two-column density pattern in the scaled view. The existing code's `fieldOrder` has been refined through implementation — this mockup shows the visual target. Some labels from the mockup may overlap with current implementation.

**Field rendering details**:
- Each field row is a **horizontal band** with consistent height (~2-3 character rows).
- **Label**: Left-aligned within its column, ~16 characters wide (matches `formLabelWidth = 16`).
- **Input field**: Blue-bordered box to the right of the label. Brighter border on the focused/active field.
- **Dropdown indicators**: Some fields show a small `▼` or chevron marker at the right edge of the input.
- **Numeric fields**: Narrow input boxes (~10 characters wide for paired fields, matching `formPairInputW = 10`).
- **Text fields** (Prompt/Negative): Taller input areas (multi-line text, ~3-5 rows).
- **Toggle fields**: Short label + `[ON]` / `[OFF]` indicator.

**Vertical scrolling**: The form may exceed the visible pane height. A scrollbar indicator is not visible in the mockup but the form continues beyond the pane boundary.

---

### 4.2 Middle Pane: Job Queue (rows ~48–63)

**Header Bar**:
- Full-width bright blue bar, same style as the form header.
- Label: "Jobs" or "Queue".
- Spans the entire content area width.

**Queue Body (rows ~49–63)**:
- A **scrollable list of job entries**, each occupying one or two rows.
- Each entry row format:
  ```
  [status_icon] [job_id]  [model_name]  [progress/status]  [time_elapsed]
  ```
- Status indicators (inferred from layout):
  - **Pending**: Blue text, dim
  - **Pricing**: Blue text with spinner/progress indicator
  - **Submitting**: Blue text, slightly brighter
  - **Polling**: Blue text with progress bar or percentage (e.g., "Generating… 45%")
  - **Downloading**: Blue text with "↓" indicator
  - **Done**: Bright blue with checkmark
  - **Error**: Bright blue with "✗" or "!" indicator
- Queue entries are **stacked vertically** with thin separators between entries.
- The **active job** (currently being processed) is highlighted with brighter blue.
- **Completed jobs** are dimmer or have a distinctive marker.

**Implementation note**: This pane is currently implemented as inline status rows in the bottom pane of the existing TUI, per `TODO.md` task "Job Queue and Status Pipeline."

---

### 4.3 Bottom Pane: Debug / Log Output (rows ~64–131)

**Header Bar**:
- Full-width bright blue bar.
- Contains **tab indicators** for sub-panels within the debug area.
- Multiple tab-like markers visible: `███▓` (active tab), `██▓` (inactive tab).
- Possible tab labels: "Output", "Log", "Errors", "API".

**Debug Body (rows ~74–131)**:
- Large scrollable text area filling most of the remaining height.
- Displays **structured terminal output** in a monospace font.
- Three-column text layout visible in rows 108–131:
  - **Column 1** (left): Timestamp or log level indicator.
  - **Column 2** (middle): Source/component label (e.g., `[API]`, `[ENGINE]`, `[UI]`).
  - **Column 3** (right): Log message text.
- The area scrolls independently — new output appears at the bottom.
- The content appears to be **line-buffered terminal output** (like `tail -f` of a log file).

**Footer area** (below debug pane, rows ~132–133):
- A **thin status bar** spanning the full content width.
- Shows contextual help or keyboard shortcuts:
  - Example format: `^S Submit  ^Q Quit  ^H Help  Tab Next`
- Bright blue text on dark background.

---

## 5. Section Dividers

Horizontal dividers between the three main panes:

| Position (approx row) | Style | Description |
|---|---|---|
| Row ~47–48 | Full-width bright blue bar | Divider between form pane and queue pane. Same style as pane headers. |
| Row ~63–64 | Full-width bright blue bar | Divider between queue pane and debug pane. |

These dividers are **distinct visual breaks** — not simple single lines, but header-style bars that include the pane title. Each reads as: `─── Section Title ───` (bright blue fill).

---

## 6. Spacing and Margins

- **Outer frame padding**: 1 character cell between the frame border and interior content.
- **Sidebar-to-content gap**: ~2-3 character cells of dark background separating the left sidebar from the main content area.
- **Pane-to-pane gap**: 0 — panes are directly adjacent, separated only by the header/divider bars.
- **Form field spacing**: 
  - 1 row between consecutive field rows (thin dark gap).
  - 2 rows between logical field groups (e.g., after toggles before upscale fields).
- **Column gap in paired fields**: ~2 character cells between left and right field columns (matches `formPairGap = 2`).

---

## 7. Panel Sizing (Proportional)

Based on the 781×544 pixel canvas:

| Element | Approx Height | Percentage |
|---|---|---|
| Top frame border | ~10px | 2% |
| Left sidebar | full height | ~7% width |
| Config form pane | ~220px | 40% |
| Job queue pane | ~85px | 16% |
| Debug/log pane | ~200px | 37% |
| Footer/status bar | ~12px | 2% |
| Bottom frame border | ~10px | 2% |

**Width breakdown**:
| Element | Approx Width | Percentage |
|---|---|---|
| Outer frame + padding | ~15px | 2% |
| Left sidebar | ~55px | 7% |
| Sidebar gap | ~15px | 2% |
| Main content area | ~696px | 89% |

---

## 8. Keyboard Navigation Hints

Based on the footer area and existing `form-layout.md`:

- **Tab / ↓**: Move to next field or pane.
- **Shift+Tab / ↑**: Move to previous field or pane.
- **Enter**: Submit / confirm selection.
- **Esc**: Go back / dismiss popup / quit (with confirmation).
- **Ctrl+Q**: Quit application.
- **Ctrl+S**: Submit generation job.
- **→**: Open preset browser / expand dropdown (per `popup-presets.md`).
- **←**: Close preset browser / collapse dropdown.
- **1-9**: Quick-select sidebar tabs (lazygit-style number shortcuts).

---

## 9. Visual Element Summary

| Element | Border Style | Fill | Text Color |
|---|---|---|---|
| Outer frame | Single-line, rounded corners (`╭─╮│╰─╯`) | None (transparent) | N/A |
| Sidebar tabs (active) | None | Bright blue solid | Dark bg (inverse) |
| Sidebar tabs (inactive) | None | None | Dim blue |
| Pane headers | None (full-width bar) | Bright blue solid | Dark bg (inverse) |
| Form field labels | None | None | Medium blue |
| Form field inputs | Single-line box border | None/dark bg | Bright blue |
| Focused field | Thicker/brighter border | Sky-blue highlight | Per form-layout.md |
| Queue entries | None (row-based) | None | Medium blue |
| Active job row | None | Subtle blue highlight | Bright blue |
| Debug output | None | None | Medium blue (monospace) |
| Footer/status bar | None (full-width bar) | Bright blue solid | Dark bg (inverse) |
| Popup overlays | Double-line border | Dark bg | Blue (per popup-presets.md) |

---

## 10. Implementation Roadmap Notes

This mockup represents the **final target layout**. The current implementation (as of 2026-07-13) has:

- ✅ Single-window lazygit-style dashboard (shipped)
- ✅ Tight form column layout with two-column pairing (shipped)
- ✅ Column-major focus navigation (shipped)
- ✅ Floating preset popups and thin help line (shipped)
- ✅ Job queue and status pipeline (shipped)

**Remaining work** to match this mockup:

1. **Left sidebar navigation panel**: Implement the vertical tab bar with Input/Help/Jobs/Thumbs/Images Library tabs, keyboard shortcuts (1-9), and tab switching.
2. **Three-pane vertical split**: Separate the current combined queue/debug area into distinct "Job Queue" and "Debug/Log" panes with their own titled headers.
3. **Debug/log pane**: Implement structured log output with tab navigation (Output/Log/Errors/API tabs), timestamp columns, and scrollable terminal-style output.
4. **Images Library**: Build the thumbnail browser sidebar tab showing downloaded/generated images.
5. **Outer frame with rounded corners**: Apply lipgloss border styling to the full application window.
6. **Footer status bar**: Implement the thin help/status bar at the bottom of the debug pane or as a global footer.
7. **Panes as independently scrollable viewports**: Each pane (form, queue, debug) should independently handle overflow with its own scroll state.
8. **Resizable panes**: Allow the user to resize the vertical split between form, queue, and debug panes (lazygit-style `[`/`]` or mouse drag).

---

## Appendix: Pixel Analysis Methodology

Since the deepseek model does not support vision/image analysis natively, this document was produced through:

1. **ImageMagick structural decomposition**: Scaled the 781×544 image to multiple resolutions (80×56, 160×112, 200×139) and performed pixel-density analysis to detect borders, panels, and text regions.
2. **OCR extraction**: Tesseract OCR at native and 2× resolution to extract text labels.
3. **Color quantization**: Histogram analysis to identify the monochrome blue palette.
4. **Row/column profiling**: Per-row and per-column brightness profiling to locate horizontal dividers and vertical separators.
5. **Existing spec cross-reference**: Validated against `specs/form-layout.md`, `specs/popup-presets.md`, and `TODO.md` for consistency with the current implementation state.

**Confidence**: High for structural layout (positions of panels, dividers, sidebar). Moderate for specific field label text (OCR quality was limited by image resolution). The architectural structure is definitive; exact text labels may need Max's confirmation.
