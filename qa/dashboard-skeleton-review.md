# QA Review — Dashboard Skeleton Spec

**Date:** 2026-07-13
**Reviewer:** Grit (Grok)
**Artifacts:**
- Spec under review: `specs/dashboard-skeleton.md`
- Gap baseline: `qa/mockup3-gap-analysis.md`
- Related: `specs/mockup3-analysis.md`, `specs/form-layout.md`, `internal/ui/ui.go`
**Scope:** Spec QA only (no implementation yet). Quality gates N/A for pure design review.

---

## Verdict

**CHANGES_REQUESTED** — The skeleton correctly targets all three architectural blockers and preserves the Input-tab core, but the keyboard focus law has implementable regressions (`left` vs cursor movement, incomplete `FocusRight`, ambiguous global keys), and the width math does not reconcile with the form’s fixed 67-cell column at common terminal sizes (80–113 cols).

---

## Checklist Answers

### 1. Does the skeleton plan address all 3 architectural blockers?

| Blocker (from `qa/mockup3-gap-analysis.md` §6) | Addressed? | How |
|---|---|---|
| Multi-column layout (left / center / right) | **Yes** | §1 wireframe + §2 grid: fixed left (18), dynamic center, fixed right (25), outer frame |
| Tab-based navigation (9 sections + content swap) | **Yes** | §3 nine tabs; §4 center swaps by active tab; §7 `FocusPanel` + `1–9` |
| Images Library right panel | **Yes (skeleton)** | §5 persistent right column, empty placeholder, future scrollable paths |

**Notes:**
- Max/Tom directed colors to stay ANSI (not mockup brown/blue/teal). Spec correctly keeps sky-blue selection + dim borders. Good.
- Gap §1.4 **Thumbs/Previews** is intentionally out of scope for this skeleton (not one of the three blockers). Acceptable if called out as Phase 2 — currently unmentioned; recommend a one-line “deferred” note so implementers do not invent it.
- Tom’s chat priority said `Jobs = queue` and `Logs = debug` as separate content swaps. Spec instead stacks **form + queue + debug inside Input** and leaves Logs as a stub. That is a reasonable skeleton choice to preserve existing behavior, but it diverges from Tom’s bullet list. Document the decision explicitly so Tom does not “fix” Logs by moving the debug panel out of Input prematurely.

**Score: PASS on blockers (with documentation nits).**

---

### 2. Does the FocusPanel state machine correctly handle keyboard conflicts?

**Partial — important holes.**

What works:
- Separating **sidebar focus** vs **content focus** is the right design for `1–9` / `tab` vs form editing.
- In `FocusContent`, treating `1–9` as field input on text/numeric fields avoids the classic “digits jump tabs while typing seed/steps” bug.
- Routing `1–9` to tab switch only on preset/toggle fields (where typing is already blocked) is clever and matches current `isPresetsField` behavior.

What fails or is underspecified:

| Issue | Severity | Detail |
|---|---|---|
| **`left` steals text cursor** | **Critical** | §7.2: in `FocusContent`, `left` returns to sidebar. Current UI uses `left` for cursor movement inside `textinput` / `textarea`, and to exit the presets popup (`handleConfigKey`). Implementing the spec literally breaks prompt/seed editing. **Must** scope “leave content” to a key that does not fight the editor — e.g. `esc` only, or `ctrl+h` / `ctrl+←`, or `left` only when the field caret is at position 0 **and** not in a textarea multi-line edit. |
| **`FocusRight` has no routing** | **High** | Enum includes `FocusRight` but §7.2 only defines Sidebar and Content. No enter/leave keys, no scroll keys, no interaction with Images Library. Either fully specify or drop `FocusRight` until the library is interactive. |
| **`esc` semantics clash** | **High** | Today: `esc` closes presets, else **quits the app**. Spec: `esc` → sidebar. Default focus + quit path must be ordered: (1) dismiss popup, (2) content→sidebar, (3) optional second esc / `ctrl+c` quit. Spec must state the cascade. |
| **Job confirm `y`/`n` not listed** | **High** | `handleKey` currently intercepts `y`/`n` globally for `JobConfirming` before form routing. Spec is silent. If focus is Sidebar or user is on a stub tab, confirm must still work (queue is only painted under Input). Specify: confirmation keys are **global**, independent of `FocusPanel` / active tab; preferably keep queue status visible or force-focus Input when a job needs confirm. |
| **Submit key drift** | **Medium** | Footer: `^S Submit`. §7.2: `ctrl+s` / `ctrl+p`. Code today: **Enter** generates. Spec does not say Enter remains. Risk of removing the primary action key users already have. Preserve Enter; treat `ctrl+s` as optional alias if desired. `ctrl+p` is undefined in current code and unexplained in the spec. |
| **Quit key drift** | **Medium** | Footer shows `^Q Quit`; code is `ctrl+c`. Align footer with real bindings. |
| **Default focus on startup** | **Medium** | Unspecified. If default is `FocusSidebar`, every launch requires Enter/→ before typing — regresses UX. Should default to **`FocusContent` + tab Input**. |
| **Preset popup vs panel focus** | **Medium** | When `inPresetsPane`, keys must stay in popup (current behavior). Spec never mentions popup focus relative to `FocusPanel`. Add: popup is a modal overlay; until dismissed, ignore sidebar/tab shortcuts except quit. |
| **Sidebar `tab` only** | **Low** | Sidebar: `tab` cycles next. No `shift+tab` for previous. TODO.md mentions both. Add reverse cycle for symmetry. |
| **Preset field `1–9` → tab switch** | **Low** | Surprising but workable. Document in footer when a preset field is focused (“1–9 switch tabs”). |

**Score: CHANGES_REQUESTED on keyboard law.**

---

### 3. Does width math (`CenterWidth = termWidth - 47`) work for 80–200 columns?

**Arithmetic is fine under one interpretation; practical fit is not.**

#### Formula check

Claimed:

```
CenterWidth = termWidth - 18 - 25 - 2 (outer) - 2 (dividers) = termWidth - 47
```

| termWidth | CenterWidth | Fits form 67 cells? |
|---|---|---|
| 80 | 33 | **No** |
| 100 | 53 | **No** |
| 114 | 67 | Barely (form only, no padding) |
| 120 | 73 | Yes (tight) |
| 160 | 113 | Yes |
| 200 | 153 | Yes |

`specs/form-layout.md` fixes the solo form column at **67 cells** (`formCursorWidth + formLabelWidth + formLabelGap + formSoloInputW`). The skeleton never says the form shrinks, wraps, or scrolls horizontally. Therefore:

- **80–113 columns:** center is narrower than the existing form. Without a responsive rule, Tom will either clip, wrap garbage, or violate the tight-column constants.
- **Reasonable “daily driver” sizes (120–200):** OK once termWidth ≥ ~114–120.

#### Divider double-count ambiguity

§2.1 says left is “18 … **including** 1 character right divider spacing” and right is “25 … **including** 1 character left divider spacing”, then **also** subtracts “Vertical Dividers: 2”. That reads as double-counting.

Clarify one of:

1. **Content-only widths:** left=18, right=25, dividers separate → `Center = termWidth - 47` (current formula; “including” wording is wrong), or  
2. **Inclusive widths:** left=18 already has its divider, right=25 already has its divider → `Center = termWidth - 18 - 25 - 2 (outer) = termWidth - 45`.

#### Responsive policy missing

Spec needs a minimum strategy for small terminals, e.g.:

- Collapse right panel below width W, and/or  
- Collapse sidebar to icon/number strip, and/or  
- Allow center horizontal scroll / reduce `formSoloInputW`, and/or  
- Document **minimum supported width** (e.g. 120) and degrade gracefully below it.

**Score: FAIL for 80–col claim; PASS only if min width raised or responsive rules added.**

---

### 4. Are all 9 tabs accounted for? Is ordering sensible?

**Yes — matches gap-analysis list.**

| # | Gap analysis | Skeleton | Match |
|---|---|---|---|
| 1 | Input | Input | Yes (default / only live pane) |
| 2 | Popups | Popups | Yes (stub; live popups stay overlays) |
| 3 | Saved Prompts | Saved Prompts | Yes |
| 4 | Saved Profiles | Profiles | Yes (name shortened — OK if intentional) |
| 5 | Settings | Settings | Yes |
| 6 | Logs | Logs | Yes (stub) |
| 7 | Detailed Help/Keybinds | Help | Yes |
| 8 | Model Info | Model Info | Yes |
| 9 | Finder | Finder | Yes |

**Ordering:** Input first is correct. Feature stubs (prompts/profiles/settings) before meta (logs/help/model info/finder) is sensible. Images Library correctly stays a **persistent right column**, not a 10th tab.

**Divergences to note (not blockers for skeleton):**
- `specs/mockup3-analysis.md` OCR order differs (Help/Jobs/Thumbs earlier). Gap analysis + Max’s OCR list in chat is the better source for the 9-tab set; skeleton follows that.
- No separate **Jobs** tab; queue lives under Input. Acceptable for skeleton; call it out.

Label width: longest label “Saved Prompts” (13) fits in 18-char sidebar with room for selection marker.

**Score: PASS.**

---

### 5. Is existing functionality preserved?

**Intent: yes. Spec risks if implemented naively: yes.**

Preserved by design (§4.1):
- Config form (two-column `fieldOrder`)
- Job queue lifecycle rows
- Debug ring buffer panel
- Input masking / validation / replace-on-focus
- API pipeline (price → confirm → submit → poll → download) via existing cmds

Must remain true in implementation (spec should nail these):

| Concern | Risk if missed |
|---|---|
| Enter still submits when focus is Input/content | Footer-only `^S` misleads implementer |
| `y`/`n` still works while pricing confirm is open | Jobs stuck in `JobConfirming` if user switched tabs |
| Preset popup overlay still works on top of multi-column layout | Overlay math uses full `termWidth`/`termHeight` today — still OK if base view is full frame |
| Numeric masking checklist unchanged | No change needed if form code path is untouched |
| `ctrl+c` quit preserved | Footer says `^Q` only |
| Help line under form (`popup-presets.md`) | Skeleton footer may replace it — specify both: field help line **and** global footer, or merge carefully |
| Vertical packing of form + queue + debug in narrower center | No scroll/viewport policy; tall forms already strain small heights — three-column layout does not fix vertical overflow |

**Score: PASS on intent; specify key/global behaviors so implementation cannot regress.**

---

### 6. Gaps and risks not covered by the spec

1. **Critical: `left` vs in-field cursor / presets dismiss** (see §2).
2. **Critical: form min width 67 vs center at 80–113 cols** (see §3).
3. **High: `FocusRight` incomplete** — remove or fully specify.
4. **High: global keys table** — quit, confirm y/n, submit, esc cascade, popup modal layer.
5. **High: default focus + default tab** on first paint.
6. **Medium: height math vs footer** — §2.2 content height = `termHeight - 3`; footer is “bottom of center pane” but not subtracted. Queue + debug + form will overflow; no viewport/scroll law (mockup analysis §10 item 7).
7. **Medium: error flash (`errMsg`)** — currently below panels; no home in the new grid.
8. **Medium: header** — current `civitui [CONFIG]` phase badge; wireframe shows only outer title. Keep or drop?
9. **Medium: outer frame style** — rounded `╭╮` in wireframe vs current square `innerPanel` boxes. Clarify whether center keeps lazygit inner panels inside the outer frame.
10. **Low: Tom Jobs/Logs split vs Input-stacked** — document decision.
11. **Low: Thumbs/Previews deferred** — one line in deferred work.
12. **Low: accessibility of empty right panel** — always costs 25 cols; consider hide-when-empty later (not required for skeleton).
13. **Constitution:** layout is pure `internal/ui` — no SoC violation expected. No engine changes required. Good.

---

## Mapping: Gap Analysis Priorities → Skeleton Coverage

### Blockers
| # | Gap | Skeleton |
|---|---|---|
| 1 | Multi-column layout | Covered §1–2 |
| 2 | Tab navigation | Covered §3–4, §7 |
| 3 | Images Library panel | Covered §5 (stub) |

### Major features (correctly stubbed / deferred)
| # | Gap | Skeleton |
|---|---|---|
| 4–9 | Saved prompts/profiles, settings, finder, model info, full help page | Stubs only — correct for skeleton |
| 10 | Thumbnails/previews | Not mentioned — add “deferred” |

### Visual polish
| # | Gap | Skeleton |
|---|---|---|
| 11 | Colored panel backgrounds | Correctly **rejected** per Max (ANSI only) |
| 12 | Active tab highlight | Covered §3.2 (sky-blue) |
| 13 | Multi-panel borders | Outer frame + columns; inner panel style TBD |
| 14 | “Input” vs “Configure generation” title | Wireframe shows form fields; panel title not specified |

---

## Required Spec Changes (before Tom implements)

Must-fix:

1. **Keyboard law rewrite for content leave:** do not use bare `left` while a text/numeric field has caret movement needs. Prefer `esc` cascade; document popup → content → sidebar → quit.
2. **Global key table:** `ctrl+c` quit, Enter submit (keep), `y`/`n` confirm always, popup modal priority, optional `ctrl+s`.
3. **`FocusRight`:** full routing **or** remove from enum/TODO until library is interactive.
4. **Width:** fix divider wording; state minimum supported width **or** collapse rules; acknowledge form 67-cell constraint.
5. **Defaults:** `activeTab = Input`, `focus = FocusContent` on startup.
6. **Preserve:** help line under form; `errMsg` placement; Enter-to-generate.

Should-fix:

7. Height/overflow: what happens when form+queue+debug exceed center height (clip vs scroll).
8. Explicit “Logs/Jobs stay under Input for v1; separate tabs later.”
9. Deferred: thumbs/previews, resizable panes, panel chrome colors.

---

## What is already solid

- Clear three-column wireframe and role split (nav / content / library).
- Nine-tab list aligned with the gap analysis Max cares about.
- ANSI color decision matches operator clarification.
- Input tab as the live home for form + queue + debug avoids a big-bang rewrite of the pipeline.
- FocusSidebar vs FocusContent is the right abstraction for digit-key conflicts — it just needs a complete event matrix.
- Skeleton stubs for non-Input tabs match the “build the frame, not every page” mandate.

---

## Final verdict

**CHANGES_REQUESTED** — Address keyboard focus law holes (`left`/esc/`FocusRight`/globals), width math vs form 67-cell minimum (and divider wording), and default focus before implementation. Structural direction is correct and covers all three architectural blockers.
