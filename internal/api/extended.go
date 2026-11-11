package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

// At provides bounded access to the undocumented positional protocol.
func At(v any, path ...int) any {
	for _, i := range path {
		a, ok := v.([]any)
		if !ok || i < 0 || i >= len(a) {
			return nil
		}
		v = a[i]
	}
	return v
}
func StringAt(v any, path ...int) string { s, _ := At(v, path...).(string); return s }
func ArrayAt(v any, path ...int) []any   { a, _ := At(v, path...).([]any); return a }
func ConsumerHeader() []any {
	return []any{2, nil, []any{1}, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1, 3}}}
}
func DecodedRPC(ctx context.Context, call RpcCaller, id string, args []any, notebook string) ([]any, error) {
	path := ""
	if notebook != "" {
		path = "/notebook/" + notebook
	}
	raw, err := call(ctx, id, args, path)
	if err != nil {
		return nil, err
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return nil, fmt.Errorf("RPC %s returned no decodable response", id)
	}
	return envelopes[0], nil
}

type ChatTurn struct {
	Turn   int    `json:"turn"`
	Query  string `json:"query"`
	Answer string `json:"answer"`
}

func ConversationTurns(ctx context.Context, call RpcCaller, notebook, conversation string, limit int) ([]ChatTurn, error) {
	data, err := DecodedRPC(ctx, call, rpc.GetConversationTurns, []any{ConsumerHeader(), nil, nil, conversation, limit}, notebook)
	if err != nil {
		return nil, err
	}
	out := []ChatTurn{}
	pending := ""
	hasAnswer := false
	for _, entry := range ArrayAt(data, 0) {
		switch At(entry, 2) {
		case float64(2):
			pending = StringAt(entry, 4, 0, 0)
			hasAnswer = true
		case float64(1):
			if hasAnswer {
				out = append(out, ChatTurn{Query: StringAt(entry, 3), Answer: pending})
				hasAnswer = false
			}
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	for i := range out {
		out[i].Turn = i + 1
	}
	return out, nil
}

type Label struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Emoji     string   `json:"emoji"`
	SourceIDs []string `json:"source_ids"`
}

func ManageLabels(ctx context.Context, call RpcCaller, notebook, action string, values []string, unlabeled bool) (any, error) {
	id := "agX4Bc"
	args := []any{[]any{2}, notebook, nil, nil, []any{}}
	switch action {
	case "auto", "list":
	case "reorganize":
		mode := 1
		if unlabeled {
			mode = 0
		}
		args[4] = []any{mode}
	case "create":
		args = []any{[]any{2}, notebook, nil, nil, nil, []any{[]any{values[0], values[1]}}}
	case "rename":
		id = "le8sX"
		args = []any{[]any{2}, notebook, values[0], []any{[]any{[]any{values[1]}}}}
	case "emoji":
		id = "le8sX"
		args = []any{[]any{2}, notebook, values[0], []any{[]any{[]any{nil, values[1]}}}}
	case "move":
		id = "le8sX"
		args = []any{[]any{2}, notebook, values[0], []any{[]any{nil, []any{[]any{values[1]}}}}}
	case "delete":
		id = "GyzE7e"
		args = []any{[]any{2}, notebook, values}
	default:
		return nil, fmt.Errorf("unknown label action %s", action)
	}
	data, err := DecodedRPC(ctx, call, id, args, notebook)
	if err != nil {
		return nil, err
	}
	if id != "agX4Bc" {
		return map[string]any{"success": true}, nil
	}
	labels := []Label{}
	for _, v := range ArrayAt(data, 1) {
		sids := []string{}
		for _, s := range ArrayAt(v, 1) {
			if id := StringAt(s, 0); id != "" {
				sids = append(sids, id)
			}
		}
		labels = append(labels, Label{StringAt(v, 2), StringAt(v, 0), StringAt(v, 3), sids})
	}
	return labels, nil
}

func AddDriveSource(ctx context.Context, call RpcCaller, notebook, document, title, mime string) (string, string, error) {
	data := []any{[]any{document, mime, 1, title}, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1}
	raw, err := call(ctx, rpc.AddSource, []any{[]any{data}, notebook, []any{2}, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}}, "/notebook/"+notebook)
	if err != nil {
		return "", "", err
	}
	id, t := parser.ParseAddSource(raw)
	if id == "" {
		return "", "", fmt.Errorf("Drive source registration returned no ID")
	}
	return id, t, nil
}
func SourceFreshness(ctx context.Context, call RpcCaller, id string) (*bool, error) {
	data, err := DecodedRPC(ctx, call, rpc.CheckSourceFreshness, []any{nil, []any{id}, []any{2}}, "")
	if err != nil {
		return nil, err
	}
	v := At(data, 0, 1)
	if b, ok := v.(bool); ok {
		return &b, nil
	}
	return nil, nil
}

type MindMap struct {
	ID      string          `json:"mind_map_id"`
	Title   string          `json:"title"`
	Content json.RawMessage `json:"mind_map_json"`
}

func CreateMindMap(ctx context.Context, call RpcCaller, notebook, title string, sids []string) (MindMap, error) {
	nested := []any{}
	simple := []any{}
	for _, id := range sids {
		nested = append(nested, []any{[]any{id}})
		simple = append(simple, []any{id})
	}
	data, err := DecodedRPC(ctx, call, rpc.GenerateMindMap, []any{nested, nil, nil, nil, nil, []any{"interactive_mindmap", []any{[]any{"[CONTEXT]", ""}}, ""}, nil, []any{2, nil, []any{1}}}, notebook)
	if err != nil {
		return MindMap{}, err
	}
	content := StringAt(data, 0, 0)
	if !json.Valid([]byte(content)) {
		return MindMap{}, fmt.Errorf("mind map generation returned invalid JSON")
	}
	if title == "" {
		title = "Mind Map"
	}
	saved, err := DecodedRPC(ctx, call, rpc.CreateNote, []any{notebook, content, []any{2, nil, nil, 5, simple}, nil, title}, notebook)
	if err != nil {
		return MindMap{}, err
	}
	id := StringAt(saved, 0, 0)
	if id == "" {
		return MindMap{}, fmt.Errorf("mind map save returned no ID")
	}
	return MindMap{id, title, json.RawMessage(content)}, nil
}
func ListMindMaps(ctx context.Context, call RpcCaller, notebook string) ([]MindMap, error) {
	data, err := DecodedRPC(ctx, call, rpc.GetNotes, []any{notebook}, notebook)
	if err != nil {
		return nil, err
	}
	out := []MindMap{}
	for _, v := range ArrayAt(data, 0) {
		d := At(v, 1)
		content := StringAt(d, 1)
		var m map[string]any
		if json.Unmarshal([]byte(content), &m) != nil {
			continue
		}
		_, children := m["children"]
		_, nodes := m["nodes"]
		if !children && !nodes {
			continue
		}
		out = append(out, MindMap{StringAt(v, 0), StringAt(d, 4), json.RawMessage(content)})
	}
	return out, nil
}
func Usage(ctx context.Context, call RpcCaller) (any, error) {
	data, err := DecodedRPC(ctx, call, "EylDcb", []any{ConsumerHeader()}, "")
	if err != nil {
		return nil, err
	}
	windows := []any{}
	for _, v := range ArrayAt(data, 1) {
		if len(ArrayAt(v)) < 8 {
			continue
		}
		kind := "unknown"
		switch At(v, 4) {
		case float64(1):
			kind = "rolling"
		case float64(2):
			kind = "weekly"
		}
		windows = append(windows, map[string]any{"window": kind, "percent_used": At(v, 6), "percent_remaining": At(v, 7), "resets_at": At(v, 5, 0)})
	}
	tierData, err := DecodedRPC(ctx, call, rpc.GetUIConfig, []any{[]any{[]any{[]any{nil, "1", 627}, []any{nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{nil, nil, 2}}, 1}}}, "")
	if err != nil {
		return nil, err
	}
	tier := ""
	var find func(any)
	find = func(v any) {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "NOTEBOOKLM_TIER_") {
			tier = s
		}
		if a, ok := v.([]any); ok {
			for _, x := range a {
				find(x)
			}
		}
	}
	find(tierData)
	return map[string]any{"windows": windows, "tier": tier}, nil
}
