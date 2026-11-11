package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/rpc"
)

type ResearchSource struct {
	Index       int    `json:"index"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        int    `json:"result_type"`
}
type ResearchTask struct {
	ID         string           `json:"task_id"`
	Status     string           `json:"status"`
	Query      string           `json:"query"`
	SourceType string           `json:"source_type"`
	Mode       string           `json:"mode"`
	Sources    []ResearchSource `json:"sources"`
	Summary    string           `json:"summary"`
	Report     string           `json:"report"`
}

func ResearchTasks(ctx context.Context, call RpcCaller, nb string) ([]ResearchTask, error) {
	data, err := DecodedRPC(ctx, call, rpc.PollResearch, []any{nil, nil, nb}, nb)
	if err != nil {
		return nil, err
	}
	return ParseResearchTasks(data), nil
}
func ParseResearchTasks(data []any) []ResearchTask {
	entries := data
	if ArrayAt(data, 0, 0) != nil {
		entries = ArrayAt(data, 0)
	}
	out := []ResearchTask{}
	for _, entry := range entries {
		id := StringAt(entry, 0)
		info := At(entry, 1)
		if id == "" || ArrayAt(info) == nil {
			continue
		}
		task := ResearchTask{ID: id, Status: "in_progress", Query: StringAt(info, 1, 0), SourceType: "web", Mode: "fast", Sources: []ResearchSource{}, Summary: StringAt(info, 3, 1)}
		if At(info, 1, 1) == float64(2) {
			task.SourceType = "drive"
		}
		if At(info, 2) == float64(5) {
			task.Mode = "deep"
		}
		if code := At(info, 4); code == float64(2) || code == float64(6) {
			task.Status = "completed"
		}
		for i, src := range ArrayAt(info, 3, 0) {
			kind := 1
			if code, ok := At(src, 3).(float64); ok {
				kind = int(code)
			}
			source := ResearchSource{Index: i, URL: StringAt(src, 0), Title: StringAt(src, 1), Description: StringAt(src, 2), Type: kind}
			if source.URL != "" {
				task.Sources = append(task.Sources, source)
			}
			if text := StringAt(src, 6, 0); text != "" {
				task.Report = text
			}
			if At(src, 0) == nil && ArrayAt(src, 1) != nil {
				task.Report = StringAt(src, 1, 1)
			}
		}
		out = append(out, task)
	}
	return out
}
func SelectResearchTask(tasks []ResearchTask, id string) (ResearchTask, error) {
	if len(tasks) == 0 {
		return ResearchTask{}, fmt.Errorf("no research task found")
	}
	if id != "" {
		for _, t := range tasks {
			if t.ID == id {
				return t, nil
			}
		}
		if len(tasks) == 1 {
			return tasks[0], nil
		}
		return ResearchTask{}, fmt.Errorf("research task %s not found among multiple tasks", id)
	}
	return tasks[0], nil
}
func ImportTaskSources(ctx context.Context, call RpcCaller, nb string, task ResearchTask, indices []int, citedOnly bool) error {
	sources := []any{}
	if task.Report != "" {
		sources = append(sources, []any{nil, []any{"Deep Research Report", task.Report}, nil, 3, nil, nil, nil, nil, nil, nil, 3})
	}
	for _, s := range task.Sources {
		if len(indices) > 0 {
			selected := false
			for _, i := range indices {
				selected = selected || s.Index == i
			}
			if !selected {
				continue
			}
		}
		if citedOnly && !strings.Contains(task.Report, s.URL) {
			continue
		}
		if s.URL == "" || s.Type == 5 {
			continue
		}
		source := []any{nil, nil, []any{s.URL, s.Title}, nil, nil, nil, nil, nil, nil, nil, 2}
		if s.Type != 1 {
			u, err := url.Parse(s.URL)
			if err != nil {
				return err
			}
			id := u.Query().Get("id")
			if id != "" {
				mime := map[int]string{2: "application/vnd.google-apps.document", 3: "application/vnd.google-apps.presentation", 8: "application/vnd.google-apps.spreadsheet"}[s.Type]
				if mime == "" {
					mime = "application/vnd.google-apps.document"
				}
				source[0] = []any{id, mime, 1, s.Title}
				source[2] = nil
			}
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return fmt.Errorf("no research sources selected")
	}
	_, err := DecodedRPC(ctx, call, rpc.ImportResearch, []any{nil, []any{1}, task.ID, nb, sources}, nb)
	return err
}
