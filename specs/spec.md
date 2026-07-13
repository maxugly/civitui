# **civitui: Command-Line TUI Specification & Architecture**

## **1. Project Overview**

`civitui` is a terminal-based user interface (TUI) client for the Civitai orchestration and site APIs. It is designed to allow developers and creators to interactively scaffold, validate, submit, and monitor AI generation jobs (images, videos, audio, 3D models) directly from the command line.

---

## **2. Dependencies**

### 🧮 The UI Framework & Forms Stack (Go)

* **[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)**
  * **What it does:** The central nervous system of your interface. It uses the Model-Update-View architecture (The Elm Architecture) to handle keyboard events, ticks, and state changes smoothly without a massive desktop environment footprint.
* **[github.com/charmbracelet/bubbles](https://github.com/charmbracelet/bubbles)**
  * **What it does:** The standard component library for Bubble Tea. This gives you pre-baked elements like `textinput` (for your prompt/negative prompt entry boxes) and `spinner` (to show visual feedback while the Civitai job is processing).
* **[github.com/charmbracelet/huh](https://github.com/charmbracelet/huh)**
  * **What it does:** A specialized form-building library from the Charm team. It lets you declaratively stitch together your prompt inputs, model-selection lists, and settings sliders with minimal boilerplate, automatically handling cursor focus navigation.
* **[github.com/charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)**
  * **What it does:** The styling and layout engine. It acts like CSS for your terminal, allowing you to build clear borders, margins, padding, and distinct layout frames (like dividing your input area from your rendering canvas).

### 🖼️ The Terminal Graphics Stack

* **[github.com/blacktop/go-termimg](https://github.com/blacktop/go-termimg)**
  * **What it does:** The critical component for our Progressive Enhancement Image Canvas. It bridges directly into Bubble Tea as a custom widget and handles automatic protocol detection at runtime.
  * **The Fallback Chain:** It tests your terminal and dynamically renders using the best method available:
    1. **Kitty Graphics Protocol:** High-res pixel buffers; perfect for Kitty, WezTerm, Ghostty.
    2. **Sixel:** Universal hardware graphics fallback.
    3. **Halfblocks / Braille / Block Elements:** Standard Unicode character arrays for text-only environments like raw TTYs or Termux on Android.
    4. **Colored ANSI / ASCII:** The lowest-common-denominator fallback that applies color gradients to standard characters.

### 🧪 The Built-in Go Testing Stack

* **`net/http/httptest`**
  * **What it does:** A native Go standard library package. It spins up an ephemeral, local HTTP server to inject mock Civitai JSON responses. This lets us verify that our JSON unmarshaling and error boundaries work perfectly in `pkg/civit_test.go` without spending actual Buzz or needing an internet connection.

### 🔍 Mentioned for Reference (Not in our Core Build)

* **textual (Python):** The asynchronous Python TUI framework we initially compared against before choosing Go.
* **gocui (Go):** The older terminal layout engine used by some early "lazy" applications before Bubble Tea standardized modern terminal design.
