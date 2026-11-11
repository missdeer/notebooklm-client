# Studio artifacts

## Generate Studio artifacts

`<kind> create` uses an existing notebook and returns an artifact ID. It does not automatically download the result: poll `studio status`, then download that exact artifact.

| Artifact | Example after `notebooklm` |
| --- | --- |
| Audio | `audio create <notebook-id> --format deep_dive --length default --focus "Compare findings" --confirm` |
| Report | `report create <notebook-id> --format "Study Guide" --confirm` |
| Custom report | `report create <notebook-id> --format "Create Your Own" --prompt "Write an evidence-based SWOT analysis" --confirm` |
| Slides | `slides create <notebook-id> --format presenter_slides --length short --focus "10-minute overview" --confirm` |
| Video | `video create <notebook-id> --format explainer --style whiteboard --focus "Explain the method" --confirm` |
| Quiz | `quiz create <notebook-id> --count 10 --difficulty 3 --focus "Exam preparation" --confirm` |
| Flashcards | `flashcards create <notebook-id> --difficulty medium --focus "Key terms" --confirm` |
| Infographic | `infographic create <notebook-id> --orientation square --detail concise --style bento_grid --confirm` |
| Data table | `data-table create <notebook-id> "Columns: study, method, sample size, result, limitations" --confirm` |
| Mind map | `mindmap create <notebook-id> --title "Topic map" --confirm` |

Use `--source-ids` for a subset. Check help for `--language` support on the particular kind. Quiz difficulty is numeric `1..5`; flashcard difficulty uses `easy`, `medium`, or `hard`. Data-table description is a required positional argument.

For a clear request, choose sensible defaults and put the requirements in the appropriate prompt: `--focus` for audio/video/slides/infographics/quizzes/flashcards, `--prompt` for custom reports, and positional description for data tables. Clarify only choices that materially change the result. Audio formats are `deep_dive`, `brief`, `critique`, and `debate`; slide formats are `detailed_deck` and `presenter_slides`. Inspect help for other formats and styles.

### Interactive reports

```bash
notebooklm report create <notebook-id> --format Interactive --template learning_overview --confirm
notebooklm studio status <notebook-id> --json
notebooklm report get <notebook-id> <report-id> --output ./lesson.md
notebooklm report elements <notebook-id> <report-id> --json
notebooklm report element create <notebook-id> <report-id> --id <element-id> --confirm
notebooklm report elements <notebook-id> <report-id> --wait <element-id> --timeout 600 --content --json
notebooklm report element create-batch <notebook-id> <report-id> --plan ./elements.json --confirm
```

Embedded elements begin as suggestions and require separate generation. Inspect their IDs, types, descriptions, and status; keep their source scope tied to the report. A plan is a JSON array such as `[{"element_id":"...","steering_prompt":"...","settings":{"difficulty":"medium"}}]`; settings must apply to the element kind. A lost kickoff response is uncertain: check status before retrying to avoid duplicate generation.

## Downloads, revisions, and exports

```bash
notebooklm studio status <notebook-id> --json
notebooklm studio rename <artifact-id> "New title"
notebooklm download audio <notebook-id> --id <artifact-id> --output ./podcast.m4a
notebooklm download slide-deck <notebook-id> --id <artifact-id> --format pptx --output ./slides.pptx
notebooklm download quiz <notebook-id> --id <artifact-id> --format markdown --output ./quiz.md
notebooklm download mind-map <notebook-id> --id <mind-map-id> --output ./map.json
notebooklm download all <notebook-id> --output-dir ./downloads --skip-existing
notebooklm slides revise <artifact-id> --slide "1 Tighten the introduction" --confirm
notebooklm export to-docs <notebook-id> <artifact-id> --title "Brief"
notebooklm export to-sheets <notebook-id> <artifact-id> --title "Comparison"
notebooklm studio delete <notebook-id> <artifact-id> --confirm
```

Use `--id` for a particular artifact; omitting it chooses a completed artifact of that kind. `download <kind> --output` is a **file path**. `download all --output-dir` is a **directory** and supports type/format choices and `--all-notebooks`. Mind maps use JSON; quizzes/flashcards support JSON/Markdown/HTML; data tables can be CSV/XLSX; `download file` handles generic Studio file exports.

Original `studio download <notebook-id> <artifact-id> -o <directory>` requires an ID and uses an output **directory**. Its slide `auto` format prefers PPTX and falls back to PDF; `download slide-deck` defaults to PDF. Downloads do not transcode files; do not rename an extension to imply conversion.

`slides revise --slide "1 ..."` uses **one-based** slide numbers and creates a new deck. Original `studio revise --slide "0:..."` uses **zero-based** indices. Do not mix them.

`studio status` JSON uses Go fields such as `ID`, `Title`, `Type`, and `StatusCode`. Codes: `1` in progress, `2` queued, `3` completed, `4` failed, `5` suggested. Audio may expose playable media at code `2`. Use actual IDs and available URLs; do not substitute unrelated artifacts when an exact match is missing.

## One-shot workflows and original commands

```bash
notebooklm audio --url "https://example.com/article" -o ./audio --format debate --length short
notebooklm report --file ./paper.pdf -o ./report --template study_guide
notebooklm slides --topic "machine learning basics" -o ./slides --format presenter --length short
notebooklm analyze --url "https://example.com/paper.pdf" --question "What are the findings?"
notebooklm chat <notebook-id> --question "What does the paper claim?" --with-citations
```

Top-level generation takes exactly one of `--url`, `--text`, `--file`, `--topic`; `--research-mode` selects fast/deep for a topic. Output directories default to `.` where supported. These commands use `--instructions` rather than `--focus`; report templates are `briefing_doc`, `study_guide`, `blog_post`, `custom`. Original slides use `detailed`/`presenter` and can take `--notebook-id` plus optional `--source-ids` instead of a new source.

Original `list`, `detail`, `create`, `rename`, `delete`, `describe`, `source refresh/summary` remain available. Do not mix their syntax with grouped commands. `create [title]` prints a bare ID; `notebook create` returns an object with `notebook_id`. Object results may use Go field names or explicit JSON tags; inspect before scripting against them.
