package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func notebookSources(ctx context.Context, c *client.NotebookClient, nb string, selected string) ([]string, error) {
	_, sources, err := c.GetNotebookDetail(ctx, nb)
	if err != nil {
		return nil, err
	}
	ids := csvValues(selected)
	if len(ids) == 0 {
		for _, s := range sources {
			ids = append(ids, s.ID)
		}
	} else {
		valid := map[string]bool{}
		for _, s := range sources {
			valid[s.ID] = true
		}
		for _, id := range ids {
			if !valid[id] {
				return nil, fmt.Errorf("source %s does not belong to notebook %s", id, nb)
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("notebook has no sources")
	}
	return ids, nil
}

func addSource(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	urls := append(flagStrings(in.cmd, "url"), flagStrings(in.cmd, "youtube")...)
	if len(urls) == 0 {
		if u := in.arg("url_arg"); u != "" {
			urls = append(urls, u)
		}
	}
	text, file, drive := in.val("text"), in.val("file"), in.val("drive")
	count := 0
	for _, b := range []bool{len(urls) > 0, text != "", file != "", drive != ""} {
		if b {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("choose exactly one of --url, --youtube, --text, --file, --drive")
	}
	ids := []string{}
	out := []any{}
	add := func(id, title string, err error) error {
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("source registration returned no ID")
		}
		ids = append(ids, id)
		out = append(out, map[string]string{"source_id": id, "title": title})
		return nil
	}
	for _, u := range urls {
		if err := add(c.AddURLSource(ctx, nb, u)); err != nil {
			return nil, err
		}
	}
	if text != "" {
		if err := add(c.AddTextSource(ctx, nb, in.val("title"), text)); err != nil {
			return nil, err
		}
	}
	if file != "" {
		if err := add(c.AddFileSource(ctx, nb, file)); err != nil {
			return nil, err
		}
	}
	if drive != "" {
		mime := map[string]string{"doc": "application/vnd.google-apps.document", "slides": "application/vnd.google-apps.presentation", "sheets": "application/vnd.google-apps.spreadsheet", "pdf": "application/pdf"}[in.val("type", "doc_type")]
		if mime == "" {
			return nil, fmt.Errorf("Drive type must be doc, slides, sheets or pdf")
		}
		if err := add(api.AddDriveSource(ctx, c.CallBatchExecute, nb, drive, in.val("title"), mime)); err != nil {
			return nil, err
		}
	}
	if flagBool(in.cmd, "wait") {
		err := waitUntil(ctx, time.Duration(flagInt(in.cmd, "wait-timeout", 600))*time.Second, func() (bool, error) {
			_, sources, err := c.GetNotebookDetail(ctx, nb)
			if err != nil {
				return false, err
			}
			ready := 0
			for _, s := range sources {
				for _, id := range ids {
					if s.ID == id && s.StatusCode != nil {
						if *s.StatusCode == 3 {
							return false, fmt.Errorf("source %s failed processing", id)
						}
						if *s.StatusCode == 2 {
							ready++
						}
					}
				}
			}
			return ready == len(ids), nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func syncSources(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	ids := csvValues(in.val("source_ids"))
	if len(ids) == 0 {
		_, sources, err := c.GetNotebookDetail(ctx, nb)
		if err != nil {
			return nil, err
		}
		for _, s := range sources {
			ids = append(ids, s.ID)
		}
	}
	out := []any{}
	for _, id := range ids {
		if in.val("source_ids") == "" {
			content, err := c.GetSourceContent(ctx, id)
			if err != nil {
				return nil, err
			}
			if content.SourceType != 1 && content.SourceType != 2 {
				continue
			}
		}
		fresh, err := api.SourceFreshness(ctx, c.CallBatchExecute, id)
		if err != nil {
			return nil, err
		}
		if in.schema.Canonical == "source sync" && (in.val("source_ids") != "" || (fresh != nil && !*fresh)) {
			if _, err := api.DecodedRPC(ctx, c.CallBatchExecute, rpc.RefreshSource, []any{nil, []any{id}, []any{2}}, nb); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"source_id": id, "synced": true})
		} else if fresh != nil && !*fresh {
			out = append(out, map[string]any{"source_id": id, "stale": true})
		}
	}
	return out, nil
}

func listSources(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	_, sources, err := c.GetNotebookDetail(ctx, nb)
	if err != nil {
		return nil, err
	}
	if flagBool(in.cmd, "quiet") || flagBool(in.cmd, "url") {
		lines := []string{}
		for _, s := range sources {
			v := s.ID
			if flagBool(in.cmd, "url") {
				v = s.URL
			}
			lines = append(lines, v)
		}
		return strings.Join(lines, "\n"), nil
	}
	if !flagBool(in.cmd, "drive") && !flagBool(in.cmd, "full") {
		return sources, nil
	}
	out := []any{}
	for _, s := range sources {
		content, err := c.GetSourceContent(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		drive := content.SourceType == 1 || content.SourceType == 2
		if flagBool(in.cmd, "drive") && !drive {
			continue
		}
		item := map[string]any{"source_id": s.ID, "title": s.Title, "url": content.URL, "type": content.SourceType, "word_count": s.WordCount}
		if drive && !flagBool(in.cmd, "skip-freshness") {
			fresh, err := api.SourceFreshness(ctx, c.CallBatchExecute, s.ID)
			if err != nil {
				return nil, err
			}
			item["fresh"] = fresh
		}
		out = append(out, item)
	}
	return out, nil
}
