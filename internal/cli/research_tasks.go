package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func runResearch(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	call := api.RpcCaller(c.CallBatchExecute)
	poll := func(id string, wait int, interval int) (api.ResearchTask, error) {
		if interval < 1 {
			interval = 1
		}
		deadline := time.Now().Add(time.Duration(wait) * time.Second)
		for {
			tasks, err := api.ResearchTasks(ctx, call, nb)
			if err != nil {
				return api.ResearchTask{}, err
			}
			task, err := api.SelectResearchTask(tasks, id)
			if err != nil {
				return task, err
			}
			if wait == 0 || task.Status == "completed" {
				return task, nil
			}
			if !time.Now().Before(deadline) {
				return task, fmt.Errorf("research poll timed out")
			}
			select {
			case <-ctx.Done():
				return task, ctx.Err()
			case <-time.After(time.Duration(interval) * time.Second):
			}
		}
	}
	switch in.schema.Canonical {
	case "research start":
		source := in.val("source")
		if source != "web" && source != "drive" {
			return nil, fmt.Errorf("source must be web or drive")
		}
		mode := in.val("mode")
		if mode != "fast" && mode != "deep" {
			return nil, fmt.Errorf("mode must be fast or deep")
		}
		if source == "drive" && mode == "deep" {
			return nil, fmt.Errorf("deep research supports web only")
		}
		if nb == "" {
			if in.val("title") == "" {
				return nil, fmt.Errorf("provide --notebook-id or --title")
			}
			id, err := c.CreateNotebook(ctx)
			if err != nil {
				return nil, err
			}
			nb = id
			if err := c.RenameNotebook(ctx, nb, in.val("title")); err != nil {
				return nil, err
			}
		}
		if !flagBool(in.cmd, "force") {
			tasks, err := api.ResearchTasks(ctx, call, nb)
			if err != nil {
				return nil, err
			}
			for _, t := range tasks {
				if t.Status == "in_progress" {
					return nil, fmt.Errorf("research %s is still running; use --force to start another", t.ID)
				}
			}
		}
		sourceCode := 1
		if source == "drive" {
			sourceCode = 2
		}
		id := rpc.CreateWebSearch
		args := []any{[]any{in.val("query"), sourceCode}, nil, 1, nb}
		if mode == "deep" {
			id = rpc.CreateDeepResearch
			args = []any{nil, []any{1}, []any{in.val("query"), sourceCode}, 5, nb}
		}
		data, err := api.DecodedRPC(ctx, call, id, args, nb)
		if err != nil {
			return nil, err
		}
		taskID := api.StringAt(data, 0)
		if taskID == "" {
			return nil, fmt.Errorf("research start returned no task ID")
		}
		if flagBool(in.cmd, "auto-import") {
			task, err := poll(taskID, 600, 5)
			if err != nil {
				return nil, err
			}
			if err := api.ImportTaskSources(ctx, call, nb, task, nil, false); err != nil {
				return nil, err
			}
		}
		return map[string]string{"notebook_id": nb, "task_id": taskID}, nil
	case "research status":
		return poll(in.val("task_id"), flagInt(in.cmd, "max-wait", 300), flagInt(in.cmd, "poll-interval", 30))
	case "research import":
		task, err := poll(in.val("task_id", "research_id"), flagInt(in.cmd, "timeout", 300), 5)
		if err != nil {
			return nil, err
		}
		indices := []int{}
		for _, s := range csvValues(in.val("indices")) {
			i, e := strconv.Atoi(s)
			if e != nil || i < 0 {
				return nil, fmt.Errorf("invalid source index %s", s)
			}
			found := false
			for _, source := range task.Sources {
				found = found || source.Index == i
			}
			if !found {
				return nil, fmt.Errorf("source index %d not found", i)
			}
			indices = append(indices, i)
		}
		return success(api.ImportTaskSources(ctx, call, nb, task, indices, flagBool(in.cmd, "cited-only")))
	}
	return nil, fmt.Errorf("unknown research command")
}
