# Notebooks, sources, notes, and sharing

## Notebooks, sources, and notes

```bash
notebooklm notebook create "Research" --json
notebooklm notebook list --json
notebooklm notebook get <notebook-id> --json
notebooklm notebook describe <notebook-id> --json
notebooklm notebook rename <notebook-id> "New title"
notebooklm notebook delete <notebook-id> --confirm

notebooklm source add <notebook-id> --url "https://example.com" --wait
notebooklm source add <notebook-id> --url "https://a.example" --url "https://b.example" --wait
notebooklm source add <notebook-id> --file ./paper.pdf --wait
notebooklm source add <notebook-id> --text "Notes..." --title "My notes"
notebooklm source add <notebook-id> --drive <document-id> --type slides --wait
notebooklm source list <notebook-id> --json
notebooklm source get <source-id> --json
notebooklm source describe <source-id> --json
notebooklm source content <source-id> --output ./source.txt
notebooklm source rename <source-id> "New title" --notebook <notebook-id>
notebooklm source stale <notebook-id>
notebooklm source sync <notebook-id> --confirm
notebooklm source delete <source-id> --confirm

notebooklm note list <notebook-id> --json
notebooklm note create <notebook-id> --title "Takeaways" --content "..."
notebooklm note update <notebook-id> <note-id> --content "..."
notebooklm note delete <notebook-id> <note-id> --confirm
```

`source add` accepts one source category per call; URL/YouTube flags can repeat. Drive types are `doc`, `slides`, `sheets`, and `pdf`. `--wait-timeout` adjusts processing waits. `source content` returns indexed text; `source describe` requests an AI summary. Unknown freshness does not establish that a Drive source is stale.

Source labels are server-side groups; notebook tags are local selection metadata:

```bash
notebooklm label list <notebook-id> --json
notebooklm label auto <notebook-id>
notebooklm label create <notebook-id> "Methods" --emoji "📚"
notebooklm label move <notebook-id> <source-id> <label-id>
notebooklm label rename <notebook-id> <label-id> "New name"
notebooklm label emoji <notebook-id> <label-id> "🔬"
notebooklm label reorganize <notebook-id> --unlabeled --confirm
notebooklm label delete <notebook-id> <label-id> --confirm
```

`label list` can trigger auto-labeling when no labels exist; treat it as potentially modifying organization. Moving a source adds label membership. Deleting a label does not delete its sources.

## Sharing

```bash
notebooklm share status <notebook-id> --json
notebooklm share public <notebook-id>
notebooklm share private <notebook-id>
notebooklm share invite <notebook-id> collaborator@example.com --role viewer
notebooklm share batch <notebook-id> "a@example.com,b@example.com" --role editor
```

Verify notebook, recipients, and permissions against the user's request before sharing.
