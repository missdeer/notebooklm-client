# Go CLI

The Go executable provides notebook, source, chat, artifact, research, account and automation commands.

The embedded catalog registers 150 executable CLI routes, including verb-first alternatives. Python is only used by the maintenance script that regenerates the catalog, never by the Go executable.

| Group | Commands |
| --- | --- |
| notebook | list, create, get, describe, rename, delete, query |
| source | list, add, get, describe, content, rename, delete, stale, sync |
| chat / chats | configure, start / list, get, export, to-note, delete |
| note | list, create, update, delete |
| label | auto, list, reorganize, create, rename, emoji, move, delete |
| studio | status, rename, delete; existing download, export, revise |
| audio, video, report, quiz, flashcards, slides, infographic, data-table, mindmap | create |
| report | get, elements, element create, element create-batch |
| slides / video | revise / list |
| download | all, audio, video, slide-deck (also slides), infographic, report, mind-map, data-table, file, quiz, flashcards |
| export | artifact, to-docs, to-sheets |
| research | start, status, import |
| share | status, public, private, invite, batch |
| alias | set, get, list, delete |
| config | show, get, set |
| tag | add, remove, list, select |
| batch | query, add-source, create, delete, studio |
| cross / pipeline | query / run, list, create |
| login | check/login, switch, profile list/delete/rename |
| auth | refresh, storage status/set/resolve/relocate |
| doctor / usage | diagnostics and auth-replay / plan usage |

Use `notebooklm --help` or `notebooklm <command> --help` for arguments and options. `--ai` prints the complete command schema.

## Examples

```sh
notebooklm notebook list --json
notebooklm create notebook "Research"
notebooklm audio create <notebook-id> --confirm
notebooklm source add <notebook-id> --url https://a.example --url https://b.example --wait
notebooklm chat start <notebook-id>
notebooklm chats export <notebook-id> --format json --output chat.json
notebooklm mindmap create <notebook-id> --confirm
notebooklm download all <notebook-id> --output-dir downloads --skip-existing
notebooklm label auto <notebook-id>
notebooklm research start "a topic" --source drive --notebook-id <notebook-id>
notebooklm alias set research <notebook-id>
notebooklm tag add research --tags ai,research
notebooklm batch query "Summarize the findings" --tags research
notebooklm login --profile work
notebooklm login switch work
notebooklm auth storage set protected --profile work
```

## Command syntax and storage

- Existing top-level generation workflows retain their source-input syntax. New `<kind> create <notebook-id>` commands generate in an existing notebook.
- Existing `source rename <notebook> <source> <title>`, `research start <notebook> --query ...`, `share public --enable/--disable`, and `share invite --email ...` remain accepted.
- `studio delete <artifact-id>` retains the old single-artifact form. The two-argument form is `studio delete <notebook-id> <artifact-id>`; use repeated invocations for multiple artifact IDs to avoid ambiguity.
- On new routes, `--profile` selects an account. On legacy browser workflows it retains its Chrome profile directory meaning. Explicit `--session-path` overrides account selection.
- `--home` / `NOTEBOOKLM_HOME` isolate configuration. Named sessions use `profiles/<name>/session.json`; the default remains `session.json`. Aliases, tags and configuration use `cli.json`; custom YAML pipelines use `pipelines/`.
- Protected mode uses AES-GCM and an OS keyring key, with no plaintext fallback. Migration verifies the destination before removing old credentials. The Go encrypted format is independent of the Python implementation.
- Routes exposing `--confirm` prompt unless that flag is passed. Batch operations report partial failures with an error exit status. Uncertain report generation requests are not retried automatically.
- Object results use JSON. Text/content commands return plain text unless `--json` is selected. Output formatting is Go-native.

## Maintenance and verification

```sh
python scripts/generate_cli_catalog.py path/to/notebooklm-mcp-cli
go test ./...
go vet ./...
go build ./cmd/notebooklm
```

Tests exercise request payloads, chat ordering, report scope, Drive research import, command registration, local persistence, downloads and protected session migration. Server features require a valid Google session; offline checks do not establish live service compatibility.
