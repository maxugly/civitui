# CivitAI Orchestrator API — textToImage Parameters Audit

> **Generated**: 2026-07-12
> **Source schema**: `src/server/schema/orchestrator/textToImage.schema.ts` (civitai/civitai, main branch)
> **Supporting files**: `src/server/common/constants.ts`, `src/shared/constants/generation.constants.ts`, `src/shared/orchestrator/ImageGen/flux2-klein.config.ts`, `src/shared/orchestrator/ImageGen/zImage.config.ts`, `src/server/orchestrator/infrastructure/base.schema.ts`

## Currently Exposed (10 fields)

These are the parameters civitui already surfaces in its config form (`internal/ui/ui.go`):

| # | Parameter | civitui Field | Schema Key | Type |
|---|-----------|--------------|------------|------|
| 1 | Prompt | `fiPrompt` | `prompt` | string |
| 2 | Negative Prompt | `fiNegativePrompt` | `negativePrompt` | string (optional) |
| 3 | Model | `fiModel` | `baseModel` | string |
| 4 | Sampler | `fiSampler` | `sampler` | string |
| 5 | Width | `fiWidth` | `width` | number |
| 6 | Height | `fiHeight` | `height` | number |
| 7 | Steps | `fiSteps` | `steps` | number (1–100) |
| 8 | CFG Scale | `fiCFGScale` | `cfgScale` | number (1.0–7.0) |
| 9 | Quantity | `fiQuantity` | `quantity` | number (1–20) |
| 10 | Seed | `fiSeed` | `seed` | number (1–4294967295, nullable) |

> **Note**: civitui sends `model` as the JSON key but the schema uses `baseModel`. The CivitAI API may accept both, but this should be verified and corrected to `baseModel` to match the schema.

---

## Missing Parameters — Complete Audit

Below are all 24 parameters from the `textToImageParamsSchema` Zod schema that are **not currently exposed** in civitui's UI.

---

### 1. `scheduler`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` (enum) |
| **Valid values** | `"EulerA"`, `"Euler"`, `"Heun"`, `"DPM2"`, `"DPM2A"`, `"DPM2SA"`, `"DPM2M"`, `"DPMSDE"`, `"DPMFast"`, `"DPMAdaptive"`, `"LMSKarras"`, `"DPM2Karras"`, `"DPM2AKarras"`, `"DPM2SAKarras"`, `"DPM2MKarras"`, `"DPMSDEKarras"`, `"DDIM"`, `"PLMS"`, `"UniPC"`, `"LCM"`, `"DDPM"`, `"DEIS"`, `"LMS"` |
| **Default** | Falls back to `"EulerA"` |
| **Optional** | Yes |
| **Zod definition** | `z.nativeEnum(Scheduler).optional()` |

> [!WARNING]
> **DEPRECATED PRESETS (simple, discrete, karras, exponential, ays)**:
> Historically documented values like `"simple"`, `"discrete"`, `"karras"`, `"exponential"`, and `"ays"` are completely incorrect. The live CivitAI API rejects all of them because it uses **sampler algorithm names** (PascalCase/camelCase) for the `scheduler` parameter.

**What it does**: Controls the noise schedule/sampler algorithm for the diffusion process. Different schedulers affect convergence speed, rendering steps required, and final image quality.

**Model-specific constraints**:
- The 23 sampler-based values are accepted by the orchestrator API.
- Individual generation backends may fall back to default behavior if a specific model is incompatible.

**UX Recommendation**: **Dropdown/preset list** of recommended values (e.g., `EulerA`, `Euler`, `Heun`, `DPM2M`, `DPM2MKarras`, `LCM`, `UniPC`). Default: `EulerA`.

---

### 2. `clipSkip`

| Attribute | Value |
|-----------|-------|
| **Type** | `number` (integer) |
| **Valid range** | 1–3 (per `generation.maxValues.clipSkip`) |
| **Default** | `2` (per `generation.defaultValues.clipSkip`), forced to `2` for SDXL/Pony models |
| **Optional** | Yes |
| **Zod definition** | `z.coerce.number().optional()` |

**What it does**: Skips the last N layers of the CLIP text encoder. Higher values reduce prompt adherence but can improve composition in some models. Only relevant for SD1/SDXL-based models — Flux, ZImage, and other newer architectures ignore it.

**UX Recommendation**: **Number input (1–3)** with sensible default of 2. Hide for Flux/ZImage models where it has no effect. Low priority — advanced users only.

---

### 3. `resolution`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | `"480p"`, `"720p"`, `"1080p"`, `"2K"`, `"4K"` (from `aspectRatioDimensions` resolution tiers) |
| **Default** | `"2k"` (from `generation.defaultValues.resolution`) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: Sets a resolution tier that auto-determines width×height when combined with an `aspectRatio`. Used primarily by the video generation path, but also recognized by some image models. The 2K/4K tiers drive Seedream-compatible dimension mapping.

**UX Recommendation**: **Dropdown (480p / 720p / 1080p / 2K / 4K)**. This is mainly useful when paired with `aspectRatio` as a shortcut instead of manually entering width/height. If the user has already set explicit width/height, this is redundant. Consider exposing as a "quick resolution" shortcut that fills width/height automatically.

---

### 4. `draft`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | `false` |
| **Optional** | No (has default) |
| **Zod definition** | `z.boolean().default(false)` |

**What it does**: Enables draft/fast-preview mode. When `true`, the orchestrator may inject draft LoRA resources (SD1: id 424706, SDXL: id 391999) that radically reduce steps (to 6–8) and set CFG to 1 for fast, low-quality preview generations. This is a "speed over quality" toggle.

**UX Recommendation**: **Toggle** — label "Draft Mode (fast preview)". Clear for users: faster but lower quality. Useful for rapid iteration.

---

### 5. `aspectRatio`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | Depends on model. Common: `"1:1"`, `"3:2"`, `"2:3"`, `"16:9"`, `"9:16"`, `"4:3"`, `"3:4"`. Flux Ultra/Kontext: also `"21:9"`, `"9:21"`. |
| **Default** | `"1:1"` (from `generation.defaultValues.aspectRatio`) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: A convenience parameter — if set, the server resolves it to width×height based on the model's aspect ratio buckets. If both `aspectRatio` and explicit `width`/`height` are provided, the CivitAI web form auto-computes aspectRatio from the dimensions (`sanitizeTextToImageParams` in `generation.constants.ts`). Models have different valid aspect ratios based on their training buckets.

**Model-specific aspect ratios**:
- SD1: 1:1 (512×512), 3:2 (768×512), 2:3 (512×768)
- SDXL/Flux/Pony/NoobAI/Illustrious/HiDream: 1:1 (1024×1024), 3:2 (1216×832), 2:3 (832×1216)
- Flux Ultra: 21:9, 16:9, 4:3, 1:1, 3:4, 9:16, 9:21
- Qwen: 16:9 (1664×928), 4:3 (1472×1104), 1:1 (1328×1328), 3:4 (1104×1472), 9:16 (928×1664)
- OpenAI: 1:1 (1024×1024), Landscape (1536×1024), Portrait (1024×1536)

**UX Recommendation**: **Dropdown with model-aware presets**. This is a high-value add — it replaces manual width/height entry with a clean aspect ratio selector. The dropdown should update based on the selected model. When the user picks an aspect ratio, auto-fill width/height. This could replace or complement the current free-text width/height fields.

---

### 6. `fluxUltraAspectRatio`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` (index into `fluxUltraAspectRatios` array) |
| **Valid values** | `"0"` through `"6"` (indices into the 7-item array) |
| **Default** | `"4"` (index 4 = 1:1 / 2048×2048) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: Specific to Flux Ultra mode. Selects one of 7 higher-resolution aspect ratios:
- 0: 21:9 (3136×1344)
- 1: 16:9 (2752×1536)
- 2: 4:3 (2368×1792)
- 3: 1:1 (2048×2048)
- 4: 3:4 (1792×2368) ← default
- 5: 9:16 (1536×2752)
- 6: 9:21 (1344×3136)

The CivitAI web UI resolves this automatically from width/height via `getClosestFluxUltraAspectRatio()`.

**UX Recommendation**: **Dropdown for Flux Ultra models only** — 7 preset options with resolution labels (e.g., "21:9 — 3136×1344"). Auto-resolve from `aspectRatio` if that's also set. **Not worth exposing as a standalone field** — it's redundant with `aspectRatio` for 99% of users. The server already computes it from width/height.

---

### 7. `denoise`

| Attribute | Value |
|-----------|-------|
| **Type** | `number` (float) |
| **Valid range** | 0.0–1.0 |
| **Default** | `0.4` (from `generation.defaultValues.denoise`) |
| **Optional** | Yes |
| **Zod definition** | `z.number().max(1).optional()` |

**What it does**: Denoising strength for image-to-image (img2img) workflows. Controls how much the source image is altered — 0.0 means "don't change anything", 1.0 means "completely regenerate". Only meaningful when `sourceImage` or `images` is provided (img2img mode).

**UX Recommendation**: **Slider or number input (0.0–1.0, step 0.05)**. Only show when a source image is attached (img2img mode). Label: "Denoising Strength". Default: 0.4. For a TUI, a number input with validation is fine.

---

### 8. `upscaleWidth`

| Attribute | Value |
|-----------|-------|
| **Type** | `number` (integer) |
| **Valid range** | 320–3840 (per `minDownscaleSize` and `maxUpscaleSize`) |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.number().optional()` |

**What it does**: Target width for upscaling workflows. When used with `upscaleHeight`, tells the orchestrator to upscale the output to these dimensions. Only available when the workflow supports upscaling.

**UX Recommendation**: **Number input, paired with upscaleHeight**. Only show for models/workflows that support upscaling. Pair with a "2x", "4x" quick-select button that auto-computes dimensions. For a TUI, a simple integer field is fine.

---

### 9. `upscaleHeight`

| Attribute | Value |
|-----------|-------|
| **Type** | `number` (integer) |
| **Valid range** | 320–3840 (per `minDownscaleSize` and `maxUpscaleSize`) |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.number().optional()` |

**What it does**: Target height for upscaling. See `upscaleWidth`.

**UX Recommendation**: **Number input, paired with upscaleWidth**. Same UX treatment as above.

---

### 10. `workflow`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | `"txt2img"` (default), potentially `"img2img"`, `"upscale"`, etc. |
| **Default** | `"txt2img"` |
| **Optional** | No (has default) |
| **Zod definition** | `z.string().default('txt2img')` |

**What it does**: Identifies the workflow type. This is an internal routing key — the orchestrator uses it to select the correct processing pipeline. civitui is exclusively a txt2img tool, so this should always be `"txt2img"`.

**UX Recommendation**: **Do not expose**. Always send `"txt2img"`. If civitui later supports img2img or upscaling, it should set this automatically based on the workflow mode, not as a user-facing field.

---

### 11. `fluxMode`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | `"urn:air:flux1:checkpoint:civitai:618692@699279"` (Draft), `"urn:air:flux1:checkpoint:civitai:618692@691639"` (Standard), `"urn:air:flux1:checkpoint:civitai:618692@2068000"` (Krea), `"urn:air:flux1:checkpoint:civitai:618692@922358"` (Pro 1.1), `"urn:air:flux1:checkpoint:civitai:618692@1088507"` (Ultra) |
| **Default** | `"urn:air:flux1:checkpoint:civitai:618692@691639"` (Standard) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: Selects the Flux model variant/mode for the Flux1 ecosystem. This is a Flux-specific parameter that chooses between Draft (fast, cheap), Standard (balanced), Krea, Pro 1.1 (higher quality), and Ultra (highest quality, higher resolution). The values are full AIR URNs.

**UX Recommendation**: **Dropdown with friendly labels** — Draft / Standard / Krea / Pro 1.1 / Ultra. Only show when the selected base model is Flux1. Map labels to URNs internally. High value for Flux users.

---

### 12. `fluxUltraRaw`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | `false` |
| **Optional** | Yes |
| **Zod definition** | `z.boolean().optional()` |

**What it does**: Flux Ultra "raw" mode. When enabled, generates less-refined, more "raw" outputs from Flux Ultra — useful for getting more photorealistic or less "aesthetic-ized" results.

**UX Recommendation**: **Toggle** — "Flux Ultra Raw Mode". Only show when `fluxMode` is Ultra. Low priority, but simple to add.

---

### 13. `experimental`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.boolean().optional()` |

**What it does**: Enables experimental/bleeding-edge features for the generation. In practice, this maps to the `enhancedCompatibility` toggle for SDCPP-supported ecosystems. Supported models: SD1, SDXL, Pony, Illustrious, NoobAI, Flux1, FluxKrea.

**UX Recommendation**: **Toggle** — "Experimental Mode". Place behind an advanced section with clear warning text. Good for power users.

---

### 14. `engine`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | `"flux2"`, `"zImage"`, `"sdcpp"`, possibly others |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: Overrides the generation engine. The orchestrator normally auto-selects the engine based on the model/resource, but this allows explicit override. Flux2Klein uses `"flux2"`, ZImage uses `"sdcpp"` with `ecosystem: "zImage"`.

**UX Recommendation**: **Do not expose**. Auto-selected by the server from the model resource. Manual override could break generation. Only useful for debugging.

---

### 15. `priority`

| Attribute | Value |
|-----------|-------|
| **Type** | `enum Priority` (from `@civitai/client`) |
| **Valid values** | Likely `"low"`, `"normal"`, `"high"` (unable to verify exact enum — imported from external npm package `@civitai/client`. Default and only observed value is `"low"`.) |
| **Default** | `"low"` |
| **Optional** | No (has default) |
| **Zod definition** | `z.enum(Priority).default('low')` |

**What it does**: Sets the queue priority for the generation job. Higher priority may mean faster processing but likely higher Buzz cost. The CivitAI web UI defaults to `"low"`.

**UX Recommendation**: **Dropdown** — Low / Normal / High. Place in an "Advanced" section. Most users won't need to change this. Important: verify the exact Priority enum values before implementing.

---

### 16. `sourceImage`

| Attribute | Value |
|-----------|-------|
| **Type** | `object` (`{ url: string, width: number, height: number, upscaleWidth?: number, upscaleHeight?: number }`) |
| **Valid values** | URL must start with `https://orchestration` and contain `.civitai.com`, or contain `.civitai.red`, or start with `https://civitai.` (i.e., must be a CivitAI-hosted image) |
| **Default** | `null` |
| **Optional** | Yes (nullable) |
| **Zod definition** | `defaultCatch(sourceImageSchema.nullable(), null)` |

**What it does**: Provides a source image for image-to-image (img2img) generation. The URL must be a CivitAI-hosted image (from a previous generation, uploaded to CivitAI, etc.). civitui currently only supports txt2img, so this isn't needed yet.

**UX Recommendation**: **Do not expose yet**. This is for img2img workflows. If civitui adds img2img support later, this would be a file upload or URL input field with CivitAI URL validation.

---

### 17. `images`

| Attribute | Value |
|-----------|-------|
| **Type** | `array of sourceImage` |
| **Default** | `null` |
| **Optional** | Yes (nullable) |
| **Zod definition** | `defaultCatch(sourceImageSchema.array().nullable(), null)` |

**What it does**: Multiple source images for multi-image input workflows (e.g., Kontext with multiple reference images, or edit modes). Same URL restrictions as `sourceImage`.

**UX Recommendation**: **Do not expose yet**. Same rationale as `sourceImage` — for future img2img/multi-image workflows.

---

### 18. `disablePoi`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | `false` |
| **Optional** | No (has default) |
| **Zod definition** | `z.boolean().default(false)` |

**What it does**: Disables Person of Interest (POI) detection. When `false` (default), the system checks generated images against a database of public figures and may block or flag matches. Set to `true` to skip this check (useful when intentionally generating images of celebrities/public figures with proper rights).

**UX Recommendation**: **Toggle** — "Disable Person-of-Interest Detection". Place in an advanced section with a note about responsible use. Only relevant for photorealistic models.

---

### 19. `openAIQuality`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` (enum) |
| **Valid values** | `"auto"`, `"high"`, `"medium"`, `"low"` |
| **Default** | `"medium"` (from `generation.defaultValues.openAIQuality`) |
| **Optional** | Yes |
| **Zod definition** | `z.enum(['auto', 'high', 'medium', 'low']).optional()` |

**What it does**: Quality setting for OpenAI-backed generation models (e.g., DALL·E via OpenAI's API on CivitAI). Controls the quality/speed tradeoff.

**UX Recommendation**: **Dropdown** — Auto / High / Medium / Low. Only show when an OpenAI-based model is selected. For most users, "auto" is the right answer.

---

### 20. `openAITransparentBackground`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.boolean().optional()` |

**What it does**: Requests a transparent background (PNG with alpha channel) from OpenAI-backed models. Only supported by specific OpenAI image models.

**UX Recommendation**: **Toggle** — "Transparent Background". Only show for OpenAI models. Clear label: "Generates PNG with transparent background where possible."

---

### 21. `process`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` |
| **Valid values** | `"txt2img"`, `"img2img"`, `"upscale"`, `"bg-removal"` (image); `"txt2vid"`, `"img2vid"`, `"vid-upscale"`, `"vid-interpolation"`, `"vid-enhancement"` (video); `"txt2music"` (audio) |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.string().optional()` |

**What it does**: Tags the generation with a process type for filtering/organization. For civitui's txt2img use case, this would always be `"txt2img"`. More relevant for video/audio/multi-modal workflows.

**UX Recommendation**: **Do not expose**. Auto-set to `"txt2img"`. Only becomes relevant if civitui adds other workflow types (img2img, upscale, etc.) — in that case, set automatically based on workflow mode.

---

### 22. `enhancedCompatibility`

| Attribute | Value |
|-----------|-------|
| **Type** | `boolean` |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `z.boolean().optional()` |

**What it does**: Toggles enhanced compatibility mode for the SDCPP engine. This is closely related to `experimental` — both route generations through the sdcpp engine for SDCPP-supported ecosystems (SD1, SDXL, Pony, Illustrious, NoobAI, Flux1, FluxKrea, ZImageBase, ZImageTurbo, Flux2Klein variants). SDCPP ecosystems get a 2-for-1 quantity bonus.

**UX Recommendation**: **Toggle** — "Enhanced Compatibility (SDCPP)" in an advanced section. This is quite technical — most users won't need to touch it. Note: it's somewhat redundant with `experimental` for the supported models.

---

### 23. `outputFormat`

| Attribute | Value |
|-----------|-------|
| **Type** | `string` (enum) |
| **Valid values** | `"png"`, `"jpeg"` |
| **Default** | `"jpeg"` (from `generation.defaultValues.outputFormat`) |
| **Optional** | Yes |
| **Zod definition** | `z.enum(['png', 'jpeg']).optional()` |

**What it does**: Controls the output image format. JPEG is smaller and faster; PNG is lossless and supports transparency (for models that can generate it).

**UX Recommendation**: **Dropdown or toggle** — PNG / JPEG. Simple and high-value for users who care about output quality. Default: JPEG (matches CivitAI web default).

---

### 24. `transformations`

| Attribute | Value |
|-----------|-------|
| **Type** | `array of { workflow: string, params?: Record<string, unknown>, resources?: Array<Record<string, unknown>> }` |
| **Default** | None (optional) |
| **Optional** | Yes |
| **Zod definition** | `transformationSchema.array().optional()` |

**What it does**: A pipeline of post-processing transformations to apply after generation. Each transformation has a workflow name, optional params, and optional resources. This enables chaining operations like "generate → upscale → background removal" in a single workflow.

**UX Recommendation**: **Do not expose**. This is a complex chaining mechanism that would require a full pipeline editor. Far beyond the scope of a simple TUI. If civitui ever supports multi-step workflows, this would need a dedicated UI.

---

## Summary: Implementation Priority

### Tier 1 — High Value, Easy Wins

| Parameter | UX | Effort | Impact |
|-----------|-----|--------|--------|
| **aspectRatio** | Dropdown (model-aware presets) | Medium | Replaces manual width/height for most users |
| **fluxMode** | Dropdown (5 Flux variants) | Low | Unlocks Flux Ultra/Pro for power users |
| **outputFormat** | Dropdown (PNG/JPEG) | Low | Simple quality-of-life |
| **scheduler** | Dropdown (5 options) | Low | Power-user feature, hidden in Advanced |
| **draft** | Toggle | Low | Fast iteration mode |

### Tier 2 — Worth Adding, Medium Effort

| Parameter | UX | Effort | Impact |
|-----------|-----|--------|--------|
| **denoise** | Number input (0.0–1.0) | Low | Needed for future img2img support |
| **clipSkip** | Number input (1–3) | Low | Relevant for SD1/SDXL users |
| **upscaleWidth / upscaleHeight** | Number inputs (paired) | Medium | Needed for upscaling workflows |
| **experimental** | Toggle | Low | Power-user toggle |
| **fluxUltraRaw** | Toggle | Low | Niche but easy |

### Tier 3 — Low Priority or Not Worth Exposing

| Parameter | Reasoning |
|-----------|-----------|
| **resolution** | Redundant with explicit width/height + aspectRatio |
| **fluxUltraAspectRatio** | Computed automatically from width/height by server |
| **priority** | Default `"low"` is fine for most; verify enum values first |
| **disablePoi** | Very niche, legal/compliance concern |
| **openAIQuality** | Only for OpenAI models |
| **openAITransparentBackground** | Only for OpenAI models |
| **enhancedCompatibility** | Too technical; redundant with experimental |
| **workflow** | Always `"txt2img"` for civitui |
| **engine** | Auto-selected; manual override dangerous |
| **sourceImage / images** | Requires img2img support first |
| **process** | Auto-set based on workflow type |
| **transformations** | Complex pipeline editor needed |

---

## Appendices

### A. Sampler → Scheduler Compatibility

The schema validates `sampler` against all known samplers (`generationSamplers ∪ zImageSampleMethods ∪ flux2KleinSampleMethods`):

**Standard samplers** (SD/SDXL/Pony/etc.):
Euler a, Euler, LMS, Heun, DPM2, DPM2 a, DPM++ 2S a, DPM++ 2M, DPM++ SDE, DPM++ 2M SDE, DPM++ 3M SDE, DPM fast, DPM adaptive, LMS Karras, DPM2 Karras, DPM2 a Karras, DPM++ 2S a Karras, DPM++ 2M Karras, DPM++ SDE Karras, DPM++ 2M SDE Karras, DPM++ 3M SDE Karras, DPM++ 3M SDE Exponential, DDIM, PLMS, UniPC, LCM

**ZImage samplers**: euler, heun

**Flux2Klein samplers**: euler, heun, dpm++2s_a, dpm++2m, dpm++2mv2, ipndm, ipndm_v, lcm

### B. CivitAI Go Client Mismatch

`pkg/civit/civit.go` line 52 sends `model` as JSON key but the schema expects `baseModel`:
```go
Model string `json:"model"`  // should be `json:"baseModel"`
```

This may work because of API backwards compatibility, but should be fixed to match the schema.

### C. CFG Scale Range Discrepancy

civitui caps CFG Scale at 7.0 (line 352 of ui.go), but the Zod schema allows 1–30. The 7.0 cap may come from earlier API versions or UI design choices. Flux models generally use CFG 1.0–7.0, but SD/SDXL models can use higher values. Consider raising the cap to 30 to match the current API, or make it model-dependent.
