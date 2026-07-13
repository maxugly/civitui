# Specification: Multi-Panel Dashboard Skeleton Layout

This specification defines the structural plan to transition `civitui` from a single-column stacked layout to a multi-panel dashboard skeleton. The design is inspired by `mockup3.png`, but keeps the existing ANSI terminal color scheme (white text, dim borders, sky-blue selection) instead of using the mockup's colored background blocks.

---

## 1. Layout Wireframe

```
╭── civitui ─────────────────────────────────────────────────────────────────╮
│ Input        │ ▶ Prompt: [car___________] ── Help  │ Images Library        │
│ Popups       │   Model: [air:flux1:...]            │ (empty)               │
│ Saved Prompts│   Width: 1024  Height: 1024         │                       │
│ Profiles     │   ...                               │                       │
│ Settings     │                                     │                       │
│ Logs         │ ── Queue ────────────────────────── │                       │
│ Help         │ ⣾ 'car'  30ⓑ  generate? [y/n]      │                       │
│ Model Info   │                                     │                       │
│ Finder       │                                     │                       │
│              │                                     │                       │
│              │ ^S Submit  ^Q Quit  1-9 Tabs        │                       │
╰────────────────────────────────────────────────────────────────────────────╯
```

---

## 2. Grid & Layout Math

To maintain a responsive full-screen TUI, the interface is split horizontally into three main panels inside a single outer frame.

### 2.1 Width Allocation
* **Total Width**: `termWidth`
* **Outer Frame Borders**: 2 characters (left and right outer edges)
* **Vertical Dividers**: 2 characters (two column dividers `│`)
* **Left Sidebar**: 18 characters (fixed width, including 1 character right divider spacing)
* **Right Panel (Images Library)**: 25 characters (fixed width, including 1 character left divider spacing)
* **Center Content Pane**: Dynamic remaining width:
  $$\text{CenterWidth} = \text{termWidth} - 18 - 25 - 2 (\text{outer borders}) - 2 (\text{dividers}) = \text{termWidth} - 47$$

### 2.2 Height Allocation
* **Total Height**: `termHeight`
* **Outer Frame Borders**: 2 rows (top and bottom edges)
* **Header Spacing**: 1 row (under the top border title)
* **Content Height**: `termHeight - 3` rows.
* All three columns (Left Sidebar, Center Pane, Right Panel) share this content height.

---

## 3. Left Sidebar (Navigation panel)

The sidebar is a vertical list of tab options acting as the primary navigation.

### 3.1 Tab Labels & Stubs
The sidebar contains exactly 9 tabs:
1. **Input** (Active: contains the existing config form, job queue, and debug panel)
2. **Popups** (Stub: displays placeholder)
3. **Saved Prompts** (Stub: displays placeholder)
4. **Profiles** (Stub: displays placeholder)
5. **Settings** (Stub: displays placeholder)
6. **Logs** (Stub: displays placeholder)
7. **Help** (Stub: displays placeholder)
8. **Model Info** (Stub: displays placeholder)
9. **Finder** (Stub: displays placeholder)

### 3.2 Visual Styling
* **Active Tab**: Highlighted with a sky-blue background block (using ANSI color `Color("25")` and bold white text) to match the form field selection color.
* **Inactive Tabs**: Rendered in dim gray or default text style.
* **Selection Cursor**: Indicates active focus when the sidebar panel is active.

---

## 4. Center Content Pane

The center pane dynamically swaps its contents based on the currently active tab.

### 4.1 'Input' Tab (Active Pane)
When the active tab is **Input**, the center pane displays the existing core interface components stacked vertically:
1. **Config Form**: The two-column configuration form.
2. **Job Queue**: The active generation status rows (pricing, confirming, polling, downloading, done).
3. **Debug Log Panel**: Active logging ring buffer.

All existing inputs, input masking, validation, and execution pipelines are preserved unchanged inside this pane.

### 4.2 Labeled Stubs
For all other tabs (indexes 1–8), the center pane displays a stylized placeholder:
```
╭───────────────────────────────────────────────────╮
│                                                   │
│            [Tab Name] — Coming Soon               │
│                                                   │
╰───────────────────────────────────────────────────╯
```

---

## 5. Right Panel (Images Library)

A persistent column on the right side of the screen.
* **Title**: `Images Library` centered at the top.
* **Initial State**: Renders an empty list placeholder:
  ```
  (empty)
  ```
* **Future State**: Will render a scrollable list of generated image file paths.

---

## 6. Footer Bar

Rendered at the bottom of the center pane, displaying context-sensitive shortcuts.
* **Layout**:
  ```
  ^S Submit  ^Q Quit  1-9 Tabs  Tab/↑↓ Navigate
  ```
* **Colors**: Dim gray text.

---

## 7. Keyboard Navigation Laws

To avoid collisions between input field editing (where letters and numbers are typed) and tab switching shortcuts:

### 7.1 Focus States
The TUI introduces a layout focus state in the `Model`:
```go
type FocusPanel int

const (
	FocusSidebar FocusPanel = iota
	FocusContent
	FocusRight
)
```

### 7.2 Event Routing Table
* **When `FocusSidebar` is active**:
  * `up` / `down` / `j` / `k`: Moves the active tab selection.
  * `1` to `9`: Switches directly to the corresponding tab index (0 to 8).
  * `tab`: Cycles to the next tab.
  * `enter` / `right` / `l`: Switches focus to `FocusContent` (if the tab is active).
* **When `FocusContent` is active**:
  * Key inputs are routed directly to the form fields (Input tab) or sub-panels.
  * `tab` / `shift+tab` / `up` / `down`: Navigate between form fields.
  * `esc` / `left`: Swaps focus back to `FocusSidebar`.
  * `ctrl+s` / `ctrl+p`: Standard submit/generate triggers.
  * `1-9` keys: Act as text/numeric input if a text/numeric field is focused. If a preset/toggle field is focused (where direct typing is blocked), `1-9` keys switch tabs immediately.
