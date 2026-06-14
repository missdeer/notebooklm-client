---
name: notecraft
description: "Google NotebookLM automation. Create notebooks, add sources, generate podcasts/videos/slides/flashcards, chat, deep research. Activates on intent like create a podcast, research topic, summarize URL."
user-invocable: true
allowed-tools: Bash, Read, Write
argument-hint: "[research|podcast|analyze|chat] [args...]"
---

# NotebookLM

Automate Google NotebookLM via the `notebooklm` CLI (single static Go binary, assumed on `$PATH`). Generate audio podcasts, reports, slides, quizzes, videos, infographics, data tables, flashcards, analyze content, manage notebooks, and chat.

## Setup

```bash
notebooklm --version              # Verify binary is on PATH
notebooklm export-session         # One-time: opens Chrome, log in to Google
notebooklm list --transport auto  # Verify
```

Session is saved to `~/.notebooklm/session.json` by default. Override with `--home <dir>` or `NOTEBOOKLM_HOME=<dir>`.

If a browser isn't available on the host (CI, server), capture cookies elsewhere and bootstrap:

```bash
notebooklm import-cookies ./cookies.txt    # Netscape format, or Firefox/Safari profile path
notebooklm import-session ./session.json   # Or paste JSON inline
notebooklm refresh-session                 # Force a token refresh
notebooklm session-status                  # Cookie expirations + remaining validity
```

## When This Skill Activates

Recognize requests like:
- "Create a podcast about [topic]"
- "Research [topic] in depth"
- "Summarize this URL"
- "Generate a report / study guide / blog post"
- "Generate flashcards / slides / quiz"
- "Create an infographic / data table / video"
- "Chat with my notebook"
- "What notebooks do I have?"
- "Add this PDF to my [company/topic] notebook"
- "Attach this URL to notebook X"

## CLI Commands

All commands use `--transport auto` for headless mode (Go uses native uTLS, no subprocess needed).

| Task | Command |
|------|---------|
| List notebooks | `notebooklm list --transport auto` |
| Notebook details | `notebooklm detail <id> --transport auto` |
| Add source to existing notebook | `notebooklm source add <id> --transport auto --file ./paper.pdf` |
| Add URL to existing notebook | `notebooklm source add <id> --transport auto --url "https://..."` |
| Delete notebooks | `notebooklm delete <id...> --transport auto` |
| Chat | `notebooklm chat <id> --transport auto --question "..."` |
| Chat with citations | `notebooklm chat <id> --transport auto --question "..." --with-citations` |
| Chat scoped to sources | `notebooklm chat <id> --transport auto --question "..." --source-ids a,b,c` |
| Podcast from URL | `notebooklm audio --transport auto --url "https://..." -o /tmp/audio -l en` |
| Podcast (debate, short) | `notebooklm audio --transport auto --topic "AI" -o /tmp/audio --format debate --length short` |
| Report (study guide) | `notebooklm report --transport auto --url "https://..." -o /tmp/report --template study_guide` |
| Report (custom) | `notebooklm report --transport auto --url "https://..." -o /tmp/report --template custom --instructions "Write a SWOT analysis"` |
| Slides | `notebooklm slides --transport auto --url "https://..." -o /tmp/slides --format presenter` |
| Video | `notebooklm video --transport auto --url "https://..." -o /tmp/video --format explainer --style whiteboard` |
| Quiz | `notebooklm quiz --transport auto --url "https://..." -o /tmp/quiz --difficulty medium` |
| Flashcards | `notebooklm flashcards --transport auto --url "https://..." -o /tmp/flashcards` |
| Infographic | `notebooklm infographic --transport auto --url "https://..." -o /tmp/infographic --style professional` |
| Data table | `notebooklm data-table --transport auto --url "https://..." -o /tmp/table --instructions "Compare by category"` |
| Analyze content | `notebooklm analyze --transport auto --url "https://..." --question "Summarize"` |
| Diagnose issues | `notebooklm diagnose` |
| Import session | `notebooklm import-session <file-or-json>` |
| Refresh tokens | `notebooklm refresh-session` |

### Source options (shared by all generation commands)

```
--url <url>              Source URL
--text <text>            Source text
--file <path>            Local file (pdf, txt, md, docx, csv, pptx, epub, mp3, wav, etc.)
--topic <topic>          Research topic (web search)
--research-mode <mode>   fast | deep (default: fast)
```

### Generation command options

| Command | Key options |
|---------|------------|
| `audio` | `--format` (deep_dive/brief/critique/debate), `--length` (short/default/long), `--instructions`, `-l` |
| `report` | `--template` (briefing_doc/study_guide/blog_post/custom), `--instructions`, `--language` |
| `video` | `--format` (explainer/brief/cinematic), `--style` (auto/classic/whiteboard/kawaii/anime/watercolor/retro_print), `--instructions`, `--language` |
| `quiz` | `--instructions`, `--quantity` (fewer/standard), `--difficulty` (easy/medium/hard) |
| `flashcards` | `--instructions`, `--quantity`, `--difficulty` |
| `infographic` | `--orientation` (landscape/portrait/square), `--detail` (concise/standard/detailed), `--style` (sketch_note/professional/bento_grid), `--instructions`, `--language` |
| `slides` | `--format` (detailed/presenter), `--length` (default/short), `--instructions`, `--language` |
| `data-table` | `--instructions` (describe table structure), `--language` |

All generation commands require `-o, --output <dir>`.

### Transport flag

```
--transport auto      Best available non-browser (recommended; uses native uTLS)
--transport http      Native uTLS HTTP/2 with Chrome fingerprint
--transport curl      curl-impersonate subprocess (requires binary on PATH)
--transport browser   Headed/headless Chrome via rod (requires --profile or --chrome-path)
```

### Proxy flags (shared)

```
--proxy <url>         Generic proxy URL (any scheme)
--socks5-proxy <hp>   SOCKS5 host:port (scheme prepended)
--http-proxy  <hp>    HTTP host:port
--https-proxy <hp>    HTTPS host:port
```

Env-var fallback: `SOCKS5_PROXY`, `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY` (lowercase variants also honored).

## Multi-Account

```bash
notebooklm --home ~/.notebooklm-work list --transport auto
# or
NOTEBOOKLM_HOME=~/.notebooklm-work notebooklm list --transport auto
```

## Autonomy Rules

**Run automatically:** `list`, `detail`, `diagnose`, `session-status`

**Ask before running:** generation commands (long-running, creates notebook), `source add` (modifies user's notebook), `delete` (irreversible), `refresh-session` (rewrites session file)

## Common Workflows

### Quick podcast from URL
```bash
notebooklm audio --transport auto --url "https://example.com/article" -o ./output -l en
```

### Research a topic (deep, debate format)
```bash
notebooklm audio --transport auto --topic "quantum computing" --research-mode deep -o ./output --format debate
```

### Generate a study guide
```bash
notebooklm report --transport auto --url "https://example.com/paper.pdf" -o ./output --template study_guide --instructions "Focus on key formulas"
```

### Generate slides from a topic
```bash
notebooklm slides --transport auto --topic "machine learning basics" -o ./output --format presenter --length short
```

### Quiz from an article
```bash
notebooklm quiz --transport auto --url "https://example.com/article" -o ./output --difficulty hard --quantity standard
```

### Analyze and ask questions
```bash
notebooklm analyze --transport auto --url "https://example.com/paper.pdf" --question "What are the key findings?"
```

### Chat with existing notebook
```bash
notebooklm list --transport auto
notebooklm chat <notebook-id> --transport auto --question "Summarize the main points"
```

### Chat with per-citation metadata (JSON output)
```bash
notebooklm chat <notebook-id> --transport auto --question "What does the paper claim?" --with-citations
```
Returns `{ text, threadId, responseId, citations: [{ index, sourceId, relevance, charStart, charEnd, excerpt, chunkId }] }`. Use this when downstream code needs to render or verify per-sentence citations.

### Add new material to an existing notebook
```bash
notebooklm source add <notebook-id> --transport auto --file ./new-report.pdf
notebooklm source add <notebook-id> --transport auto --url "https://static.cninfo.com.cn/finalpage/2026-04-16/1225107391.PDF"
```
Use this when the user wants to extend a long-running topic/company notebook with a fresh document rather than creating a new throwaway notebook.

## Error Handling

| Error | Action |
|-------|--------|
| "No session available" | `notebooklm export-session` (or `import-cookies` / `import-session` on headless hosts) |
| "Session expired" | Auto-refreshes via uTLS-fingerprinted client; if it still fails, re-export |
| "in-body UNAUTHENTICATED (code 16)" | Transport detected stale `at`/`bl`; refresh is automatic — only act if it loops |
| "Quota exceeded" | Daily limit hit — wait or upgrade |
| "Rate limited" | Wait a few minutes, retry |
| "curl-impersonate binary not found" | Either drop `--transport curl` (default auto picks native uTLS) or install curl-impersonate |
| "Audio download returned login page" | Re-run `notebooklm export-session` to refresh domain-scoped cookies for CDN downloads |

## Known Limits

- Audio generation takes 5–10 minutes; video can take longer
- Daily generation limits per artifact type — no API to query remaining quota
- Studio artifact types are server-driven (`get_studio_config`) — Google may add/remove types anytime
- Session auto-refreshes on `at`/`bl` expiry (~1–2 h); long-lived cookies last weeks
- Research deep-mode imports 40+ URL sources; readiness threshold is "70% indexed or 30 sources, whichever is lower" — a handful of unreachable URLs is tolerated by design
- Generation RPC sometimes returns a task ID that differs from the final artifact ID; the poller falls back to any ready AUDIO/VIDEO artifact with a download URL
