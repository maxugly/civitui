# Specification: Configuration File Loading

This specification details how the Civitai TUI (`civitui`) resolves and loads the API key from local user configuration files.

## 1. Resolution Hierarchy
The application will resolve the Civitai API key using the following priority:
1. **Environment Variable**: `CIVITAI_API_KEY` (takes precedence for ease of temporary overrides).
2. **Primary TUI Config**: `~/.config/civitui/civitui.conf` (configured specifically for this TUI client).
3. **Legacy CLI Config**: `~/.config/civitai/config.yaml` (falls back to the existing CLI configuration if present, for a smooth migration).

---

## 2. Configuration File Format support
To avoid external dependencies (e.g., third-party YAML or INI parsers), the loader will parse config files line-by-line:
* Ignore empty lines and lines starting with `#` (comments).
* Match keys split by either `:` or `=`.
* Strip leading/trailing whitespaces and quotes (`"` or `'`).
* Identify the key `api_key`.

---

## 3. Configuration Path Construction
Paths will be constructed relative to the user's home directory using `os.UserHomeDir()`:
* Primary: `<HomeDir>/.config/civitui/civitui.conf`
* Legacy: `<HomeDir>/.config/civitai/config.yaml`
