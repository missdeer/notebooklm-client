# Aliases, tags, batches, and pipelines

```bash
notebooklm alias set research <notebook-id>
notebooklm alias get research
notebooklm alias list
notebooklm tag add research --tags ai,research
notebooklm tag remove research --tags ai
notebooklm tag list
notebooklm tag select "ai research"
notebooklm batch query "Summarize findings" --tags research
notebooklm cross query "Where do studies disagree?" --notebooks <id1>,<id2>
notebooklm batch add-source "https://example.com" --notebooks <id1>,<id2>
notebooklm batch create "Topic A,Topic B"
notebooklm batch studio audio --tags research
notebooklm pipeline list
notebooklm pipeline run ingest-and-podcast --notebook <notebook-id> --input-url "https://example.com"
notebooklm pipeline create my-pipeline --file ./pipeline.yaml
```

Grouped commands resolve aliases where implemented; original top-level workflows can still require full IDs. Tags are local; use lowercase tags to match `tag select` queries. Batches can partially succeed and return a nonzero status. Read per-item results before retrying; do not repeat successful mutations.

Built-in pipelines: `ingest-and-podcast`, `research-and-report`, `multi-format`. Custom YAML uses `steps` with `action` and `params`; supported actions are `source_add`, `notebook_query`, `studio_create`, `notebook_create`, `notebook_delete`. `$INPUT_URL` comes from `--input-url`. Inspect steps before execution so their mutations stay within the requested scope.
