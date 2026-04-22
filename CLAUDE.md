# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Dual-implementation (TypeScript + Go) client for Google NotebookLM. Reverse-engineers the undocumented `batchexecute` RPC protocol to programmatically create notebooks, add sources, generate artifacts (audio podcasts, reports, slides, quizzes, videos, infographics, data tables, flashcards), and download results.

- **TypeScript** (`notebooklm-client-ts/`): The original, full-featured implementation. Published as npm package `notebooklm-client`. See `notebooklm-client-ts/CLAUDE.md` for TS-specific details.
- **Go** (root): Rewrite targeting single static binary. Should stay aligned with the TypeScript version — TS is the source of truth for features, behavior, and protocol details.

Both share the same CLI command surface, session format (`~/.notebooklm/session.json`), and architectural layering.

## Commands

### Go

```bash
go build -o notebooklm ./cmd/notebooklm    # Build
go test ./...                                # Run all tests
go test ./internal/parser/...                # Single package tests
```

### TypeScript (run from notebooklm-client-ts/)

```bash
npm run build          # tsc → dist/
npm run dev -- <args>  # Run CLI via tsx without building
npm test               # Unit tests (vitest, excludes e2e)
npm run test:e2e       # E2E tests (sequential, 2min timeout, needs real session)
npx vitest run tests/parser.test.ts              # Single test file
npx vitest run tests/parser.test.ts -t "name"    # Single test by name
```

## Architecture

Both implementations follow the same layered design:

```
CLI (Commander / Cobra)
  → NotebookClient (public API, transport-agnostic)
    → Workflows (orchestration: create → add source → generate → poll → download)
      → API (stateless RPC builders & parsers, take RpcCaller callback)
        → Transport (pluggable HTTP execution)
```

### Transport tiers (auto-selectable via `--transport`)

| Tier | TS implementation | Go implementation | TLS fidelity |
|------|------------------|-------------------|-------------|
| browser | Puppeteer (rebrowser) | rod | 100% |
| curl | curl-impersonate subprocess | curl-impersonate subprocess | 100% |
| tls-client | Go uTLS via FFI | — | 99% |
| http | Node.js undici | utls native | ~40% (TS) / 99% (Go) |
| auto | Best available non-browser | Best available non-browser | — |

`transport-resolver.ts` (TS) and `internal/transport/resolver.go` (Go) handle auto-detection.

### RPC protocol

- Endpoint: `notebooklm.google.com/_/LabsTailwindUi/data/batchexecute`
- Payloads are deeply nested positional arrays (no named keys)
- Responses use a binary envelope format parsed by `boq-parser` then per-operation parsers
- RPC IDs are static strings that may change; dynamic overrides supported via `~/.notebooklm/rpc-ids.json`

### Session management

- Persisted to `~/.notebooklm/session.json` (override: `--home` flag or `NOTEBOOKLM_HOME` env var)
- Tokens: `at` (access, ~1-2h), `bl` (batch ID), `fsid` (filesystem ID), plus long-lived cookies (weeks/months)
- Auto-refresh on 401 with refresh guard (prevents thundering herd on concurrent requests)
- Session format is compatible between TS and Go versions

### Workflow pattern (both implementations)

1. Create notebook
2. Add source (URL / file / text / research)
3. Navigate to studio (get config)
4. Generate artifact with type-specific payload
5. Poll with exponential backoff + jitter until ready
6. Download result
7. Clean up (delete notebook unless `--keep-notebook`)

## Go project structure

```
cmd/notebooklm/main.go          Entry point
internal/
├── cli/          Cobra commands (mirrors TS cli.ts)
├── client/       NotebookClient + workflow orchestration
├── api/          Stateless RPC functions
├── parser/       BOQ protocol + RPC response parsing
├── payload/      Per-artifact payload builders
├── transport/    Transport interface + utls/curl/rod implementations
├── session/      Session persistence + token refresh
├── download/     File/audio download + CDN retry
├── rpc/          RPC IDs, URLs, config overrides
├── types/        Domain types, enums, errors
└── util/         Jitter/sleep, refresh guard
```

## Key constraints

- **RPC IDs are fragile** — captured from live traffic, may change without notice. Both projects support runtime overrides via `~/.notebooklm/rpc-ids.json`
- **TypeScript is ESM-only** (`"type": "module"`), strict mode, `noUncheckedIndexedAccess`, Node 20+, all internal imports use `.js` extension
- **`puppeteer-core` is aliased** to `rebrowser-puppeteer-core` in TS package.json
- **Artifact type codes** are numeric enums (1=audio, 2=report, 3=video, etc.) shared across both implementations
- **Parser helpers** (`get`, `getString`, `getArray`) provide safe nested array access for the fragile positional-array RPC responses

## Adding a new artifact type

1. Add type enum/constant to types (TS: `types.ts`, Go: `types/`)
2. Add RPC ID to rpc-ids (TS: `rpc-ids.ts`, Go: `rpc/`)
3. Add payload builder (TS: `artifact-payloads.ts`, Go: `payload/`)
4. Add workflow function (TS: `workflows.ts`, Go: `client/`)
5. Add CLI command (TS: `cli.ts`, Go: `cli/`)
6. Add parser if response format differs (TS: `parser.ts`, Go: `parser/`)
