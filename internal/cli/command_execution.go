package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/spf13/cobra"
)

func runCommand(cmd *cobra.Command, s commandSchema, args []string) error {
	in := commandInput{cmd, s, args}
	if cmd.Flags().Lookup("confirm") != nil && !flagBool(cmd, "confirm") {
		fmt.Fprintf(cmd.ErrOrStderr(), "Run %s? [y/N] ", s.Canonical)
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if !scanner.Scan() {
			return fmt.Errorf("confirmation required; pass --confirm for unattended use")
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("cancelled")
		}
	}
	if handled, err := runLocalCommand(in); handled {
		return err
	}
	if timeout := flagString(cmd, "timeout"); timeout != "" {
		if seconds, e := time.ParseDuration(timeout + "s"); e == nil && seconds > 0 {
			ctx, cancel := context.WithTimeout(cmd.Context(), seconds)
			defer cancel()
			cmd.SetContext(ctx)
		}
	}
	return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
		value, err := runRemoteCommand(ctx, c, in)
		if err != nil {
			return err
		}
		if value == nil {
			return nil
		}
		return emitResult(cmd, value)
	})
}

func emitResult(cmd *cobra.Command, v any) error {
	if s, ok := v.(string); ok && !flagBool(cmd, "json") {
		_, e := fmt.Fprintln(cmd.OutOrStdout(), s)
		return e
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func resolveResource(id string) (string, error) {
	state, err := readCLIState()
	if err != nil {
		return "", err
	}
	if v := state.Aliases[id]; v != "" {
		return v, nil
	}
	return id, nil
}

func runRemoteCommand(ctx context.Context, c *client.NotebookClient, in commandInput) (any, error) {
	cmd := in.cmd
	path := in.schema.Canonical
	call := api.RpcCaller(c.CallBatchExecute)
	nb, err := resolveResource(in.val("notebook_id", "notebook"))
	if err != nil {
		return nil, err
	}
	sourceID, err := resolveResource(in.val("source_id"))
	if err != nil {
		return nil, err
	}
	artifactID, err := resolveResource(in.val("artifact_id"))
	if err != nil {
		return nil, err
	}
	switch path {
	case "notebook list":
		notebooks, err := c.ListNotebooks(ctx)
		if err != nil {
			return nil, err
		}
		if flagBool(cmd, "quiet") || flagBool(cmd, "title") {
			lines := []string{}
			for _, n := range notebooks {
				v := n.ID
				if flagBool(cmd, "title") {
					v = n.Title
				}
				lines = append(lines, v)
			}
			return strings.Join(lines, "\n"), nil
		}
		return notebooks, nil
	case "notebook create":
		id, err := c.CreateNotebook(ctx)
		if err != nil {
			return nil, err
		}
		title := in.val("title")
		if title != "" {
			err = c.RenameNotebook(ctx, id, title)
		}
		return map[string]any{"notebook_id": id, "title": title}, err
	case "notebook get":
		title, sources, err := c.GetNotebookDetail(ctx, nb)
		return map[string]any{"notebook_id": nb, "title": title, "sources": sources}, err
	case "notebook describe":
		return c.GetNotebookSummary(ctx, nb)
	case "notebook rename":
		return success(c.RenameNotebook(ctx, nb, in.val("new_title", "title")))
	case "notebook delete":
		return success(c.DeleteNotebook(ctx, nb))
	case "notebook query":
		return queryNotebook(ctx, c, in, nb)
	case "chat configure":
		return success(c.ConfigureChat(ctx, nb, in.val("goal"), in.val("prompt", "custom_prompt"), in.val("response_length")))
	case "chat start":
		return nil, chatREPL(ctx, c, in, nb)
	case "source list":
		return listSources(ctx, c, in, nb)
	case "source get", "source content":
		content, err := c.GetSourceContent(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		if path == "source content" {
			if output := in.val("output"); output != "" {
				return output, writeOutput(output, []byte(content.Content))
			}
			if !flagBool(cmd, "json") {
				return content.Content, nil
			}
		}
		return content, nil
	case "source describe":
		data, err := api.DecodedRPC(ctx, c.CallBatchExecute, rpc.GetSourceSummary, []any{[]any{[]any{[]any{sourceID}}}}, "")
		if err != nil {
			return nil, err
		}
		return map[string]any{"source_id": sourceID, "summary": api.At(data, 1), "keywords": api.At(data, 2)}, nil
	case "source add":
		return addSource(ctx, c, in, nb)
	case "source rename":
		return success(c.RenameSource(ctx, nb, sourceID, in.val("new_title", "title")))
	case "source delete":
		ids := csvValues(in.val("source_ids", "source_id"))
		for _, id := range ids {
			id, err = resolveResource(id)
			if err != nil {
				return nil, err
			}
			if err := c.DeleteSource(ctx, id); err != nil {
				return nil, err
			}
		}
		return success(nil)
	case "source stale", "source sync":
		return syncSources(ctx, c, in, nb)
	case "note list":
		notes, err := c.ListNotes(ctx, nb)
		if err != nil {
			return nil, err
		}
		if flagBool(cmd, "quiet") {
			ids := []string{}
			for _, n := range notes {
				ids = append(ids, n.ID)
			}
			return strings.Join(ids, "\n"), nil
		}
		return notes, nil
	case "note create":
		id, err := c.CreateNote(ctx, nb, in.val("title"), in.val("content"))
		return map[string]string{"note_id": id}, err
	case "note update":
		return success(c.UpdateNote(ctx, nb, in.val("note_id"), in.val("content"), in.val("title")))
	case "note delete":
		return success(c.DeleteNote(ctx, nb, in.val("note_id")))
	case "studio status":
		all, err := c.GetArtifacts(ctx, nb)
		if err != nil {
			return nil, err
		}
		out := []types.ArtifactInfo{}
		filter := in.val("artifact_id")
		for _, a := range all {
			if filter == "" || a.ID == filter {
				out = append(out, a)
			}
		}
		offset := flagInt(cmd, "offset", 0)
		if offset < 0 {
			return nil, fmt.Errorf("offset must be nonnegative")
		}
		if offset > len(out) {
			offset = len(out)
		}
		out = out[offset:]
		limit := flagInt(cmd, "limit", 0)
		if limit > 0 && limit < len(out) {
			out = out[:limit]
		}
		return out, nil
	case "studio delete":
		return success(c.DeleteArtifact(ctx, artifactID))
	case "studio rename":
		return success(api.RenameArtifact(ctx, call, artifactID, in.val("new_title", "title")))
	case "slides revise":
		return reviseSlides(ctx, c, in)
	case "video list":
		all, err := c.GetArtifacts(ctx, nb)
		out := []types.ArtifactInfo{}
		for _, a := range all {
			if a.Type == 3 {
				out = append(out, a)
			}
		}
		return out, err
	case "share status":
		return c.GetShareStatus(ctx, nb)
	case "share public":
		return success(c.ShareNotebookPublic(ctx, nb, true))
	case "share private":
		return success(c.ShareNotebookPublic(ctx, nb, false))
	case "share invite":
		return success(c.ShareNotebookWithUser(ctx, nb, in.val("email"), in.val("role", "permission"), true, ""))
	case "share batch":
		for _, email := range csvValues(in.val("emails")) {
			if err := c.ShareNotebookWithUser(ctx, nb, email, in.val("role"), true, ""); err != nil {
				return nil, err
			}
		}
		return success(nil)
	case "usage":
		return api.Usage(ctx, call)
	}
	if strings.HasPrefix(path, "label ") {
		action := strings.TrimPrefix(path, "label ")
		values := []string{in.val("label_id"), in.val("new_name", "emoji", "source_id")}
		if action == "create" {
			values = []string{in.val("name"), in.val("emoji")}
		}
		if action == "delete" {
			values = csvValues(in.val("label_ids", "label_id"))
		}
		return api.ManageLabels(ctx, call, nb, action, values, flagBool(cmd, "unlabeled"))
	}
	if strings.HasPrefix(path, "chats ") {
		return manageChats(ctx, c, in, nb)
	}
	if strings.HasPrefix(path, "research ") {
		return runResearch(ctx, c, in, nb)
	}
	if strings.HasPrefix(path, "download ") {
		return downloadArtifact(ctx, c, in, nb)
	}
	if strings.HasPrefix(path, "export ") {
		format := in.val("type", "format")
		if path == "export to-docs" {
			format = "docs"
		}
		if path == "export to-sheets" {
			format = "sheets"
		}
		if format == "" {
			format = "docs"
		}
		return c.ExportArtifact(ctx, nb, artifactID, in.val("title"), format)
	}
	if strings.HasPrefix(path, "report ") && path != "report create" {
		return manageReport(ctx, c, in, nb)
	}
	if strings.HasPrefix(path, "batch ") || path == "cross query" || path == "pipeline run" {
		return runAutomation(ctx, c, in)
	}
	if strings.HasSuffix(path, " create") {
		return createArtifact(ctx, c, in, nb)
	}
	return nil, fmt.Errorf("unhandled command route %s", path)
}

func success(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func writeOutput(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func waitUntil(ctx context.Context, timeout time.Duration, check func() (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
