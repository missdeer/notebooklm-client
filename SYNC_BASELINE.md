# Sync Baselines

The Go implementation tracks two upstream references:

1. **`notebooklm-client-ts/`** — primary source of truth for the
   reverse-engineered RPC protocol, transport tiers, session format, and
   workflow layering. Feature parity is expected.
2. **`notebooklm-mcp-cli/`** (Python) — secondary reference for the CLI
   /MCP-tool surface. Useful for discovering operations to expose
   (notebook CRUD, source CRUD, notes, sharing, research import,
   studio_revise, export_artifact, etc.) and for cross-checking payload
   field positions when a TS port hasn't been done yet. Protocol parity
   is *not* required — only feature inspiration.

## TypeScript baseline

Go was last synced against this TS commit. Use
`git -C notebooklm-client-ts log <commit>..HEAD` to find unported changes.

| Field  | Value |
|--------|-------|
| Commit | `7319b6c` |
| Message | `feat(rpc): add 5 missing RPC IDs from upstream (#26)` |
| Date   | 2026-05-27 |

Last verified against TS HEAD: 2026-06-14 — no new TS commits to port.

## Python (notebooklm-mcp-cli) reference snapshot

Used as inspiration for the Go-only CLI expansion in commit `09ce672`.
Bump this entry when re-surveying the Python project for new MCP tools
or RPC field discoveries.

| Field  | Value |
|--------|-------|
| Commit | `6d41c75` |
| Message | `release: v0.7.3` |
| Date   | 2026-06-09 |

Use `git -C notebooklm-mcp-cli log <commit>..HEAD --oneline` to see new
features added since this snapshot. Reach for `core/client.py` and the
`services/` layer when porting; payload shapes there are well-commented
and easier to follow than the TS positional arrays.

## Go-only divergence (not present in TS)

The Go CLI now exposes operations beyond the TS surface. The underlying
RPCs are shared, but the user-facing commands originated in Go and
should not be reverse-ported expectations against TS.

- `create` / `rename` (notebook)
- `source list` / `source delete` / `source rename` / `source refresh` / `source summary`
- `note` subcommand (create / list / update / delete)
- `share` subcommand (status / public / invite)
- `research` subcommand (start / status / import)

Inspiration came from the Python `notebooklm-mcp-cli` project's tool
surface; the API-layer wrappers in `internal/api/*.go` already existed
from prior TS syncs, so this was purely CLI plumbing.
