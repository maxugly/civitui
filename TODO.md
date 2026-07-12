- [x] Create CONSTITUTION.md — completed in commit pending. File at `CONSTITUTION.md`.

The following task was completed:

Hermes, please create a new file named `CONSTITUTION.md` in the root of the project directory. This file will serve as the project's development guidelines and rules of engagement.

## Requirements for CONSTITUTION.md:

### 1. Immutable Engineering Laws
- **Strict Separation of Concerns**: 
  - `pkg/civit/` must remain a completely headless Core Engine. It manages the 5-step API transaction chain (`CalculatePrice`, `SubmitJob`, `PollJobStatus`, `DownloadImage`). It must never import or contain UI, terminal, or Bubble Tea (`tea`) logic.
  - `internal/ui/` handles all user interface elements (Bubble Tea main loop, `huh` input forms, progressive viewport canvas rendering).
- **Code Block Rule**:
  - Any code block in documentation, tasks, or prompts containing terminal commands must only contain the executable command. No comments (e.g., no lines starting with `#`), no annotations inside the block. Limit to one command per block for instant copy-paste usability.
- **Environment Policy**:
  - Keep dependencies minimal, high-performance, and independent of heavy system frameworks or systemd.

### 2. File-Driven Workflow
- Architectural designs and tasks are drafted into local markdown files (e.g., `TODO.md`, `agent.md`) by the Architect Copilot (Antigravity).
- Implementation agents (Hermes) read these files, execute the requested code modifications/creations, and update status in the relevant file once complete.

Please write the complete `CONSTITUTION.md` now and delete this `TODO.md` (or mark it complete) when finished.
