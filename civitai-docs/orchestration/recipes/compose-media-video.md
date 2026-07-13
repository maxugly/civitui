***

## title: Compose media (video)

# Compose media (video)

The `composeMedia` step composes multiple audio and video elements onto a single timeline and canvas. The output is **video whenever any element is a video** (or you set `output.type: "video"`), and audio otherwise — so stitching clips together is as simple as listing them. Each video element is scaled and placed on the canvas, layered by z-order, while audio from every element is mixed and muxed in. Use it to stitch clips end-to-end, drop a soundtrack onto a clip, or build picture-in-picture.

::: tip Audio-only form
With only audio elements, `composeMedia` produces a single mixed audio blob — see [Multi-speaker dialogue](./multi-speaker-dialogue). The timeline rules (`at`, `offset`) and `transformers` are identical; this page covers the video-producing form.
:::

## How it composes

* **Elements are resources.** Each element must be AIR-resolvable — a `$ref` to a prior step's output, or a Civitai resource/blob URL — because the worker pre-downloads it. Direct third-party URLs aren't supported for now.
* **Output kind** is derived from the elements — video if any element is video, otherwise audio. Force it with `output.type` (`"audio"` or `"video"`), and choose the file format with `output.container`.
* **`canvas`** is optional. It overrides the output `width`, `height`, `fps`, and `background`; when omitted, the canvas is derived from the elements (largest video, fastest frame rate).
* **Each element** is placed in time exactly like the audio form — implicitly back-to-back, nudged by `offset`, or pinned with an absolute `at`.
* **Video elements** are drawn on the canvas; **audio elements** (and the audio tracks of video elements) are mixed. An element whose URL is an audio file is mixed, not drawn.
* **`layout`** controls where a video element sits: `zOrder` (array order breaks ties), position `x`/`y`, and `scale`. With no `layout`, a video element is scaled to fit the canvas and centred.
* **`transformers`** apply per-element `fadeIn`/`fadeOut` (to both picture and sound) and `volume` (to sound).

## Stitching clips end-to-end

List the clips in order — no `canvas`, no `layout`. The output is derived as video, the clips play back-to-back, and the canvas is taken from the inputs.

## Audio over a video

A single video fills the canvas; a music bed is mixed in at an absolute time, attenuated and faded up.

## Picture-in-picture

Two videos on one canvas. The base clip fills the frame; the second is scaled to a 30% inset, positioned in the lower-right, and drawn on top.

Array order is z-order: later elements draw over earlier ones (ties broken by `layout.zOrder`). Position the inset with `layout.x`/`layout.y` in canvas pixels, and size it with `layout.scale` (a fraction of the canvas).

## Input fields

### `composeMedia` step

| Field | Required | Default | Notes |
|-------|----------|---------|-------|
| `elements` | ✅ | — | Array of elements to compose. At least one. |
| `output` | | derived | `{ type, container }`. `type` is `audio`/`video` (derived from the elements when unset); `container` is `mp4`/`webm` for video or `ogg`/`mp3` for audio (`auto` picks mp4/ogg). |
| `canvas` | | derived | Optional output geometry. When omitted for video, derived from the elements. |
| `normalize` | | `false` | When `true`, the audio mix divides by N to avoid clipping. |

### `canvas`

| Field | Required | Default | Notes |
|-------|----------|---------|-------|
| `width` | ✅ | — | Output width in pixels (clamped server-side, rounded to even). |
| `height` | ✅ | — | Output height in pixels. |
| `fps` | | `30` | Output frame rate. Source clips are resampled to this rate. |
| `background` | | `#000000` | Hex colour painted where no element covers the canvas. |

### Per-element fields

| Field | Required | Default | Notes |
|-------|----------|---------|-------|
| `url` | ✅ | — | A `{ "$ref": "<step>", "path": "..." }` referencing a prior step's output (`output.video.url` for `videoGen`, `output.videoBlob.url`/`output.audioBlob.url` for a prior compose/TTS step), or a Civitai resource/blob URL. The worker pre-downloads each element as a resource, so direct third-party URLs are **not supported for now**. |
| `at` | | implicit | Absolute timeline anchor in seconds. When unset, the element follows the previous non-anchored element. |
| `offset` | | `0` | Seconds to nudge from the implicit position. Ignored when `at` is set. |
| `layout` | | fit + centre | Spatial placement of a video element. Ignored for audio. |
| `transformers` | | `[]` | Ordered per-element effects, applied in array order. |

### `layout`

| Field | Default | Notes |
|-------|---------|-------|
| `zOrder` | `0` | Higher draws on top; ties broken by array order. |
| `x` / `y` | `0` | Top-left position on the canvas, in pixels. With no `layout` the element is centred. |
| `scale` | `1.0` | Fraction of the canvas the element is fitted into (`0.3` = a 30% inset). |
| `fit` | `contain` | `contain` (fit + letterbox), `cover` (fill + crop), or `stretch` (ignore aspect). |

### Transformers

Each entry is `{ "type": "<name>", ...params }`:

| `type` | Params | Applies to |
|--------|--------|-----------|
| `fadeIn` | `durationMs` (int) | Picture and sound |
| `fadeOut` | `durationMs` (int) | Picture and sound |
| `volume` | `db` (float) | Sound only |

## Reading the result

```json
{
  "status": "succeeded",
  "steps": [
    {
      "$type": "composeMedia",
      "status": "succeeded",
      "output": {
        "type": "video",
        "videoBlob": {
          "id": "A1B2C3...mp4",
          "url": "https://orchestration-new.civitai.com/v2/consumer/blobs/A1B2C3...mp4?sig=...",
          "width": 1280,
          "height": 720
        },
        "elements": [
          { "startSeconds": 0.0, "duration": 8.0 },
          { "startSeconds": 1.0, "duration": 10.0 }
        ]
      }
    }
  ]
}
```

* **`type`** — `"video"` here. The output is discriminated on `type`: a video composition carries `videoBlob`, an audio mixdown carries `audioBlob`.
* **`videoBlob.url`** — signed URL for the composed MP4/WebM.
* **`videoBlob.width` / `height`** — the output canvas dimensions.
* **`elements[]`** — per-input resolved timing in submission order.

## Runtime

Video compositing runs ffmpeg on a GPU worker (NVENC, with a software fallback). Wall-clock scales with output duration, resolution, and the number of layered elements — expect seconds-to-minutes for short clips. Submit with `wait=0` and poll.

## Cost

Video compositions carry a small compositing charge scaled by the number of elements (audio-only mixdowns remain free). See [Payments (Buzz)](/orchestration/guide/submitting-work#payments-buzz) for how charges surface in the cost preview, and run a `whatif=true` submission to see the exact Buzz cost before executing.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Job fails at the worker | No GPU-capable worker advertised the `composeMedia` capability | Ensure the target environment runs media workers with compose-media enabled. |
| `400` validation error, "is not an AIR-resolvable resource" | An element `url` is a direct third-party URL | Reference a prior step via `$ref`, or use a Civitai resource/blob URL — external URLs aren't supported yet. |
| An element you expected on-screen is only audible | Its URL is an audio file (or it has no video stream), so it is mixed, not drawn | Supply a video URL for elements you want on the canvas. |
| Inset is letterboxed inside its box | `fit: contain` preserves aspect ratio | Use `fit: cover` to fill the inset (cropping overflow), or size the box to the clip's aspect. |
| Output longer/shorter than expected | Total length is `max(at + duration)` across all elements | Use `at`/`offset` to place elements on the timeline. |

## Related

* [Multi-speaker dialogue](./multi-speaker-dialogue) — the audio-only form of the same step.
* [WAN video generation](./wan) and [other video recipes](./) — produce clips to feed into a composition via `$ref`.
* [Workflows → Dependencies](/orchestration/guide/workflows#dependencies-parallelism) — how `$ref` chains one step's output into another.
* The [`ComposeMediaInput` and `ComposeMediaOutput` schemas](/orchestration/reference/operations/InvokeComposeMediaStepTemplate) — full parameter reference.
