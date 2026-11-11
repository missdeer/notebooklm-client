package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func envelope(id string, v any) string {
	inner, _ := json.Marshal(v)
	out, _ := json.Marshal([]any{[]any{"wrb.fr", id, string(inner)}})
	return string(out)
}
func TestConversationTurnsChronological(t *testing.T) {
	call := func(ctx context.Context, id string, args []any, path string) (string, error) {
		if id != "khqZz" || args[3] != "conversation" || path != "/notebook/notebook" {
			t.Fatalf("incorrect request %s %#v %s", id, args, path)
		}
		return envelope(id, []any{[]any{[]any{"a2", nil, 2, nil, []any{[]any{"second answer"}}}, []any{"q2", nil, 1, "second question"}, []any{"a1", nil, 2, nil, []any{[]any{"first answer"}}}, []any{"q1", nil, 1, "first question"}}, nil}), nil
	}
	turns, err := ConversationTurns(context.Background(), call, "notebook", "conversation", 20)
	if err != nil || len(turns) != 2 || turns[0].Query != "first question" || turns[1].Answer != "second answer" || turns[1].Turn != 2 {
		t.Fatalf("%+v %v", turns, err)
	}
}
func TestLabelRenameAndFreshness(t *testing.T) {
	call := func(ctx context.Context, id string, args []any, path string) (string, error) {
		if id == "le8sX" {
			want := []any{[]any{2}, "nb", "label", []any{[]any{[]any{"new"}}}}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("bad rename payload %#v", args)
			}
			return envelope(id, []any{}), nil
		}
		return envelope(id, []any{[]any{"source", false}}), nil
	}
	if _, err := ManageLabels(context.Background(), call, "nb", "rename", []string{"label", "new"}, false); err != nil {
		t.Fatal(err)
	}
	fresh, err := SourceFreshness(context.Background(), call, "source")
	if err != nil || fresh == nil || *fresh {
		t.Fatalf("freshness %v %v", fresh, err)
	}
}
func TestInteractiveReportAndElementKickoffFailure(t *testing.T) {
	raw := make([]any, 36)
	raw[0], raw[2], raw[4] = "el", float64(3), float64(5)
	raw[3] = []any{[]any{[]any{"source"}}}
	calls := []string{}
	call := func(ctx context.Context, id string, args []any, path string) (string, error) {
		calls = append(calls, id)
		switch id {
		case "v9rmvd":
			return envelope(id, []any{raw}), nil
		case "rc3d8d":
			mask := args[1].([]any)[0].([]string)
			if mask[0] != "explainer_video.generation_options.language_code" {
				t.Fatalf("wrong mask %#v", args)
			}
			return envelope(id, []any{}), nil
		case "Rytqqe":
			return "", errors.New("connection lost")
		}
		t.Fatalf("unexpected %s", id)
		return "", nil
	}
	_, err := StartReportElement(context.Background(), call, "nb", "el", "prompt", "en", nil, map[string]int{"video_format": 4})
	if err == nil {
		t.Fatal("uncertain kickoff reported success")
	}
	if len(calls) != 3 {
		t.Fatalf("kickoff retried: %v", calls)
	}
}
func TestReportRenderingAndScope(t *testing.T) {
	raw := make([]any, 35)
	embed := make([]any, 13)
	embed[12] = []any{"element", 1}
	raw[34] = []any{[]any{[]any{[]any{nil, nil, []any{"Heading"}}, embed, []any{1, nil, []any{[]any{[]any{nil, nil, []any{"Text"}}}}}}}}
	elements := ReportElements(raw)
	if len(elements) != 1 || elements[0].ID != "element" {
		t.Fatalf("%+v", elements)
	}
	text := RenderReport(raw)
	if text != "\n## Heading\n\n\n[Embedded element: element]\n\nText\n\n" {
		t.Fatalf("%q", text)
	}
}
func TestResearchTaskSelectionAndDriveImport(t *testing.T) {
	data := []any{[]any{[]any{"task", []any{nil, []any{"query", float64(2)}, float64(1), []any{[]any{[]any{"https://drive.google.com/open?id=document", "Drive doc", "Description", float64(2)}}, "summary"}, float64(2)}}}}
	tasks := ParseResearchTasks(data)
	if len(tasks) != 1 || tasks[0].SourceType != "drive" || tasks[0].Status != "completed" {
		t.Fatalf("%+v", tasks)
	}
	call := func(ctx context.Context, id string, args []any, path string) (string, error) {
		if id != "LBwxtb" || StringAt(args, 4, 0, 0, 0) != "document" || StringAt(args, 4, 0, 0, 1) != "application/vnd.google-apps.document" {
			t.Fatalf("bad Drive import %#v", args)
		}
		return envelope(id, []any{}), nil
	}
	if err := ImportTaskSources(context.Background(), call, "notebook", tasks[0], []int{0}, false); err != nil {
		t.Fatal(err)
	}
}
