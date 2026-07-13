# Gemini image generation

The `gemini` engine exposes **Gemini 2.5 Flash Image** — the same underlying model product as Google's [`nano-banana-*`](./google) variants, but via the direct Gemini API rather than Vertex AI. Simpler input shape: no aspect-ratio or resolution picker, just prompt (+ optional reference images for edits) and a `quantity`. Uses `operation` as the discriminator, mirroring most other imageGen engines.

::: tip Gemini vs Google
If you want explicit aspect-ratio control, resolution tiers, or web-search grounding, use the [`google` engine](./google) with `model: "nano-banana-2"` — same product, richer controls. Pick `gemini` when you want the minimal shape and the direct Gemini-API semantics.
:::

## Prerequisites

* A Civitai orchestration token ([Quick start → Prerequisites](/orchestration/guide/getting-started#prerequisites))
* For `editImage`: 1–4 source images (URLs, data URLs, or Base64)

## Text-to-image (`createImage`)

```http
POST https://orchestration.civitai.com/v2/consumer/workflows?wait=60
Authorization: Bearer <your-token>
Content-Type: application/json

{
  "steps": [{
    "$type": "imageGen",
    "input": {
      "engine": "gemini",
      "model": "2.5-flash",
      "operation": "createImage",
      "prompt": "A photorealistic portrait of a woman with flowers in her hair, golden hour lighting",
      "quantity": 1
    }
  }]
}
```

## Image editing (`editImage`)

```json
{
  "steps": [{
    "$type": "imageGen",
    "input": {
      "engine": "gemini",
      "model": "2.5-flash",
      "operation": "editImage",
      "prompt": "Make it a winter scene with snow falling",
      "images": [
        "https://image.civitai.com/.../source.jpeg"
      ]
    }
  }]
}
```

Pass 1–4 reference images in `images[]` — the prompt is treated as an edit instruction applied across them.

## Parameters

| Field | Required | Default | Notes |
|-------|----------|---------|-------|
| `model` | ✅ | — | Only `2.5-flash` exposed today. |
| `operation` | ✅ | — | `createImage` or `editImage`. |
| `prompt` | ✅ | — | Natural-language. No explicit cap documented; keep it reasonable. |
| `quantity` | | `1` | `1`–`4`. |
| `images[]` | ✅ on `editImage` | — | 1–4 entries. URLs, data URLs, or Base64. |

No aspect-ratio control, no resolution tier, no seed, no safety toggle. If you need any of those, switch to the [`google` engine](./google) with `nano-banana-2`.

## Reading the result

Standard `imageGen` output — an `images[]` array, one per `quantity`:

```json
{
  "status": "succeeded",
  "steps": [{
    "name": "0",
    "$type": "imageGen",
    "status": "succeeded",
    "output": {
      "images": [
        { "id": "blob_...", "url": "https://.../signed.png" }
      ]
    }
  }]
}
```

Blob URLs are signed and expire — refetch the workflow or call [`GetBlob`](/orchestration/reference/operations/GetBlob) for a fresh URL.

## Runtime

Typical wall time 8–20 s per image including queue. `wait=60` is comfortable for `quantity: 1`–`2`; larger batches or busy periods warrant `wait=0` + polling.

## Cost

Billed in Buzz on the workflow's `transactions`. Use `whatif=true` for an exact preview; see [Payments (Buzz)](/orchestration/guide/submitting-work#payments-buzz) for currency selection.

Flat per-image:

```
total = 60 × quantity
```

| Shape | Buzz |
|-------|------|
| `createImage`, `quantity: 1` | **~60** |
| `editImage` with 1 reference | ~60 |
| `createImage`, `quantity: 4` | ~240 |

Gemini 2.5 Flash's price doesn't depend on resolution (there's no `resolution` field) or the number of reference images. If you need the same product with tiered resolution pricing, the [`google` engine](./google)'s `nano-banana-2` is materially cheaper at 1K (~104 Buzz) and has a tiered scale for 2K / 4K.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `400` with unknown property `aspectRatio` / `resolution` | Those fields live on the `google` engine, not `gemini` | Switch engines, or drop the field. |
| `400` with "images minItems" on `editImage` | Empty `images[]` | Include at least one source image when `operation: "editImage"`. |
| `400` with "images maxItems" | More than 4 source images | Trim to 4 — `google/nano-banana-2` accepts up to 10 if you need more. |
| Output doesn't look edited | Prompt described target state rather than the change | Phrase as an instruction (`"Make it a winter scene"`) rather than a description of the result. |
| Request timed out (`wait` expired) | Busy Gemini API queue | Resubmit with `wait=0` and poll. |
| Step `failed`, `reason = "blocked"` | Google's content filter | Don't retry the same input — see [Errors & retries → Step-level failures](/orchestration/guide/errors-and-retries#step-level-failures). |

## Related

* [`SubmitWorkflow`](/orchestration/reference/operations/SubmitWorkflow) — operation used by every example here
* [`GetWorkflow`](/orchestration/reference/operations/GetWorkflow) — for polling
* [Google image generation](./google) — Nano Banana / Imagen 4 via Vertex AI (alternate routing with richer controls)
* [OpenAI image generation](./openai) — alternative commercial tier
* [Flux 2](./flux2) / [Flux 1](./flux1) / [Qwen](./qwen) — open-weights alternatives on Civitai-hosted workers
* Full parameter catalog: the `Gemini25FlashCreateImageGenInput` and `Gemini25FlashEditImageGenInput` schemas in the [API reference](/orchestration/reference/)
* [`imageGen` endpoint OpenAPI spec](https://orchestration.civitai.com/v2/consumer/recipes/imageGen/openapi.yaml) — standalone OpenAPI 3.1 YAML covering the full `imageGen` surface
