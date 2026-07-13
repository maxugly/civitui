# civitui

Terminal UI for [Civitai](https://civitai.com) image generation. Configure a job, price it in buzz, submit, poll, and download — all in one lazygit-style window.

Built on the [Civitai Orchestrator API](https://developer.civitai.com/).

## Requirements

- Go 1.22+
- A Civitai API key ([account settings](https://civitai.com/user/account))

## Install

```bash
go build -o civitui .
```

Optional: put the binary on your `PATH` (e.g. `~/.local/bin/civitui`).

## API key

Resolved in this order:

1. `CIVITAI_API_KEY` environment variable
2. `~/.config/civitui/civitui.conf` (`api_key=…`)
3. `~/.config/civitai/config.yaml` (legacy CLI config)

## Usage

```bash
civitui
```

```bash
civitui --debug
```

```bash
civitui --dump
```

- `--debug` — verbose request/response logging (`~/.local/share/civitui/debug.log`)
- `--dump` / `--dry-run` — print the default JSON payload and exit (no API call)

### Keys (in the TUI)

| key | action |
|-----|--------|
| tab / ↑↓ | move between fields |
| → | open presets (where available) |
| enter | price & queue a generation |
| y / n | confirm or cancel a priced job |
| ctrl+c / esc | quit |

## Project layout

| path | role |
|------|------|
| `pkg/civit/` | headless API client |
| `internal/ui/` | Bubble Tea TUI |
| `specs/` | design specs |

## Links

- [Civitai](https://civitai.com)
- [Developer docs](https://developer.civitai.com/)
- API key: [civitai.com/user/account](https://civitai.com/user/account)

## Develop

```bash
go build ./...
```

```bash
go vet ./...
```

```bash
go test ./... -count=1
```
