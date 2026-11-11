---
name: notebooklm
description: "Use the Go notebooklm CLI to manage Google NotebookLM notebooks and sources, query and export chats, generate and download Studio artifacts, run Web or Drive research, check usage, and automate batches or pipelines. Applies to NotebookLM tasks such as podcasts, study materials, interactive reports, and source-based analysis."
user-invocable: true
allowed-tools: Bash, Read, Write
argument-hint: "[notebook|source|chat|research|audio|report|download|batch] [args...]"
---

# NotebookLM

Use the standalone Go `notebooklm` CLI for Google NotebookLM tasks. The executable must be on PATH; installing this skill installs instructions and references, not the CLI binary. Inspect `notebooklm <command> --help` before supplying unfamiliar arguments. `notebooklm --ai` prints the command schema.

## Start and select the account

```bash
notebooklm --version
notebooklm notebook list --json
# If no valid session is available:
notebooklm login
```

For named accounts, use `--profile <name>` on grouped commands. Prefer an explicit profile for the task over changing the default account. `--home <dir>` / `NOTEBOOKLM_HOME` isolates configuration; `--session-path` overrides session selection. Read [accounts and recovery](references/accounts-recovery.md) for login/import, protected storage, profile management, usage, proxies, or authentication errors.

## Read the relevant reference

Read only the files needed for the current task. Follow a workflow across references when it spans more than one area.

| Task | Reference |
| --- | --- |
| Manage notebooks, add or sync sources, organize source labels, edit notes, share or invite | [Notebooks and sources](references/notebooks-sources.md) |
| Query sources, resume/export conversations, discover Web/Drive sources, import research | [Chat and research](references/chat-research.md) |
| Generate podcasts/reports/slides/video/quizzes/flashcards/mind maps/tables, manage interactive reports, download/revise/export artifacts | [Studio artifacts](references/studio.md) |
| Set aliases/tags, select notebooks, run batches or YAML pipelines | [Automation](references/automation.md) |
| Select accounts, import sessions, protect credentials, inspect usage or recover failures | [Accounts and recovery](references/accounts-recovery.md) |

## Choose the workflow

- Existing notebook: inspect its sources, then use `notebook`, `source`, `chats`, and `<kind> create <notebook-id>`. Narrow generation with `--source-ids` when appropriate.
- New topic: create a notebook, add material or research results, then generate. Preserve the notebook ID for follow-up work.
- One-shot conversion: top-level workflows such as `audio --url ...` handle notebook creation, generation, polling, and output. Their flags differ from `<kind> create`.
- Existing artifact: list its ID and status before downloading, renaming, revising, or exporting. A timeout does not establish that generation failed.
- Multiple notebooks: select targets explicitly, by tags, or with an available `--all` flag. Inspect the selection before batch mutations.

## Essential operating rules

Follow the user's requested account, output path, language, audience, and format. An explicit request to perform an operation already authorizes it; do not ask again merely because the CLI exposes `--confirm`. Pass it when the operation is authorized. Ask only when target or scope is materially unclear. Sharing and invitations require the user's instruction to share or invite.

Never print cookies, tokens, decrypted credentials, or keyring keys. Check usage when quota affects the task, and do not repeatedly retry generation after a rate or quota error.

If a command returns `token expired` (case-insensitive), run `notebooklm export-session --headless` once. If that succeeds, retry the original command once with the same arguments. If session export or the retry fails, report the error and stop; do not repeat the recovery loop. Use this recovery before the general authentication troubleshooting steps in the reference.

`<kind> create` returns an artifact ID and does not automatically download the result. Inspect Studio status and download that exact artifact. `download <kind> --output` is a file path; original `studio download -o` and `download all --output-dir` take directories. See the Studio reference before revising slides: `slides revise` uses one-based numbers, while original `studio revise` uses zero-based indices.

Return actual notebook/artifact IDs, relevant links, and verified output paths. Distinguish accepted requests from completed artifacts, and reported paths from successfully downloaded files. For partial batches, report completed items and retry only remaining authorized work.
