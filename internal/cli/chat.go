package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func queryNotebook(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	ids, err := notebookSources(ctx, c, nb, in.val("source_ids"))
	if err != nil {
		return nil, err
	}
	conversation := in.val("conversation_id")
	if flagBool(in.cmd, "new-conversation") {
		conversation = uuid.NewString()
	}
	if conversation != "" {
		c.SelectChatConversation(nb, conversation)
	}
	return c.SendChatWithCitations(ctx, nb, in.val("question", "query"), ids)
}

func chatREPL(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) error {
	ids, err := notebookSources(ctx, c, nb, "")
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(in.cmd.InOrStdin())
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for {
		fmt.Fprint(in.cmd.OutOrStdout(), "> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		q := strings.TrimSpace(scanner.Text())
		switch q {
		case "/exit", "/quit":
			return nil
		case "/help":
			fmt.Fprintln(in.cmd.OutOrStdout(), "/exit /new /help")
			continue
		case "/new":
			c.SelectChatConversation(nb, uuid.NewString())
			continue
		case "":
			continue
		}
		answer, _, err := c.SendChat(ctx, nb, q, ids)
		if err != nil {
			return err
		}
		fmt.Fprintln(in.cmd.OutOrStdout(), answer)
	}
}

func manageChats(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	threads, err := c.ListChatThreads(ctx, nb)
	if err != nil {
		return nil, err
	}
	action := strings.TrimPrefix(in.schema.Canonical, "chats ")
	conversation := in.val("conversation_id")
	if conversation == "" && len(threads) > 0 {
		conversation = threads[0]
	}
	if action == "list" {
		out := []any{}
		for _, id := range threads {
			turns, err := api.ConversationTurns(ctx, c.CallBatchExecute, nb, id, flagInt(in.cmd, "limit", 20))
			if err != nil {
				return nil, err
			}
			preview := ""
			if len(turns) > 0 {
				preview = turns[0].Query
			}
			out = append(out, map[string]any{"conversation_id": id, "turn_count": len(turns), "preview": preview})
		}
		return out, nil
	}
	if action == "delete" {
		for _, id := range threads {
			if _, err := api.DecodedRPC(ctx, c.CallBatchExecute, rpc.DeleteChatThread, []any{api.ConsumerHeader(), id, nil, 1}, nb); err != nil {
				return nil, err
			}
		}
		return success(nil)
	}
	if conversation == "" {
		return nil, fmt.Errorf("notebook has no conversations")
	}
	turns, err := api.ConversationTurns(ctx, c.CallBatchExecute, nb, conversation, 1000)
	if err != nil {
		return nil, err
	}
	if action == "get" {
		return map[string]any{"conversation_id": conversation, "transcript": turns}, nil
	}
	if turn := flagInt(in.cmd, "turn", 0); turn > 0 {
		if turn > len(turns) {
			return nil, fmt.Errorf("turn %d is out of range", turn)
		}
		turns = turns[turn-1 : turn]
	}
	var text strings.Builder
	for _, t := range turns {
		fmt.Fprintf(&text, "## Turn %d\n\n**User:** %s\n\n**NotebookLM:** %s\n\n", t.Turn, t.Query, t.Answer)
	}
	if action == "to-note" {
		id, err := c.CreateNote(ctx, nb, in.val("title"), text.String())
		return map[string]string{"note_id": id}, err
	}
	if format := in.val("format"); format == "json" {
		b, err := json.MarshalIndent(turns, "", "  ")
		if err != nil {
			return nil, err
		}
		text.Reset()
		text.Write(b)
	} else if format != "md" && format != "markdown" && format != "" {
		return nil, fmt.Errorf("format must be md or json")
	}
	if output := in.val("output"); output != "" {
		return output, writeOutput(output, []byte(text.String()))
	}
	return text.String(), nil
}
