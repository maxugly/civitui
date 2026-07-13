# Project Todo List

## Active Tasks — Feature Expansion & API Integration

- [ ] **Floating Preset Popups and Thin Help Line** (per [specs/popup-presets.md](file:///home/m/snc/cod/civitui/specs/popup-presets.md))
  - [ ] **Remove Old Split-Pane Layout**: Delete the right-pane presets and help column rendering from `viewConfig` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go), simplifying the config pane to a single-column layout.
  - [ ] **Implement Contextual Help Line**: Render the active input's help text as a single thin status line (`ℹ Help: <text> [Press → to browse presets]`) at the bottom of the config pane.
  - [ ] **Build Bordered Preset Popup Panel**: Write `viewPresetsPopup() string` to compile a bordered panel containing the active input's presets, highlighting the item at `activePreset`.
  - [ ] **Write ANSI-Safe Overlay Logic**: Implement the `overlayLine` helper to safely slice and write the popup box on top of the base view without breaking ANSI/color escape codes.
  - [ ] **Wire Popup Event Loop & Key Block**: Update key routing in `handleConfigKey` and `Update` so that all keystrokes except global quit are routed to the active popup and blocked from underlying inputs when `inPresetsPane` is true.
- [ ] **img2img Source Image Interface** — Implement user interface elements (e.g., text input field for local file path or URL) to support source image selection. When an image is loaded/linked, automatically set `sourceImage` on the model to enable the `denoise` parameter input field.
- [ ] **Migrate Engine to `imageGen` URN Orchestration** — Transition `pkg/civit` client workflow payload creation to target the newly-supported `imageGen` orchestration workflow API (featuring `engine`, `ecosystem`, and `operation` parameters) instead of legacy `textToImage` steps.
- [ ] **Stale Job Queue Auto-Cleanup** — Add periodic background cleanup (or a manual clear shortcut) for completed or failed jobs in the TUI queue older than 5 minutes to prevent memory accumulation.

---

## Completed Tasks

### Single-Window Lazygit Layout (shipped)
- [x] **Single-Window Dashboard** — Redesign TUI layout to keep the configuration form visible at all times and render generation progress/status inline in a bottom queue pane.
- [x] **Job Queue and Status Pipeline** — Define `Job` and `JobStatus` state models. Collapse pricing, submitting, polling, downloading, and done phases into status rows in the bottom pane.
- [x] **Two-Column Form Pairing** — Implement a paired two-column layout via `fieldOrder` mapping with negative indices indicating pair starts.
- [x] **Column-Major Focus Navigation** — Implement `navOrder()` and `advanceFocus()` to traverse left-column fields top-to-bottom first, then right-column fields.
- [x] **Autofocus Recovery** — Add `refocusIfHidden()` to shift focus away from fields that are dynamically hidden after changing models.
- [x] **Tight Form Column Layout** — Formulate a content-sized left-column width grid (`formSoloInputW`, `formPairInputW`, `formColGap`) to remove the dead gutter and align left and right panes perfectly.

### Tier 1 API Parameters (shipped)
- [x] **aspectRatio** — model-aware dropdown that auto-fills width/height.
- [x] **fluxMode** — dropdown for Flux model variants (Draft/Standard/Krea/Pro/Ultra).
- [x] **outputFormat** — JPEG/PNG toggle dropdown.
- [x] **scheduler** — Restore correct sampler-algorithm-based scheduler presets.
- [x] **draft** — Toggle for fast preview mode (Draft LoRAs, reduced steps, CFG=1).

### Tier 2 API Parameters (shipped)
- [x] **denoise** — Float input 0.0–1.0. (Logic, validation, and field initialization completed; temporarily hidden in TUI until source image attachment is integrated).
- [x] **clipSkip** — Int input 1–3. Hidden for Flux/ZImage models.
- [x] **upscaleWidth + upscaleHeight** — Paired inputs 320–3840 for upscaling.
- [x] **experimental** — Toggle for SDCPP edge features.
- [x] **fluxUltraRaw** — Toggle for Flux Ultra raw mode.

### Bugfixes & Visibility Blockers (shipped)
- [x] **Fix Custom AIR Substring Matching** — Check `flux2` before `flux` to prevent incorrect ecosystem fallback to `sdxl`.
- [x] **Implement Active Sampler/Scheduler Adjustment** — Reset sampler/scheduler to valid defaults during model changes.
- [x] **Fix Navigation Loops** — Bound the tab traversal iterations to prevent terminal hangs when all fields are hidden.
- [x] **BUGFIX: model -> baseModel JSON key** — Sent correct API payload keys.
