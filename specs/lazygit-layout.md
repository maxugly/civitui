# Specification: Lazygit-Style Single-Window Layout

Redesign the civitui TUI into a single-window layout where the config form is always
visible and generation jobs render inline in a scrollable queue below. The full-screen
phase transitions (pricing → confirm → submitting → polling → downloading → done)
are collapsed into inline status rows in the queue.

## 1. Layout Wireframe

```
┌─ civitui ──────────────────────────────────────────────────────────────────────┐
│                                                                                │
│  Configure generation:                                                         │
│                                                                                │
│  ▶ Prompt:         [car_____________________________]                          │
│    Negative Prompt: [optional - what to avoid________]                         │
│    Model:           [air:flux1:checkpoint:civitai...]                          │
│    Flux Mode:       [urn:air:flux1:checkpoint:civ...]                          │
│    Scheduler:       [EulerA_________________________]                          │
│    ...                                                                         │
│                                                                                │
│  ℹ Help: Describe the image you want to generate.                              │
│                                                                                │
│  ── Generation Queue ───────────────────────────────────────────────────────── │
│                                                                                │
│  ⣾  "car"                                     30ⓑ  submitting...               │
│  ✓  "a majestic cat wearing a top hat"         42ⓑ  civitai_output_wf_01.png   │
│  ✗  "test prompt"                               -   API error 400              │
│                                                                                │
│  ctrl+c quit  •  tab/↑↓ navigate  •  enter generate                            │
└────────────────────────────────────────────────────────────────────────────────┘
```

Config form occupies the top portion. Queue occupies the bottom. Both visible
simultaneously. User edits config above while previous jobs complete below.

## 2. Data Model Changes

### 2a. Job struct

```go
// Job represents a single generation request and its lifecycle.
type Job struct {
    ID        string              // unique identifier (uuid or counter)
    Prompt    string              // truncated prompt for display
    Cost      int                 // buzz cost (0 until pricing completes)
    Status    JobStatus           // current lifecycle phase
    ErrMsg    string              // error message if failed
    Result    string              // downloaded image path (or first of many)
    Spinner   int                 // animation frame index
    
    // Internal pipeline state (not displayed)
    req       civit.GenerationRequest
    jobID     string
    pollResp  *civit.WorkflowResponse
    pollTick  int
    dlPaths   []string
    dlErrors  []string
}
```

### 2b. JobStatus enum

```go
type JobStatus int
const (
    JobPricing     JobStatus = iota  // waiting for CalculatePrice
    JobSubmitting                    // waiting for SubmitJob
    JobPolling                       // polling for completion
    JobDownloading                   // downloading images
    JobDone                          // completed successfully
    JobFailed                        // error at any stage
)
```

### 2c. Model struct additions

```go
type Model struct {
    // ... existing fields (client, phase, inputs, presets, etc.) ...
    
    // Job queue replaces phase-based pipeline state.
    jobs       []*Job              // all jobs, newest at bottom
    nextJobID  int                 // auto-incrementing ID counter
    
    // Phase is reduced to a single value: always PhaseConfig.
    // The phase field stays (for esc/ctrl+c routing and footer context)
    // but is never set to anything other than PhaseConfig.
}
```

### 2d. Removed fields

The following fields are removed from Model (moved into Job struct):
- `cost int`
- `jobID string`
- `pollResp *civit.WorkflowResponse`
- `pollTick int`
- `downloadPaths []string`
- `downloadErrors []string`
- `errMsg string` (becomes per-job, not global)

## 3. State Machine Changes

### 3a. Phase simplification

The `phase` field no longer transitions through PRICING/CONFIRM/SUBMITTING/POLLING/
DOWNLOADING/DONE. It stays at `PhaseConfig` permanently. The old phases become
job statuses rendered in the queue.

### 3b. Submit flow

```
User presses enter on config form
  → validate form (prompt required, numeric fields valid)
  → create new Job{ID, Prompt, Status: JobPricing}
  → append to m.jobs
  → dispatch priceCmd as tea.Cmd (same async command, different target)
  → config stays active, cursor stays in form

priceResultMsg arrives
  → find job by context (msg carries job ID)
  → if err: job.Status = JobFailed, job.ErrMsg = err
  → if ok: job.Cost = msg.cost, job.Status = JobSubmitting
  → auto-submit: dispatch submitCmd immediately (skip confirm)
  → queue updates inline, no full-screen takeover

submitResultMsg arrives
  → if err: job.Status = JobFailed
  → if ok: job.jobID = msg.jobID, job.Status = JobPolling, dispatch pollCmd

pollResultMsg arrives
  → if err: job.Status = JobFailed
  → if "succeeded": job.Status = JobDownloading, dispatch downloadCmd
  → if "failed"/"cancelled": job.Status = JobFailed
  → else: keep polling

downloadResultMsg arrives
  → accumulate dlPaths/dlErrors on job
  → when all done: job.Status = JobDone, job.Result = dlPaths[0]
```

### 3c. Confirm phase removed

No more "this will cost X buzz. confirm? (y/n)". The user pressed enter to submit.
They saw the prompt, they hit enter. That IS the confirmation. Cost appears in the
queue row after pricing completes. If the user wants to know cost beforehand, they
can still use `--dump`.

### 3d. Multiple concurrent jobs

Jobs are independent. Each has its own tea.Cmd chain. When a priceCmd completes,
its message carries the job ID so Update() routes to the correct job. Multiple
jobs can be in different stages simultaneously — one polling while another
downloads while a third prices.

## 4. View Rendering

### 4a. viewConfig (modified)

The config form renders as a single column without the right split-pane. The presets pane is replaced by floating popups, and the help text renders as a single thin line at the bottom of the config pane (see [specs/popup-presets.md](file:///home/m/snc/cod/civitui/specs/popup-presets.md)).

### 4b. viewQueue (new)

Renders below the config form. Each job is one line:

```
⣾  "car"                                     30ⓑ  submitting...
✓  "a majestic cat wearing a top hat"         42ⓑ  civitai_output_wf_01.png
✗  "test prompt"                               -   API error 400
```

Format: `{status_icon}  "{prompt_truncated}"  {cost}ⓑ  {status_text}`

- Status icons: ⣾ (spinning, in progress), ✓ (done), ✗ (failed)
- Prompt truncated to 40 chars with "..."
- Cost shown as `Nⓑ` (0 if not yet priced, "-" if failed)
- Status text: "pricing...", "submitting...", "polling...", "downloading...", filename (done), error (failed)

Queue scrolls if there are more jobs than visible lines. Newest job at bottom.
Config form height is fixed; queue takes remaining terminal height.

### 4c. View composition

```go
func (m Model) View() string {
    // 1. Header bar (unchanged)
    // 2. Config form (unchanged — viewConfig)
    // 3. Separator: "── Generation Queue ──"
    // 4. Queue (new — viewQueue)
    // 5. Footer (updated: "enter generate" instead of "enter confirm")
}
```

No phase switching in View(). The old case statement is replaced with just
the config + queue. No more viewPricing, viewConfirm, viewSubmitting,
viewPolling, viewDownloading, viewDone.

## 5. Key Handling

### 5a. handleKey (simplified)

```go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    key := msg.String()
    
    switch key {
    case "ctrl+c":
        return m, tea.Quit
    case "esc":
        if m.inPresetsPane {
            m.inPresetsPane = false
            return m, nil
        }
        return m, tea.Quit
    }
    
    // All config navigation is unchanged — tab, shift+tab, up, down.
    return m.handleConfigKey(msg)
}
```

No more phase-aware routing. Esc always quits (after exiting presets pane).
PhaseConfirm and PhaseDone esc routing is gone — there is no confirm or done phase.

### 5b. handleConfigKey (modified)

```go
case "enter":
    // Validate, create job, dispatch priceCmd.
    // Config stays active. No phase transition.
    if m.inputs[fiPrompt].Value() == "" {
        m.errMsg = "prompt is required"
        m.inputs[m.activeInput].Focus()
        return m, nil
    }
    if err := m.validateNumericFields(); err != nil {
        m.errMsg = err.Error()
        m.inputs[m.activeInput].Focus()
        return m, nil
    }
    
    job := m.createJob()
    m.jobs = append(m.jobs, job)
    m.errMsg = ""
    return m, priceCmd(m.client, job.ID, m.toRequest())
```

### 5c. Footer

```
ctrl+c quit  •  tab/↑↓ navigate  •  enter generate
```

## 6. Message Routing

### 6a. Async command changes

All async commands now carry a job ID for routing:

```go
type priceResultMsg struct {
    jobID string
    cost  int
    err   error
}

type submitResultMsg struct {
    jobID  string
    apiJobID string
    err    error
}

type pollResultMsg struct {
    jobID string
    resp  *civit.WorkflowResponse
    err   error
}

type downloadResultMsg struct {
    jobID string
    path  string
    err   error
}
```

### 6b. Update() routing

```go
case priceResultMsg:
    job := m.findJob(msg.jobID)
    if job == nil { return m, nil }
    if msg.err != nil {
        job.Status = JobFailed
        job.ErrMsg = msg.err.Error()
    } else {
        job.Cost = msg.cost
        job.Status = JobSubmitting
        return m, submitCmd(m.client, msg.jobID, job.req)
    }
```

Same pattern for all four message types. findJob locates by ID, updates status,
dispatches next command in chain.

## 7. Task Breakdown

### 7a. Engine changes (pkg/civit/civit.go)

- [ ] No changes needed — async commands already return typed messages. Only
  the message structs gain a jobID field.

### 7b. UI changes (internal/ui/ui.go)

- [ ] Add Job struct and JobStatus enum
- [ ] Add `jobs []*Job` and `nextJobID int` to Model
- [ ] Remove per-phase pipeline fields from Model (cost, jobID, pollResp, etc.)
- [ ] Add jobID field to all four async message structs
- [ ] Modify submit flow in handleConfigKey: create Job, dispatch priceCmd
- [ ] Add findJob helper
- [ ] Rewrite Update() message routing: find job by ID, update status, chain commands
- [ ] Remove PhasePricing/PhaseConfirm/PhaseSubmitting/PhasePolling/PhaseDownloading/PhaseDone
  from View() switch — config only
- [ ] Remove PhaseConfirm/PhaseDone from handleKey esc routing
- [ ] Add viewQueue() rendering function
- [ ] Update footer text: "enter generate"
- [ ] Wire tea.Batch for concurrent downloads (already done, port to per-job)

### 7c. Tests

- [ ] Update mock tests for new message structs with jobID
- [ ] Run integration tests to verify API acceptance

### 7d. Quality gates

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test ./... -count=1` passes
- [ ] `go test -tags=integration -run Integration ./pkg/civit/ -v` passes

## 8. Edge Cases

- **Job fails before pricing**: Status icon ✗, cost "-", error message shown
- **Job fails during polling**: same as above, with API error details
- **Multiple downloads**: result shows count "3 images" instead of single filename
- **Empty prompt**: blocked at submit time, never enters queue
- **Queue overflow**: scrollable, oldest jobs scroll off top, newest at bottom
- **Terminal resize**: config gets minimum 15 rows, queue gets remainder
- **esc/ctrl+c while jobs running**: quit immediately, jobs may continue on server
- **Stale job cleanup**: jobs older than 5 minutes with Done/Failed status auto-removed
- **No confirm phase**: the user intended to submit. no double-check. cost visible in queue.

## 9. Migration Notes

This is a breaking change to the UX. The old phase system is fully removed.
Users who previously saw full-screen pricing/confirm/submit/etc. will instead
see jobs appear in the queue. No functionality is lost — all the same API calls
happen, just rendered differently.

The `--dump` flag and `requests.log` remain available for pre-submit inspection.

## 10. References

- Current TUI: `internal/ui/ui.go` (1968 lines)
- Current state machine spec: `specs/tui_state_machine.md`
- Official API docs: https://developer.civitai.com (orchestration recipes / flux1)
- Input masking pattern: `internal/ui/AGENTS.md`
