package cli

import (
	"reflect"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/spf13/cobra"
)

func TestSlideDeckCLIInput(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		wantIDs    []string
		wantSource types.SourceType
		wantError  bool
	}{
		{name: "all sources", args: []string{"--notebook-id", "existing"}},
		{name: "selected sources", args: []string{"--notebook-id", "existing", "--source-ids", "one, two"}, wantIDs: []string{"one", "two"}},
		{name: "new source", args: []string{"--text", "Content"}, wantSource: types.SourceText},
		{name: "mixed inputs", args: []string{"--notebook-id", "existing", "--url", "https://example.com"}, wantError: true},
		{name: "missing notebook", args: []string{"--source-ids", "one"}, wantError: true},
		{name: "empty notebook ID", args: []string{"--notebook-id", " "}, wantError: true},
		{name: "empty source selection", args: []string{"--notebook-id", "existing", "--source-ids", ""}, wantError: true},
		{name: "empty source ID", args: []string{"--notebook-id", "existing", "--source-ids", "one, "}, wantError: true},
		{name: "no input", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			addSourceFlags(cmd)
			for _, flag := range []string{"notebook-id", "source-ids", "output", "instructions", "language", "format", "length"} {
				cmd.Flags().String(flag, "", "")
			}
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			opts, err := slideDeckOptions(cmd)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && (!reflect.DeepEqual(opts.SourceIDs, tc.wantIDs) || opts.Source.Type != tc.wantSource) {
				t.Fatalf("unexpected options: %+v", opts)
			}
		})
	}
}
