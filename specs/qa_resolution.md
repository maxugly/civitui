# Specification: QA Defect Resolutions (Updated)

This spec details the resolutions for the residual QA defects identified by Grit.

## 1. Concurrent Image Downloads (Multi-Image Stall Fix)

### Issue
The current `downloadCmd` downloads all images sequentially within a single `tea.Cmd`, but only returns `results[0]` (the first download result). If the generation `Quantity > 1`, the UI only receives one `downloadResultMsg` and stalls in `PhaseDownloading` indefinitely because `done` never reaches `expected`.

### Resolution
Refactor `downloadCmd` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go) to utilize Charm's `tea.Batch(cmds...)`. This runs each download concurrently as an independent `tea.Cmd`, and emits a distinct `downloadResultMsg` for each completion.

---

## 2. Done Phase State Machine Alignment (Spec-Gap Fix)

### Issue
On `PhaseDone` (the completion screen), pressing `q` or `enter` returns the user to the `CONFIG` screen, but pressing `esc` still exits/quits the application. The TUI State Machine spec states that `q` / `esc` / `enter` should return the user to the `CONFIG` screen.

### Resolution
Refactor the global key interceptor in `handleKey` at the top of [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go) to handle `"esc"` phase-aware, routing it to `refocusConfig()` during both `PhaseConfirm` and `PhaseDone`.
* Keep `"ctrl+c"` as the global quit command.
* `"esc"` should quit only in `PhaseConfig` or other non-confirm/done phases.

### Implementation Specification
```diff
-	case "ctrl+c", "esc":
-		switch m.phase {
-		case PhaseConfig:
-			return m, tea.Quit
-		case PhaseConfirm:
-			m.refocusConfig()
-			return m, nil
-		default:
-			return m, tea.Quit
-		}
+	case "ctrl+c":
+		return m, tea.Quit
+	case "esc":
+		switch m.phase {
+		case PhaseConfirm, PhaseDone:
+			m.refocusConfig()
+			return m, nil
+		default:
+			return m, tea.Quit
+		}
```

---

## 3. Multi-Run Download State Accumulator Reset

### Issue
Returning to the `CONFIG` screen from `PhaseDone` allows starting a second generation run. However, `m.downloadPaths` and `m.downloadErrors` are not cleared. When a second generation begins, the accumulated slices are still populated, causing the done condition (`done >= expected`) to immediately trigger and transition the UI to `PhaseDone` prematurely.

### Resolution
Reset `m.downloadPaths = nil` and `m.downloadErrors = nil` when transitioning to `PhasePolling` (on `submitResultMsg` success) or `PhaseDownloading` (on polling completion).

### Implementation Specification
In `Update()` for `submitResultMsg`:
```go
	case submitResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.refocusConfig()
			return m, nil
		}
		m.jobID = msg.jobID
		m.pollTick = 0
		m.phase = PhasePolling
		m.downloadPaths = nil
		m.downloadErrors = nil
		return m, pollCmd(m.client, m.jobID)
```

In `Update()` for `pollResultMsg`:
```go
		case "succeeded":
			m.phase = PhaseDownloading
			m.downloadPaths = nil
			m.downloadErrors = nil
			return m, downloadCmd(m.client, msg.resp)
```
