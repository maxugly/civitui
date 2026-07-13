# Project Todo List

## Active Tasks — API Parameter Expansion

Following Grok's audit: [specs/api_parameters_audit.md](specs/api_parameters_audit.md)

### Tier 1 — High Value, Easy Wins

- [x] **aspectRatio** — model-aware dropdown that auto-fills width/height. Presets per model: 1:1, 3:2, 2:3, 16:9, 9:16, 4:3, 3:4. Flux Ultra adds 21:9, 9:21.
- [x] **fluxMode** — dropdown for Flux model variants: Draft / Standard / Krea / Pro 1.1 / Ultra. Only visible when base model is Flux1. Maps labels to AIR URNs internally.
- [x] **outputFormat** — PNG vs JPEG toggle. Simple dropdown. Default JPEG.
- [x] **scheduler** — dropdown: simple, discrete, karras, exponential, ays. Hide in advanced section. Model-aware: ZImage only simple/discrete, Flux2Klein all except ays.
- [x] **draft** — toggle for fast preview mode. Injects draft LoRAs, drops steps to 6–8. "Speed over quality." (Spec: [specs/draft-toggle.md](specs/draft-toggle.md)) — completed in 7e93466, QA passed

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

- [ ] **denoise** — float input 0.0–1.0 (step 0.05). For img2img. Only show when source image is attached. (Spec: [specs/denoise.md](specs/denoise.md))
- [ ] **clipSkip** — int input 1–3. SD/SDXL only. Default 2. Hide for Flux/ZImage. (Spec: [specs/clip-skip.md](specs/clip-skip.md))
- [ ] **upscaleWidth + upscaleHeight** — paired int inputs 320–3840. For upscaling workflows. (Spec: [specs/upscale.md](specs/upscale.md))
- [ ] **experimental** — toggle for SDCPP edge features. Advanced section. (Spec: [specs/experimental.md](specs/experimental.md))
- [ ] **fluxUltraRaw** — toggle. Only visible when fluxMode is Ultra. (Spec: [specs/flux-ultra-raw.md](specs/flux-ultra-raw.md))

## Active Tasks — Denoising Strength (denoise)

- [x] **Write Specification** — Denoising Strength specification [specs/denoise.md](specs/denoise.md)
- [ ] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `SourceImage` struct and `SourceImage`, `Denoise` fields to `civit.GenerationRequest` struct.
- [ ] **Update Constants and config (`internal/ui/ui.go`)**
  - Increment `numFormFields` from 15 to 21.
  - Add `fiDenoise` constant (value 15).
  - Update `isReplaceOnFocus()` to return true for `fiDenoise`.
- [ ] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - In `NewModel()`, initialize `inputs[fiDenoise]` with placeholder "0.4" and value "0.4".
  - In `fieldLabels`, add `"Denoise"` at index 15.
  - In `fieldHelpText`, add description for the denoise field at index 15.
  - Define `isFieldVisible()` helper and update `viewConfig()` rendering loop and key navigation to skip `fiDenoise` when no source image is attached.
- [ ] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - Add numeric range validation to `validateNumericFields()` for `fiDenoise`.
  - In `toRequest()`, parse string value of `inputs[fiDenoise]` into `Denoise` float64 pointer of `civit.GenerationRequest`.
- [ ] **Add Unit Tests and Verify Quality Gates**
  - Update `pkg/civit/civit_test.go` to assert correct marshalling of `denoise` and `sourceImage`.
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.

## Active Tasks — CLIP Skip (clip-skip)

- [x] **Write Specification** — CLIP Skip specification [specs/clip-skip.md](specs/clip-skip.md)
- [ ] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `ClipSkip` integer field (`json:"clipSkip,omitempty"`) to `civit.GenerationRequest` struct.
- [ ] **Update Constants and config (`internal/ui/ui.go`)**
  - Add `fiClipSkip` constant (value 16).
  - Update `isReplaceOnFocus()` to return true for `fiClipSkip`.
- [ ] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - In `NewModel()`, initialize `inputs[fiClipSkip]` with placeholder "2" and value "2".
  - In `fieldLabels`, add `"CLIP Skip"` at index 16.
  - In `fieldHelpText`, add description for clip skip at index 16.
  - Update `isFieldVisible()` helper to hide `fiClipSkip` when the base model is Flux or ZImage.
- [ ] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - Add numeric range validation to `validateNumericFields()` for `fiClipSkip`.
  - In `toRequest()`, parse string value of `inputs[fiClipSkip]` into `ClipSkip` integer pointer of `civit.GenerationRequest`.
- [ ] **Add Unit Tests and Verify Quality Gates**
  - Update `pkg/civit/civit_test.go` to assert correct marshalling of `clipSkip`.
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.

## Active Tasks — Upscale Dimensions (upscale)

- [x] **Write Specification** — Upscale Dimensions specification [specs/upscale.md](specs/upscale.md)
- [ ] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `UpscaleWidth` and `UpscaleHeight` integer fields to `civit.GenerationRequest`.
- [ ] **Update Constants and config (`internal/ui/ui.go`)**
  - Add `fiUpscaleWidth` (value 17) and `fiUpscaleHeight` (value 18) constants.
  - Update `isReplaceOnFocus()` to return true for both.
- [ ] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - In `NewModel()`, initialize both inputs with placeholder "disabled" and empty value.
  - In `fieldLabels` and `fieldHelpText`, add labels and descriptions for upscaling width/height.
- [ ] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - Add range validations to `validateNumericFields()`.
  - In `toRequest()`, parse values into `UpscaleWidth` and `UpscaleHeight` integer pointers.
- [ ] **Add Unit Tests and Verify Quality Gates**
  - Verify marshalling of upscaling fields in `civit_test.go`.
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.

## Active Tasks — Experimental Mode (experimental)

- [x] **Write Specification** — Experimental Mode specification [specs/experimental.md](specs/experimental.md)
- [ ] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `Experimental` boolean field to `civit.GenerationRequest`.
- [ ] **Update Constants and Preset Config (`internal/ui/ui.go`)**
  - Add `fiExperimental` constant (value 19).
  - Update `isPresetsField()` to return true for `fiExperimental`.
  - Add `experimentalPresets` slice with `true` and `false` options and descriptions.
- [ ] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - In `NewModel()`, initialize `inputs[fiExperimental]` with placeholder "false" and value "false".
  - In `fieldLabels`, add `"Experimental Mode"` at index 19.
  - In `fieldHelpText`, add description at index 19.
  - In `viewConfig()`, add a visual separator `"── Advanced Settings ──"` before advanced fields and add a case for `fiExperimental` to render the preset pane.
- [ ] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - Handle key navigation and preset selection inside `handleConfigKey()`.
  - In `toRequest()`, parse value into `Experimental` boolean pointer.
- [ ] **Add Unit Tests and Verify Quality Gates**
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.

## Active Tasks — Flux Ultra Raw (flux-ultra-raw)

- [x] **Write Specification** — Flux Ultra Raw specification [specs/flux-ultra-raw.md](specs/flux-ultra-raw.md)
- [ ] **Update Core Engine Struct (`pkg/civit/civit.go`)**
  - Add `FluxUltraRaw` boolean field to `civit.GenerationRequest`.
- [ ] **Update Constants and Preset Config (`internal/ui/ui.go`)**
  - Add `fiFluxUltraRaw` constant (value 20).
  - Update `isPresetsField()` to return true for `fiFluxUltraRaw`.
  - Add `fluxUltraRawPresets` slice with `true`/`false`.
- [ ] **Initialize and Render Form Field (`internal/ui/ui.go`)**
  - Initialize input in `NewModel()` with placeholder "false" and value "false".
  - In `fieldLabels`, add `"Flux Ultra Raw"` at index 20.
  - In `fieldHelpText`, add description at index 20.
  - Update `isFieldVisible()` to hide `fiFluxUltraRaw` unless `fluxMode` is set to Ultra.
  - In `viewConfig()`, add case to render presets.
- [ ] **Handle User Input and Mapping (`internal/ui/ui.go`)**
  - Handle key navigation and preset selection in `handleConfigKey()`.
  - In `toRequest()`, parse value into `FluxUltraRaw` boolean pointer.
- [ ] **Add Unit Tests and Verify Quality Gates**
  - Run quality gates: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.### Bugfix

- [x] **BUGFIX: model → baseModel JSON key** — `pkg/civit/civit.go` line 52 sends `json:"model"` but schema expects `json:"baseModel"`. Fixed in commit.

## Active Tasks — Dynamic Visibility Blockers

- [ ] **Fix Custom AIR Substring Matching**
  - Update `currentEcosystem()` in `internal/ui/ui.go` to match `"flux"` (without `"flux2"`) as `"flux1"` to prevent custom Flux 1 models from falling back to `"sdxl"`.
- [ ] **Implement Active Sampler/Scheduler Adjustment**
  - Add `adjustFieldsForEcosystem()` in `internal/ui/ui.go` to check and auto-reset `fiSampler` and `fiScheduler` to valid options if they become invalid after switching base models.
  - Reset `fiFluxMode` and `fiFluxUltraRaw` when switching away from a `"flux1"` model.
- [ ] **Fix Navigation Loops and Stuck Focus**
  - Add an iteration limit/exit condition to Tab/Shift+Tab loops in `handleConfigKey()` in `internal/ui/ui.go` to prevent infinite loops if all fields are hidden.
  - Add a refocus handler to automatically shift focus to the next visible field if the currently active input is hidden by a model switch.

## Completed Tasks

- [x] **Add Sampler Parameter and Range Validation** — *Completed by Tom on 2026-07-12*
  - All subtasks complete. Additionally: input masking on CFG Scale (keystroke-level max-7.0 enforcement), resolution raised to 4K (4096), quantity default 4.
  - **API-aligned pass (Grok research):** Steps capped at 100 (API Zod schema: max(100)), quantity capped at 20 (API: max(20)), seed bounded 1–4294967295 (API: uint32 max). CFG kept at 7.0 per operator directive despite API allowing 30.
