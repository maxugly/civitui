# Project Todo List

## Active Tasks — API Parameter Expansion

Following Grok's audit: [specs/api_parameters_audit.md](specs/api_parameters_audit.md)

### Tier 1 — High Value, Easy Wins

- [x] **aspectRatio** — model-aware dropdown that auto-fills width/height. Presets per model: 1:1, 3:2, 2:3, 16:9, 9:16, 4:3, 3:4. Flux Ultra adds 21:9, 9:21.
- [x] **fluxMode** — dropdown for Flux model variants: Draft / Standard / Krea / Pro 1.1 / Ultra. Only visible when base model is Flux1. Maps labels to AIR URNs internally.
- [x] **outputFormat** — PNG vs JPEG toggle. Simple dropdown. Default JPEG.
- [x] **scheduler** — dropdown: simple, discrete, karras, exponential, ays. Hide in advanced section. Model-aware: ZImage only simple/discrete, Flux2Klein all except ays.
- [x] **draft** — toggle for fast preview mode. Injects draft LoRAs, drops steps to 6–8. "Speed over quality." (Spec: [specs/draft-toggle.md](specs/draft-toggle.md)) — completed in commit pending

## Active Tasks — Draft Toggle

- [x] **Write Specification** — Draft toggle specification [specs/draft-toggle.md](specs/draft-toggle.md)
- [x] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `Draft` boolean field (`json:"draft"`) to `civit.GenerationRequest` struct.
- [x] **Update Constants and Preset Config (`internal/ui/ui.go`)**
  - Increment `numFormFields` from 14 to 15.
  - Add `fiDraft` constant (value 14).
  - Update `isPresetsField()` to return true for `fiDraft`.
  - Add `draftPresets` slice with `true` and `false` options and descriptions.
- [x] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - In `NewModel()`, initialize `inputs[fiDraft]` with placeholder "false" and initial value "false".
  - In `fieldLabels`, add `"Draft Mode (fast preview)"` at index 14.
  - In `fieldHelpText`, add description for the draft toggle at index 14.
  - In `viewConfig()`, add case for `fiDraft` to display the "Draft Mode" preset list.
- [x] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - In `handleConfigKey()`, handle preset navigation length (`presetsLen`) and selection (`SetValue()`) for `fiDraft`.
  - In `toRequest()`, parse the string value of `inputs[fiDraft]` into `Draft` boolean of `civit.GenerationRequest`.
- [x] **Add Unit Tests and Verify Quality Gates**
  - Update `pkg/civit/civit_test.go` or add assertions to verify that the `draft` field is marshalled correctly.
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1` to ensure all pass.

### Tier 2 — Worth Adding, Medium Effort

- [ ] **denoise** — float input 0.0–1.0 (step 0.05). For img2img. Only show when source image is attached.
- [ ] **clipSkip** — int input 1–3. SD/SDXL only. Default 2. Hide for Flux/ZImage.
- [ ] **upscaleWidth + upscaleHeight** — paired int inputs 320–3840. For upscaling workflows.
- [ ] **experimental** — toggle for SDCPP edge features. Advanced section.
- [ ] **fluxUltraRaw** — toggle. Only visible when fluxMode is Ultra.

### Bugfix

- [x] **BUGFIX: model → baseModel JSON key** — `pkg/civit/civit.go` line 52 sends `json:"model"` but schema expects `json:"baseModel"`. Fixed in commit.

## Completed Tasks

- [x] **Add Sampler Parameter and Range Validation** — *Completed by Tom on 2026-07-12*
  - All subtasks complete. Additionally: input masking on CFG Scale (keystroke-level max-7.0 enforcement), resolution raised to 4K (4096), quantity default 4.
  - **API-aligned pass (Grok research):** Steps capped at 100 (API Zod schema: max(100)), quantity capped at 20 (API: max(20)), seed bounded 1–4294967295 (API: uint32 max). CFG kept at 7.0 per operator directive despite API allowing 30.
