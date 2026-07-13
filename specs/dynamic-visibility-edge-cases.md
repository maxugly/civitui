# Dynamic UI Field Visibility: Edge Cases Audit

This document outlines the findings of the edge cases audit performed on the dynamic field visibility implementation in `civitui`.

---

## Findings & Analysis

### 1. Model Switching: Flux Ultra Raw Leak
* **Severity**: Minor
* **What Happens**: 
  When a user switches from a Flux1 model (with `fluxMode` set to `"Ultra"`) to a non-Flux model like SDXL, the `fiFluxMode` field is hidden. However, the value of the field remains set to the Ultra URN (`"urn:air:flux1:checkpoint:civitai:618692@1088507"`).
  Because `isFieldVisible(fiFluxUltraRaw)` only checks if `inputs[fiFluxMode].Value() == "urn:air:flux1:checkpoint:civitai:618692@1088507"`, it returns `true` even when the current ecosystem is `"sdxl"`. As a result, the `Flux Ultra Raw` toggle remains visible on the UI for non-Flux models.
* **What Should Happen**: 
  The `fiFluxUltraRaw` field must only be visible when the active ecosystem is `"flux1"` and `fluxMode` is set to `"Ultra"`.
* **Recommended Fix**: 
  Update the visibility condition for `fiFluxUltraRaw` in `isFieldVisible()` to explicitly verify that the current ecosystem is `"flux1"`:
  ```go
  case fiFluxUltraRaw:
  	return m.currentEcosystem() == "flux1" && m.inputs[fiFluxMode].Value() == "urn:air:flux1:checkpoint:civitai:618692@1088507"
  ```

---

### 2. Custom AIR Strings: Fallback Parsing for General "flux" Checkpoints
* **Severity**: Blocker
* **What Happens**: 
  `currentEcosystem()` performs substring checks for `"flux1"`, `"flux2"`, and `"klein"`. However, if a user enters a custom AIR containing just `"flux"` (e.g. `air:flux-dev:checkpoint:civitai:123@456` or `air:flux-schnell:...`), it fails all Flux checks and falls back to `"sdxl"`.
  This disables Flux-specific behaviors, exposes invalid samplers, uses SDXL CFG scale limits (up to 30.0), and skips setting the correct request parameters.
* **What Should Happen**: 
  Custom AIR strings containing `"flux"` (without `"flux2"`) should resolve to the `"flux1"` ecosystem.
* **Recommended Fix**: 
  Update the parsing fallback in ecosystem resolution to check for `"flux"`:
  ```go
  if strings.Contains(valLower, "flux2") || strings.Contains(valLower, "klein") {
  	return "flux2"
  }
  if strings.Contains(valLower, "flux") || strings.Contains(valLower, "flux1") {
  	return "flux1"
  }
  ```

---

### 3. Sampler/Scheduler Edge Cases: Stale Value Persistence
* **Severity**: Blocker
* **What Happens**: 
  When switching from SDXL (with a sampler like `"DPM++ 2M Karras"`) to a Flux2 model (which only supports Euler, Heun, DPM++ 2M, LCM) or a ZImage model, the sampler field remains visible (since multiple samplers exist for the new model).
  However, the value of the sampler field is not validated or updated, so it remains `"DPM++ 2M Karras"`.
  When the user submits, `validateNumericFields()` passes, and the stale invalid sampler is serialized and sent to the API, causing a generation failure. The same issue applies to the `scheduler` parameter.
* **What Should Happen**: 
  When the ecosystem changes, active sampler and scheduler values must be checked against the new model's valid presets. If the current value is invalid, it must be reset to a default valid value (e.g. the first available filtered preset).
* **Recommended Fix**: 
  Implement an `adjustFieldsForEcosystem()` method on `Model` and call it whenever the model changes (e.g. in `handleConfigKey` when a preset is chosen). The method should:
  1. Verify if the current sampler value is valid in `filteredSamplerPresets()`. If not, reset it to the first valid preset.
  2. Verify if the current scheduler value is valid in `filteredSchedulerPresets()`. If not, reset it to the first valid preset.
  3. Reset `fluxMode` and `fluxUltraRaw` values to their defaults if the ecosystem is not `"flux1"`.

---

### 4. CFG Scale Validation: Keystroke vs. Submit-Time Inconsistency
* **Severity**: Minor
* **What Happens**: 
  Real-time keystroke validation in `NewModel()` uses a simple string match (`strings.Contains(modelVal, "flux")`) to cap CFG scale at `7.0`.
  Submit-time validation in `validateNumericFields()` uses `m.currentEcosystem()` which also matches `"klein"` (capping at `7.0`).
  If a user types a custom Flux2 model containing `"klein"` but not `"flux"`, they can type up to `30.0` at the keystroke level, but will get blocked at submit time with a validation error.
* **What Should Happen**: 
  Keystroke and submit-time validations must use identical ecosystem-matching logic to prevent user confusion.
* **Recommended Fix**: 
  Extract the fallback ecosystem matching logic into a package-level helper `ecosystemForModel(val string) string`. Use this helper in both `currentEcosystem()` and the `inputs[fiCFGScale].Validate` closure.

---

### 5. isPresetsField vs. isFieldVisible: Sampler Preset Pane
* **Severity**: None (Inquiry Only)
* **What Happens**: 
  The codebase has `fiSampler` included in `isPresetsField()` inside [ui.go:107](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L107).
  Thus, there is no code inconsistency; Right Arrow triggers the presets pane, and Enter selects the preset correctly.
* **What Should Happen**: 
  No change required as it functions correctly according to the current code.

---

### 6. Navigation: Infinite Loop and Lost Focus
* **Severity**: Blocker
* **What Happens**: 
  1. **Infinite Loop**: If all fields in the form are somehow hidden, pressing Tab or Shift+Tab will cause the scanning loop to run forever, freezing the terminal UI and consuming 100% CPU.
  2. **Lost Focus**: If a user is focused on a field (e.g., `fiFluxMode`) and then changes the model to SDXL, `fiFluxMode` becomes hidden. However, `m.activeInput` remains pointing to `fiFluxMode`. The cursor disappears, and pressing Right Arrow shows the presets pane for the hidden field.
* **What Should Happen**: 
  1. Navigation loops must have safety exits.
  2. Focus must shift to the first visible field if the current active field is hidden by a state change.
* **Recommended Fix**: 
  1. Add an iteration limit to the focus loops:
     ```go
     startInput := m.activeInput
     for {
     	m.activeInput = (m.activeInput + 1) % numFormFields
     	if m.isFieldVisible(m.activeInput) {
     		break
     	}
     	if m.activeInput == startInput {
     		break
     	}
     }
     ```
  2. Call a refocus helper (`refocusIfHidden()`) after model changes:
     ```go
     func (m *Model) refocusIfHidden() {
     	if !m.isFieldVisible(m.activeInput) {
     		m.inputs[m.activeInput].Blur()
     		for i := 0; i < numFormFields; i++ {
     			m.activeInput = (m.activeInput + 1) % numFormFields
     			if m.isFieldVisible(m.activeInput) {
     				break
     			}
     		}
     		m.inputs[m.activeInput].Focus()
     	}
     }
     ```

---

### 7. toRequest Cleanup: JSON Serialization
* **Severity**: None (Correct behavior confirmed)
* **What Happens**: 
  `fluxMode` and `sampler` return `""` when hidden. Their JSON tags have `omitempty`, meaning they are correctly omitted from serialization. Stale values stored in inputs do not bleed onto the wire.
* **What Should Happen**: 
  No action required.

---

### 8. Initial State: Default Sampler Inconsistency
* **Severity**: Minor
* **What Happens**: 
  `NewModel()` initializes `inputs[fiSampler]` to `"Euler a"`. Currently, the default model is Flux1, which hides the sampler field (preventing invalid transmission). However, if the default model is changed to ZImage or Flux2, `"Euler a"` will be visible but invalid.
* **What Should Happen**: 
  `NewModel()` should initialize the sampler and scheduler based on the initial model's ecosystem.
* **Recommended Fix**: 
  Query the initial model's ecosystem during initialization and set the default values of `fiSampler` and `fiScheduler` to their first available presets if they are not hidden.
