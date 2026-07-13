# Image upscaling

The `imageUpscaler` step type takes an image and returns a higher-resolution version. The **upscaler model** sets the scale factor per pass (a "4×" model like [4x-Remacri](https://civitai.com/models/147759/remacri?modelVersionId=164821) — the default — applies a 4× enlargement in one run). You can then run the same model up to 3 times in one step via `numberOfRepeats` for compounding scale.

Common uses:

* Finishing step after image generation (chain `imageGen` → `imageUpscaler`)
* Rescuing low-resolution assets
* Preparing images for print / large-format display

## Prerequisites

* A Civitai orchestration token ([Quick start → Prerequisites](/orchestration/guide/getting-started#prerequisites))
* A source image — URL, data URL, or Base64 string

## The simplest request

Use the per-recipe endpoint when you're just upscaling one image and don't need webhooks or multi-step chaining:

```http
POST https://orchestration.civitai.com/v2/consumer/recipes/imageUpscaler?wait=60
Authorization: Bearer <your-token>
Content-Type: application/json

{
  "image": "https://image.civitai.com/.../00890-23.jpeg"
}
```

That's it — the defaults run the 4x-Remacri upscaler once. The response is a full [`Workflow`](/orchestration/reference/operations/GetWorkflow) whose single step carries the upscaled blob.

## Via the generic workflow endpoint

Equivalent request through [`SubmitWorkflow`](/orchestration/reference/operations/SubmitWorkflow) — use this path when you need webhooks, tags, or to chain with other steps:

```http
POST https://orchestration.civitai.com/v2/consumer/workflows?wait=60
Authorization: Bearer <your-token>
Content-Type: application/json

{
  "steps": [{
    "$type": "imageUpscaler",
    "input": {
      "image": "https://image.civitai.com/.../00890-23.jpeg",
      "numberOfRepeats": 2
    }
  }]
}
```

## Input fields

See the [`ImageUpscalerInput` schema](/orchestration/reference/operations/InvokeImageUpscalerStepTemplate) for the full definition.

| Field | Required | Default | Notes |
|-------|----------|---------|-------|
| `image` | ✅ | — | URL, data URL, or raw Base64 string. Civitai CDN URLs work directly. |
| `model` | | [`4x-Remacri`](https://civitai.com/models/147759/remacri?modelVersionId=164821) (`urn:air:other:upscaler:civitai:147759@164821`) | AIR URN of the upscaler model. The model's own spec determines the **scale factor per pass**. |
| `numberOfRepeats` | | `1` | `1`–`3`. How many times to run the model end-to-end. Total scale ≈ `(model_scale) ^ numberOfRepeats`. |

### Picking a model

Two dimensions to consider:

**Content type** — different upscaler families handle different content best:

* **Photographic / real-world images** — general-purpose upscalers (ESRGAN derivatives like 4x-Remacri, the default).
* **Anime / illustrated art** — anime-tuned upscalers produce cleaner line work.
* **Faces / portraits** — face-restoration–aware upscalers reduce artifacts around features.

**Scale factor** — upscaler models advertise their scale in the name (`2x-…`, `4x-…`, `8x-…`). This is typically the multiplication factor per pass — a `4x` model on a 1024×1024 input produces 4096×4096 output in a single run. Combined with `numberOfRepeats: 2`, a 4× model produces a 16× total enlargement.

Browse [Civitai's upscaler catalog](https://civitai.com/models?tag=upscaler) and pass the AIR URN you want. Leave `model` unset to accept 4x-Remacri.

## Chaining: generate then upscale

One of the most common two-step workflows — produce at native resolution, then upscale with a single submission:

```json
{
  "steps": [
    {
      "$type": "imageGen",
      "name": "hero",
      "input": {
        "engine": "flux2",
        "model": "klein",
        "operation": "createImage",
        "modelVersion": "4b",
        "prompt": "A cat astronaut floating through neon space",
        "width": 1024,
        "height": 1024
      }
    },
    {
      "$type": "imageUpscaler",
      "name": "hero-4k",
      "input": {
        "image": {
          "$ref": "hero",
          "path": "output.images[0].url"
        },
        "numberOfRepeats": 1
      }
    }
  ]
}
```

The `{ "$ref": "hero", "path": "output.images[0].url" }` reference creates a dependency — `hero-4k` doesn't start until `hero` succeeds, and the upscaler's `image` field is filled in with the generated image's signed URL at runtime. See [Workflows → Dependencies](/orchestration/guide/workflows#dependencies-parallelism) for the full reference syntax.

## Targeting an exact resolution

Upscalers only know how to multiply (4× per pass with the default model). If you need a specific output width — say, 1920 px wide for a hero image — chain a `convertImage` step after the upscaler to downscale to your exact target.

```json
{
  "steps": [
    {
      "$type": "imageGen",
      "name": "hero",
      "input": {
        "engine": "flux2",
        "model": "klein",
        "operation": "createImage",
        "modelVersion": "4b",
        "prompt": "A cat astronaut floating through neon space",
        "width": 1024,
        "height": 1024
      }
    },
    {
      "$type": "imageUpscaler",
      "name": "upscaled",
      "input": {
        "image": { "$ref": "hero", "path": "output.images[0].url" },
        "numberOfRepeats": 1
      }
    },
    {
      "$type": "convertImage",
      "name": "hero-1920",
      "input": {
        "image": { "$ref": "upscaled", "path": "output.blob.url" },
        "transforms": [
          { "type": "resize", "targetWidth": 1920 }
        ],
        "output": {
          "format": "webp",
          "quality": 85,
          "lossless": false,
          "hideMetadata": true
        }
      }
    }
  ]
}
```

What happens at runtime:

1. **`hero`** generates a 1024×1024 image.
2. **`upscaled`** runs 4x-Remacri once → 4096×4096 (intermediate, oversized).
3. **`hero-1920`** downsamples to 1920 px wide (height auto-computed from aspect ratio = 1920×1920 here) and re-encodes as WebP at quality 85.

`ResizeTransform` keeps aspect ratio — set only `targetWidth` (1–4096). For other format / quality knobs see the [`ConvertImageInput` schema](/orchestration/reference/operations/InvokeConvertImageStepTemplate); supported `format` values are `jpeg`, `png`, `webp`, `gif`.

## Reading the result

A successful `imageUpscaler` step emits a single upscaled image blob:

```json
{
  "status": "succeeded",
  "steps": [{
    "name": "0",
    "$type": "imageUpscaler",
    "status": "succeeded",
    "output": {
      "blob": { "id": "blob_...", "url": "https://.../signed.png" }
    }
  }]
}
```

Note: `imageUpscaler` output is `blob` (singular), not `blobs[]` — the step always returns exactly one image.

Blob URLs are signed and expire — refetch the workflow or call [`GetBlob`](/orchestration/reference/operations/GetBlob) to get a fresh URL.

## Cost

Billed in Buzz on the workflow's `transactions`. Use `whatif=true` for an exact preview; see [Payments (Buzz)](/orchestration/guide/submitting-work#payments-buzz) for currency selection.

Cost scales with input pixel area and the total scale factor applied by `numberOfRepeats`:

```
inputMegapixels = width × height / 1 000 000
scale           = 2 ^ numberOfRepeats      // default 4x-Remacri, 2 per pass
total           = max(1, ceil(inputMegapixels)) × scale
```

| Shape | Buzz |
|-------|------|
| 512×512 input, `numberOfRepeats: 1` | **2** |
| 1024×1024 input, `numberOfRepeats: 1` | **4** |
| 2048×2048 input, `numberOfRepeats: 1` | **10** |
| 1024×1024 input, `numberOfRepeats: 2` | **8** |
| 1024×1024 input, `numberOfRepeats: 3` | **16** |

Upscaling is one of the cheapest operations exposed — even aggressive stacked passes on a 2-megapixel source land under a few dozen Buzz. The practical ceiling is usually the [upscaler's content-size cap](#runtime), not cost.

## Runtime

A single pass (`numberOfRepeats: 1`) with the default 4x-Remacri on a ~1-megapixel input usually completes in 5–15 s and fits inside `wait=60`. Multiple repeats stack both runtime *and* output size — `numberOfRepeats: 3` with a 4× model produces a 64× enlargement, which will exceed the [100-second request timeout](/orchestration/guide/getting-started#_3-poll-if-you-didn-t-wait-inline) and is rarely what you actually want. Use `wait=0` plus webhooks/polling for anything beyond one pass, and keep the total scale in mind before cranking `numberOfRepeats`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `400` with "image could not be loaded" | URL not publicly reachable, or data URL malformed | Make sure the URL is fetchable without auth; re-encode the Base64 payload. |
| `400` with "numberOfRepeats out of range" | Value outside `1`–`3` | Clamp client-side. |
| Output looks soft / painterly | Default model mismatch for this content | Specify a content-appropriate `model` AIR (anime-tuned for illustration, face-aware for portraits, etc.). |
| Output has halos or ringing | `numberOfRepeats` too aggressive for the source | Drop to a single pass; or pre-denoise the source. |
| Step `failed`, `reason = "blocked"` | Source image hit content moderation | Don't retry the same input — see [Errors & retries → Step-level failures](/orchestration/guide/errors-and-retries#step-level-failures). |

## Related

* [`InvokeImageUpscalerStepTemplate`](/orchestration/reference/operations/InvokeImageUpscalerStepTemplate) — the per-recipe endpoint
* [Endpoint OpenAPI spec](https://orchestration.civitai.com/v2/consumer/recipes/imageUpscaler/openapi.yaml) — standalone OpenAPI 3.1 YAML for this endpoint, ready to import into Postman / Insomnia / OpenAPI Generator
* [`SubmitWorkflow`](/orchestration/reference/operations/SubmitWorkflow) — generic path for chaining
* [Video upscaling](./video-upscaler) — the `videoUpscaler` equivalent for video
* [Workflows → Dependencies](/orchestration/guide/workflows#dependencies-parallelism) — how the `@step.output.*` references work
