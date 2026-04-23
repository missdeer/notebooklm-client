# CLI Commands Reference

Complete list of commands and options for the `notebooklm` CLI.

## Global Flags

Available on all commands:

| Flag | Description | Default |
|---|---|---|
| `--home <dir>` | Config directory | `~/.notebooklm` |

## Shared Flag Groups

### Transport Flags

Applied to commands that connect to NotebookLM (most commands).

| Flag | Description | Default |
|---|---|---|
| `--transport <mode>` | Transport mode: `auto`, `http`, `browser`, `curl` | `auto` |
| `--session-path <path>` | Session file path | — |
| `--proxy <url>` | Proxy URL (must include scheme) | — |
| `--socks5-proxy <addr>` | SOCKS5 proxy address (auto-prepends `socks5://`) | — |
| `--http-proxy <addr>` | HTTP proxy address (auto-prepends `http://`) | — |
| `--https-proxy <addr>` | HTTPS proxy address (auto-prepends `https://`) | — |
| `--profile <dir>` | Chrome profile directory (browser transport) | — |
| `--headless` | Run browser headless | `false` |
| `--chrome-path <path>` | Chrome executable path | auto-detect |

### Source Flags

Applied to artifact-generation commands. Exactly one is required.

| Flag | Description |
|---|---|
| `--url <url>` | Source URL |
| `--text <text>` | Source text content |
| `--file <path>` | Source file path |
| `--topic <topic>` | Research topic (triggers web research) |
| `--research-mode <mode>` | Research mode: `fast`, `deep` (default: `fast`) |

---

## Artifact Generation Commands

### `audio`

Generate an audio podcast from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `-l, --language <lang>` | Audio language |
| `--instructions <text>` | Custom instructions |
| `--format <fmt>` | Audio format: `deep_dive`, `brief`, `critique`, `debate` |
| `--length <len>` | Audio length: `short`, `default`, `long` |

### `analyze`

Analyze source material with a question.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `-q, --question <text>` | Question to analyze (**required**) |

### `report`

Generate a report from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--template <name>` | Report template: `briefing_doc`, `study_guide`, `blog_post`, `custom` |

### `video`

Generate a video from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--format <fmt>` | Video format: `explainer`, `brief`, `cinematic` |
| `--style <style>` | Video style: `auto`, `classic`, `whiteboard`, `kawaii`, `anime`, `watercolor`, `retro_print` |

### `quiz`

Generate a quiz from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--quantity <q>` | Quiz quantity: `fewer`, `standard` |
| `--difficulty <d>` | Quiz difficulty: `easy`, `medium`, `hard` |

### `flashcards`

Generate flashcards from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--quantity <q>` | Quantity: `fewer`, `standard` |
| `--difficulty <d>` | Difficulty: `easy`, `medium`, `hard` |

### `infographic`

Generate an infographic from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--orientation <o>` | Orientation: `landscape`, `portrait`, `square` |
| `--detail <d>` | Detail: `concise`, `standard`, `detailed` |
| `--style <s>` | Style: `sketch_note`, `professional`, `bento_grid` |

### `slides`

Generate a slide deck from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |
| `--format <fmt>` | Slide format: `detailed`, `presenter` |
| `--length <len>` | Slide length: `default`, `short` |

### `data-table`

Generate a data table from source material.

**Flags:** transport + source flags, plus:

| Flag | Description |
|---|---|
| `-o, --output <dir>` | Output directory (default: `.`) |
| `--language <lang>` | Output language |
| `--instructions <text>` | Custom instructions |

---

## Notebook Management Commands

### `list`

List all notebooks.

**Flags:** transport flags only.

### `detail <notebook-id>`

Show notebook details (title, sources).

**Args:** `<notebook-id>` (required)

**Flags:** transport flags only.

### `delete <notebook-id> [notebook-ids...]`

Delete one or more notebooks.

**Args:** one or more `<notebook-id>` (required)

**Flags:** transport flags only.

### `chat <notebook-id>`

Chat with a notebook.

**Args:** `<notebook-id>` (required)

**Flags:** transport flags, plus:

| Flag | Description |
|---|---|
| `-q, --question <text>` | Question to ask (**required**) |

---

## Source Management Commands

### `source add <notebook-id>`

Add a source to an existing notebook. Exactly one of `--url`, `--text`, or `--file` is required.

**Args:** `<notebook-id>` (required)

**Flags:** transport flags, plus:

| Flag | Description |
|---|---|
| `--url <url>` | URL to add |
| `--text <text>` | Text content to add |
| `--file <path>` | File to upload |
| `--title <text>` | Title for text source (defaults to `Pasted Text`) |

---

## Session Management Commands

### `export-session`

Launch browser to log in and export session.

**Flags:** transport flags (browser-focused), plus:

| Flag | Description |
|---|---|
| `-o, --output <path>` | Output path for session file |

Note: Typically uses `--profile`, `--headless`, `--chrome-path` from transport flags.

### `import-session <json-file-or-string>`

Import a session from JSON file or inline string.

**Args:** `<json-file-or-string>` (required) — either a path to a session JSON file, or an inline JSON string.

**Flags:** none.

### `refresh-session`

Refresh short-lived tokens using long-lived cookies (no browser required).

**Flags:**

| Flag | Description |
|---|---|
| `--session-path <path>` | Session file path |

---

## Diagnostic Commands

### `diagnose`

Show system info and session status (OS, Go version, home dir, session path, validity, RPC overrides count).

**Flags:** none.

---

## Proxy Resolution

CLI proxy flags are resolved in this priority order (first non-empty wins):

1. `--socks5-proxy` → normalized to `socks5://...`
2. `--http-proxy` → normalized to `http://...`
3. `--https-proxy` → normalized to `https://...`
4. `--proxy` (used as-is; must include scheme)
5. Environment variables (case-insensitive): `SOCKS5_PROXY` → `HTTP_PROXY` → `HTTPS_PROXY` → `ALL_PROXY`

All proxy types support `user:pass@host:port` authentication except the `browser` transport (Chrome's `--proxy-server` does not accept embedded credentials).

## Environment Variables

| Variable | Description |
|---|---|
| `NOTEBOOKLM_HOME` | Override default config directory |
| `NOTEBOOKLM_AUTH_JSON` | Inline session JSON (skip file load) |
| `SOCKS5_PROXY` / `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` | Proxy URL fallback (case-insensitive) |
