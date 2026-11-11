package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

type pipelineStep struct {
	Action string         `yaml:"action" json:"action"`
	Params map[string]any `yaml:"params" json:"params"`
}
type pipelineDefinition struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description" json:"description"`
	Steps       []pipelineStep `yaml:"steps" json:"steps"`
}

func validatePipeline(p pipelineDefinition) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("pipeline must have steps")
	}
	for _, s := range p.Steps {
		switch s.Action {
		case "source_add", "notebook_query", "studio_create", "notebook_create", "notebook_delete":
		default:
			return fmt.Errorf("unsupported pipeline action %s", s.Action)
		}
	}
	return nil
}
func loadPipelines() (map[string]pipelineDefinition, error) {
	all := map[string]pipelineDefinition{
		"ingest-and-podcast":  {Name: "ingest-and-podcast", Description: "Add URL, summarize and generate audio", Steps: []pipelineStep{{"source_add", map[string]any{"url": "$INPUT_URL"}}, {"notebook_query", map[string]any{"query": "Summarize the content that was just added"}}, {"studio_create", map[string]any{"artifact_type": "audio"}}}},
		"research-and-report": {Name: "research-and-report", Steps: []pipelineStep{{"source_add", map[string]any{"url": "$INPUT_URL"}}, {"studio_create", map[string]any{"artifact_type": "report", "report_format": "Briefing Doc"}}}},
		"multi-format":        {Name: "multi-format", Steps: []pipelineStep{{"studio_create", map[string]any{"artifact_type": "audio"}}, {"studio_create", map[string]any{"artifact_type": "report"}}, {"studio_create", map[string]any{"artifact_type": "flashcards"}}}},
	}
	entries, err := os.ReadDir(filepath.Join(rpc.HomeDir(), "pipelines"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(rpc.HomeDir(), "pipelines", e.Name()))
		if err != nil {
			return nil, err
		}
		var p pipelineDefinition
		if err := yaml.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		if err := validatePipeline(p); err != nil {
			return nil, err
		}
		all[strings.TrimSuffix(e.Name(), ".yaml")] = p
	}
	return all, nil
}
func internalInput(parent *cobra.Command, path string, values map[string]any) commandInput {
	cmd := &cobra.Command{Use: "internal"}
	cmd.SetOut(parent.OutOrStdout())
	cmd.SetErr(parent.ErrOrStderr())
	schema := commandSchema{Canonical: path}
	for _, s := range commandDefinitions {
		if s.Path == path {
			schema = s
			break
		}
	}
	for _, f := range schema.Flags {
		addSchemaFlag(cmd.Flags(), f)
	}
	args := []string{}
	for _, a := range schema.Args {
		v := values[a.Name]
		if v == nil {
			args = append(args, "")
		} else {
			args = append(args, fmt.Sprint(v))
		}
	}
	for name, v := range values {
		key := strings.ReplaceAll(name, "_", "-")
		if cmd.Flags().Lookup(key) == nil {
			cmd.Flags().String(key, "", "")
		}
		_ = cmd.Flags().Set(key, fmt.Sprint(v))
	}
	return commandInput{cmd, schema, args}
}
func selectedNotebooks(ctx context.Context, c *client.NotebookClient, in commandInput) ([]string, error) {
	ids := csvValues(in.val("notebooks"))
	tags := csvValues(in.val("tags"))
	all := flagBool(in.cmd, "all")
	if len(ids) == 0 && len(tags) == 0 && !all {
		return nil, fmt.Errorf("select --notebooks, --tags or --all")
	}
	state, err := readCLIState()
	if err != nil {
		return nil, err
	}
	notebooks, err := c.ListNotebooks(ctx)
	if err != nil {
		return nil, err
	}
	out := []string{}
	if len(ids) > 0 {
		for _, v := range ids {
			resolved, err := resolveResource(v)
			if err != nil {
				return nil, err
			}
			matches := []string{}
			for _, n := range notebooks {
				if n.ID == resolved || n.Title == v {
					matches = append(matches, n.ID)
				}
			}
			if len(matches) != 1 {
				return nil, fmt.Errorf("notebook %q is missing or ambiguous", v)
			}
			if !containsString(out, matches[0]) {
				out = append(out, matches[0])
			}
		}
		return out, nil
	}
	for _, n := range notebooks {
		match := all
		if len(tags) > 0 {
			match = true
			for _, tag := range tags {
				match = match && containsString(state.Tags[n.ID], tag)
			}
		}
		if match {
			out = append(out, n.ID)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("selection matched no notebooks")
	}
	return out, nil
}
func runAutomation(ctx context.Context, c *client.NotebookClient, in commandInput) (any, error) {
	path := in.schema.Canonical
	if path == "pipeline run" {
		all, err := loadPipelines()
		if err != nil {
			return nil, err
		}
		p, ok := all[in.val("pipeline_name")]
		if !ok {
			return nil, fmt.Errorf("pipeline not found")
		}
		nb, err := resolveResource(in.val("notebook"))
		if err != nil {
			return nil, err
		}
		out := []any{}
		for _, step := range p.Steps {
			params := map[string]any{"notebook_id": nb}
			for k, v := range step.Params {
				if v == "$INPUT_URL" {
					v = in.val("input_url")
				}
				params[k] = v
			}
			route := ""
			switch step.Action {
			case "source_add":
				route = "source add"
			case "notebook_query":
				route = "notebook query"
				params["question"] = params["query"]
			case "studio_create":
				kind, _ := params["artifact_type"].(string)
				if kind == "" {
					kind = "audio"
				}
				route = strings.ReplaceAll(kind, "_", "-") + " create"
				if kind == "slide_deck" {
					route = "slides create"
				}
				if kind == "mind_map" {
					route = "mindmap create"
				}
				if v, ok := params["audio_format"]; ok {
					params["format"] = v
				}
				if v, ok := params["audio_length"]; ok {
					params["length"] = v
				}
				if v, ok := params["report_format"]; ok {
					params["format"] = v
				}
				if v, ok := params["focus_prompt"]; ok {
					params["focus"] = v
				}
				if v, ok := params["custom_prompt"]; ok {
					params["prompt"] = v
				}
			case "notebook_create":
				route = "notebook create"
			case "notebook_delete":
				route = "notebook delete"
			}
			child := internalInput(in.cmd, route, params)
			value, err := runRemoteCommand(ctx, c, child)
			if err != nil {
				_ = emitResult(in.cmd, out)
				return nil, fmt.Errorf("pipeline step %s: %w", step.Action, err)
			}
			if step.Action == "notebook_create" {
				if m, ok := value.(map[string]any); ok {
					nb, _ = m["notebook_id"].(string)
				}
			}
			out = append(out, map[string]any{"action": step.Action, "result": value})
		}
		return out, nil
	}
	if path == "batch create" {
		out := []any{}
		for _, title := range csvValues(in.val("titles")) {
			v, err := runRemoteCommand(ctx, c, internalInput(in.cmd, "notebook create", map[string]any{"title": title}))
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	notebooks, err := selectedNotebooks(ctx, c, in)
	if err != nil {
		return nil, err
	}
	out := []any{}
	failed := 0
	for _, id := range notebooks {
		params := map[string]any{"notebook_id": id}
		route := ""
		switch path {
		case "batch query", "cross query":
			route = "notebook query"
			params["question"] = in.val("query")
		case "batch add-source":
			route = "source add"
			params["url"] = in.val("url")
		case "batch delete":
			route = "notebook delete"
		case "batch studio":
			kind := in.val("artifact_type")
			if kind == "" {
				kind = "audio"
			}
			route = kind + " create"
		}
		value, err := runRemoteCommand(ctx, c, internalInput(in.cmd, route, params))
		item := map[string]any{"notebook_id": id, "success": err == nil, "result": value}
		if err != nil {
			failed++
			item["error"] = err.Error()
		}
		out = append(out, item)
	}
	result := map[string]any{"results": out, "total": len(notebooks), "succeeded": len(notebooks) - failed, "failed": failed}
	if failed > 0 {
		_ = emitResult(in.cmd, result)
		return nil, fmt.Errorf("%d notebook operations failed", failed)
	}
	return result, nil
}
