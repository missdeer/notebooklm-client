# Accounts, usage, and recovery

## Authentication and accounts

```bash
notebooklm --version
notebooklm login
notebooklm notebook list --json
notebooklm login --profile work
notebooklm notebook list --profile work --json
notebooklm login profile list
notebooklm login switch work
notebooklm login --check --profile work
notebooklm auth refresh --profile work
notebooklm auth storage status --profile work
notebooklm auth storage set protected --profile work
```

Prefer `--profile work` for an individual command over changing the default unless requested. `config get default_profile` reads a saved default; the key may be absent before any switch. Config keys are literal: do not substitute Python-specific keys such as `auth.default_profile`.

`--home <dir>` / `NOTEBOOKLM_HOME` isolates configuration. The default session is `~/.notebooklm/session.json`; named sessions use `profiles/<name>/session.json`. Protected storage uses AES-GCM with a key in the OS keyring and has no plaintext fallback. Never print cookies, tokens, decrypted credentials, or keyring keys. Go's encrypted storage is independent of the Python client's format.

For an existing session or a host without a browser:

```bash
notebooklm import-cookies ./cookies.txt   # Netscape format; also Firefox/Safari profiles
notebooklm import-session ./session.json
notebooklm refresh-session
notebooklm session-status
notebooklm diagnose
notebooklm doctor
```

`export-session` remains available for browser sign-in. On grouped commands, `--profile` selects an account; on original browser workflows and `export-session`, it means a Chrome profile directory. `--session-path` overrides session selection.

## Usage, transport, and recovery

```bash
notebooklm usage --profile work --json
notebooklm config show --json
notebooklm doctor auth-replay --profile work
```

`usage` reports measured rolling/weekly percentages and reset times when supplied by the server. Go returns `resets_at` as epoch seconds; do not assume Python's ISO timestamps or fixed daily request caps.

Default `--transport auto` uses native uTLS. `--transport browser` launches Chrome through rod. `--proxy` is available on grouped network commands; original workflows also expose scheme-specific proxy flags. Environment fallbacks include `SOCKS5_PROXY`, `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY` and lowercase variants. Inspect help before using transport-specific flags.

| Situation | Response |
| --- | --- |
| No session / authentication failure | Check the account, try `auth refresh --profile <name>`, then `login --profile <name>` if needed. |
| Quota or rate limit | Inspect `usage`, report reset information, and stop repeated generation attempts. |
| Generation/download timeout | Inspect the exact artifact in Studio before retrying. |
| Unknown command or flag | Check help/version; use the current Go build rather than switching clients. |
| Partial batch/pipeline failure | Report completed items and failed steps; retry only remaining authorized work. |
| Keyring unavailable | Report the storage error; do not silently export credentials to plaintext. |
| RPC drift | Apply verified method-ID overrides in `rpc-ids.json` under the configured home; do not guess IDs. |

Return actual notebook/artifact IDs, relevant links, and verified output paths. Distinguish accepted generation requests from completed artifacts, and reported paths from successfully downloaded files.
