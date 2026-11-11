package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/download"
)

func artifactKind(raw any) string {
	switch api.At(raw, 2) {
	case float64(1):
		return "audio"
	case float64(2):
		return "report"
	case float64(3):
		return "video"
	case float64(4):
		switch api.At(raw, 9, 1, 0) {
		case float64(2):
			return "quiz"
		case float64(4):
			return "mind_map"
		}
		return "flashcards"
	case float64(7):
		return "infographic"
	case float64(8):
		return "slide_deck"
	case float64(9):
		return "data_table"
	case float64(10):
		if strings.Contains(strings.ToLower(api.StringAt(raw, 24, 0)), ".xlsx") || strings.Contains(api.StringAt(raw, 24, 1), "spreadsheetml") {
			return "data_table"
		}
		return "file"
	case float64(11):
		return "interactive_report"
	}
	return "file"
}

func downloadArtifact(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	kind := strings.TrimPrefix(in.schema.Canonical, "download ")
	id := in.val("id", "artifact_id")
	output := in.val("output")
	format := in.val("format")
	if kind == "all" {
		return downloadAll(ctx, c, in, nb)
	}
	if kind == "mind-map" {
		maps, err := api.ListMindMaps(ctx, c.CallBatchExecute, nb)
		if err != nil {
			return nil, err
		}
		for _, m := range maps {
			if id == "" || id == m.ID {
				if output == "" {
					output = nb + "_mindmap.json"
				}
				return output, writeOutput(output, m.Content)
			}
		}
		return nil, fmt.Errorf("no matching mind map")
	}
	raw, err := api.DecodedRPC(ctx, c.CallBatchExecute, "gArtLc", []any{[]any{2}, nb, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}, nb)
	if err != nil {
		return nil, err
	}
	wanted := strings.ReplaceAll(kind, "-", "_")
	if wanted == "slides" {
		wanted = "slide_deck"
	}
	var selected []any
	for _, v := range api.ArrayAt(raw, 0) {
		matches := artifactKind(v) == wanted
		if wanted == "file" && api.At(v, 2) == float64(10) {
			matches = true
		}
		if wanted == "report" && api.At(v, 2) == float64(11) {
			matches = true
		}
		if (id == "" || api.StringAt(v, 0) == id) && matches && api.At(v, 4) == float64(3) {
			selected = api.ArrayAt(v)
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no completed %s artifact matching %q", kind, id)
	}
	id = api.StringAt(selected, 0)
	if api.At(selected, 2) == float64(11) {
		if output == "" {
			output = nb + "_report.md"
		}
		return output, writeOutput(output, []byte(api.RenderReport(selected)))
	}
	if kind == "quiz" || kind == "flashcards" {
		if format == "" {
			format = "json"
		}
		html, err := c.GetInteractiveHTML(ctx, id)
		if err != nil {
			return nil, err
		}
		data, err := download.FormatInteractive(html, api.StringAt(selected, 1), kind, format)
		if err != nil {
			return nil, err
		}
		ext := format
		if ext == "markdown" {
			ext = "md"
		}
		if output == "" {
			output = nb + "_" + kind + "." + ext
		}
		return output, writeOutput(output, data)
	}
	if api.At(selected, 2) == float64(10) || kind == "file" {
		u, resolvedFormat, err := c.ResolveFileExport(ctx, api.StringAt(selected, 24, 3), api.StringAt(selected, 24, 2))
		if err != nil {
			return nil, err
		}
		if output == "" {
			output = nb + "_" + safeFilename(api.StringAt(selected, 24, 0))
			if strings.HasSuffix(output, "_") {
				output += "file.bin"
			}
		}
		if resolvedFormat == "pdf" && strings.HasSuffix(output, ".bin") {
			output = strings.TrimSuffix(output, ".bin") + ".pdf"
		}
		return c.DownloadFile(ctx, u, filepath.Dir(output), filepath.Base(output))
	}
	dir := "."
	if output != "" {
		dir = filepath.Dir(output)
	}
	if format == "" && wanted == "slide_deck" {
		format = "pdf"
	}
	path, err := c.DownloadArtifact(ctx, nb, id, dir, format)
	if err != nil {
		return nil, err
	}
	if output != "" && filepath.Clean(output) != filepath.Clean(path) {
		if err := os.Rename(path, output); err != nil {
			return nil, err
		}
		path = output
	}
	return path, nil
}

func safeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}

func downloadAll(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	notebooks := []string{}
	if nb != "" {
		notebooks = append(notebooks, nb)
	}
	if flagBool(in.cmd, "all-notebooks") {
		if nb != "" {
			return nil, fmt.Errorf("choose notebook or --all-notebooks")
		}
		all, err := c.ListNotebooks(ctx)
		if err != nil {
			return nil, err
		}
		for _, n := range all {
			notebooks = append(notebooks, n.ID)
		}
	}
	if len(notebooks) == 0 {
		return nil, fmt.Errorf("provide notebook or --all-notebooks")
	}
	base := in.val("output_dir", "output")
	if base == "" {
		base = "."
	}
	selectedTypes := csvValues(in.val("types"))
	slideFormat := in.val("slide_format")
	if slideFormat == "" {
		slideFormat = "pdf"
	}
	interactiveFormat := in.val("interactive_format")
	if interactiveFormat == "" {
		interactiveFormat = "json"
	}
	results := []any{}
	failures := 0
	for _, notebook := range notebooks {
		data, err := api.DecodedRPC(ctx, c.CallBatchExecute, "gArtLc", []any{[]any{2}, notebook, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}, notebook)
		if err != nil {
			return nil, err
		}
		for _, raw := range api.ArrayAt(data, 0) {
			if api.At(raw, 4) != float64(3) {
				continue
			}
			kind := artifactKind(raw)
			routeKind := strings.ReplaceAll(kind, "_", "-")
			if kind == "interactive_report" {
				routeKind = "report"
			}
			if len(selectedTypes) > 0 && !containsString(selectedTypes, kind) && !containsString(selectedTypes, routeKind) {
				continue
			}
			format := ""
			ext := map[string]string{"audio": "m4a", "video": "mp4", "report": "md", "interactive_report": "md", "infographic": "png", "data_table": "csv", "file": "bin"}[kind]
			if kind == "slide_deck" {
				format = slideFormat
				ext = format
			}
			if kind == "quiz" || kind == "flashcards" {
				format = interactiveFormat
				ext = format
				if ext == "markdown" {
					ext = "md"
				}
			}
			if api.At(raw, 2) == float64(10) {
				ext = filepath.Ext(api.StringAt(raw, 24, 0))
				ext = strings.TrimPrefix(ext, ".")
				if ext == "" {
					ext = "bin"
				}
			}
			target := filepath.Join(base, safeFilename(notebook), safeFilename(api.StringAt(raw, 0))+"."+ext)
			if flagBool(in.cmd, "skip-existing") {
				if _, err := os.Stat(target); err == nil {
					continue
				}
			}
			input := internalInput(in.cmd, "download "+routeKind, map[string]any{"notebook_id": notebook, "id": api.StringAt(raw, 0), "output": target})
			if format != "" {
				_ = input.cmd.Flags().Set("format", format)
			}
			value, err := downloadArtifact(ctx, c, input, notebook)
			item := map[string]any{"artifact_id": api.StringAt(raw, 0), "path": value}
			if err != nil {
				failures++
				item["error"] = err.Error()
			}
			results = append(results, item)
		}
		if len(selectedTypes) == 0 || containsString(selectedTypes, "mind_map") || containsString(selectedTypes, "mind-map") {
			maps, err := api.ListMindMaps(ctx, c.CallBatchExecute, notebook)
			if err != nil {
				return nil, err
			}
			for _, m := range maps {
				target := filepath.Join(base, safeFilename(notebook), safeFilename(m.ID)+".json")
				if flagBool(in.cmd, "skip-existing") {
					if _, err := os.Stat(target); err == nil {
						continue
					}
				}
				if err := writeOutput(target, m.Content); err != nil {
					return nil, err
				}
				results = append(results, map[string]any{"artifact_id": m.ID, "path": target})
			}
		}
	}
	if failures > 0 {
		_ = emitResult(in.cmd, results)
		return nil, fmt.Errorf("%d downloads failed", failures)
	}
	return results, nil
}
