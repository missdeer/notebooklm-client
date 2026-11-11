# Chat and research

## Chat and saved conversations

```bash
notebooklm notebook query <notebook-id> "What are the main findings?" --json
notebooklm notebook query <notebook-id> "Explain further" --conversation-id <conversation-id>
notebooklm notebook query <notebook-id> "Compare these" --source-ids <id1>,<id2>
notebooklm notebook query <notebook-id> "Another topic" --new-conversation
notebooklm chat start <notebook-id>
notebooklm chat configure <notebook-id> --goal learning_guide --response-length longer
notebooklm chat configure <notebook-id> --goal custom --prompt "Cite evidence for each claim."
notebooklm chats list <notebook-id> --json
notebooklm chats get <notebook-id> <conversation-id> --json
notebooklm chats export <notebook-id> --conversation-id <conversation-id> --format json --output ./chat.json
notebooklm chats to-note <notebook-id> <conversation-id> --turn 1 --title "Finding"
notebooklm chats delete <notebook-id> --confirm
```

Interactive chat supports `/help`, `/new`, and `/exit`. `chats get` defaults to the latest conversation when its ID is omitted. Export formats are `md` and `json`. `chats delete` deletes the notebook's chat history, not one turn. Chat configuration persists server-side.

## Research

```bash
notebooklm research start "topic" --notebook-id <notebook-id> --mode fast
notebooklm research start "topic" --title "New research" --mode deep --auto-import
notebooklm research start "internal topic" --notebook-id <notebook-id> --source drive
notebooklm research status <notebook-id> --max-wait 0
notebooklm research status <notebook-id> --task-id <task-id> --max-wait 900 --poll-interval 30
notebooklm research import <notebook-id> <task-id> --timeout 600
notebooklm research import <notebook-id> <task-id> --indices 0,2
notebooklm research import <notebook-id> <task-id> --cited-only
```

Deep research is Web-only. Start requires a notebook or `--title`; `--auto-import` waits and imports results. Status polls for up to 300 seconds by default; `--max-wait 0` checks once. Import can discover the task when its ID is omitted. Indices are zero-based. Inspect active tasks before using `--force`. Distinguish polling timeouts from server-side failures.
