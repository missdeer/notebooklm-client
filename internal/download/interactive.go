package download

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// FormatInteractive reads JSON data attributes and application/json script tags.
// It never executes the downloaded HTML or its JavaScript.
func FormatInteractive(source, title, kind, format string) ([]byte, error) {
	if format == "html" {
		return []byte(source), nil
	}
	if format != "json" && format != "markdown" && format != "md" {
		return nil, fmt.Errorf("format must be json, markdown or html")
	}
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	var data map[string]any
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken {
			continue
		}
		token := tokenizer.Token()
		script := false
		for _, a := range token.Attr {
			switch a.Key {
			case "data-app-data", "data-state", "data-config", "data-initial-state":
				var candidate map[string]any
				if json.Unmarshal([]byte(a.Val), &candidate) == nil {
					data = candidate
				}
			case "id":
				if a.Val == "application-data" {
					script = true
				}
			case "type":
				if a.Val == "application/json" {
					script = true
				}
			}
		}
		if token.Data == "script" && script && tokenizer.Next() == html.TextToken {
			var candidate map[string]any
			if json.Unmarshal(tokenizer.Text(), &candidate) == nil {
				data = candidate
			}
		}
		if data != nil {
			break
		}
	}
	if data == nil {
		return nil, fmt.Errorf("interactive HTML has no recognized JSON data")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n\n", title)
	if kind == "quiz" {
		questions, _ := data["quiz"].([]any)
		if questions == nil {
			return nil, fmt.Errorf("quiz data missing")
		}
		if format == "json" {
			return json.MarshalIndent(map[string]any{"title": title, "questions": questions}, "", "  ")
		}
		for i, v := range questions {
			q, _ := v.(map[string]any)
			fmt.Fprintf(&out, "## Question %d\n\n%s\n\n", i+1, textField(q, "question"))
			options, _ := q["answerOptions"].([]any)
			for _, v := range options {
				o, _ := v.(map[string]any)
				mark := " "
				if o["isCorrect"] == true {
					mark = "x"
				}
				fmt.Fprintf(&out, "- [%s] %s\n", mark, textField(o, "text"))
			}
			if h := textField(q, "hint"); h != "" {
				fmt.Fprintf(&out, "\n**Hint:** %s\n", h)
			}
			out.WriteString("\n")
		}
	} else {
		cards, _ := data["flashcards"].([]any)
		if cards == nil {
			return nil, fmt.Errorf("flashcard data missing")
		}
		normalized := []any{}
		for i, v := range cards {
			card, _ := v.(map[string]any)
			front, back := textField(card, "f"), textField(card, "b")
			normalized = append(normalized, map[string]string{"front": front, "back": back})
			fmt.Fprintf(&out, "## Card %d\n\n**Front:** %s\n\n**Back:** %s\n\n", i+1, front, back)
		}
		if format == "json" {
			return json.MarshalIndent(map[string]any{"title": title, "cards": normalized}, "", "  ")
		}
	}
	return []byte(out.String()), nil
}
func textField(m map[string]any, k string) string { s, _ := m[k].(string); return s }
