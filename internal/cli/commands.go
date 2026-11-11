package cli

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//go:embed command_catalog.json
var commandCatalog []byte

type commandArgument struct {
	Name, Help     string
	Required, Many bool
}
type commandFlag struct {
	Name, Help, Short, Type, Negative string
	Aliases                           []string
	Required                          bool
	Default                           any
}
type commandSchema struct {
	Path, Canonical, Help string
	Args                  []commandArgument
	Flags                 []commandFlag
}

var commandDefinitions []commandSchema
var initializeCommandsOnce sync.Once

// initializeCommands registers the command catalog after the package's workflow
// commands and flags have been initialized, independent of file naming order.
func initializeCommands() {
	initializeCommandsOnce.Do(registerCommands)
}

func registerCommands() {
	if err := json.Unmarshal(commandCatalog, &commandDefinitions); err != nil {
		panic(err)
	}
	for _, s := range commandDefinitions {
		registerCommand(s)
	}
	rootCmd.PersistentFlags().Bool("ai", false, "Print the complete command schema for AI agents")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable diagnostic logging")
	rootCmd.Version = "dev"
	oldRun := rootCmd.RunE
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if b, _ := cmd.Flags().GetBool("ai"); b {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(commandDefinitions)
		}
		if oldRun != nil {
			return oldRun(cmd, args)
		}
		return cmd.Help()
	}
}

func directChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
func registerCommand(s commandSchema) {
	parent := rootCmd
	parts := strings.Fields(s.Path)
	for _, name := range parts[:len(parts)-1] {
		c := directChild(parent, name)
		if c == nil {
			c = &cobra.Command{Use: name, Short: "Manage " + name}
			parent.AddCommand(c)
		}
		parent = c
	}
	name := parts[len(parts)-1]
	cmd := directChild(parent, name)
	var legacyRun func(*cobra.Command, []string) error
	var legacyArgs cobra.PositionalArgs
	if cmd != nil {
		legacyRun, legacyArgs = cmd.RunE, cmd.Args
	}
	// Existing top-level workflows remain runnable while gaining subcommands.
	if cmd == nil {
		cmd = &cobra.Command{Use: name}
		parent.AddCommand(cmd)
	}
	cmd.Use = name
	for _, a := range s.Args {
		label := "<" + a.Name + ">"
		if !a.Required {
			label = "[" + a.Name + "]"
		}
		if a.Many {
			label += "..."
		}
		cmd.Use += " " + label
	}
	cmd.Short = s.Help
	cmd.Annotations = map[string]string{"cli-canonical": s.Canonical}
	// Source/research/studio legacy entry points accept their old flags as well.
	for _, f := range s.Flags {
		if existing := cmd.Flags().Lookup(f.Name); existing != nil {
			fresh := pflag.NewFlagSet("command", pflag.ContinueOnError)
			addSchemaFlag(fresh, f)
			replacement := fresh.Lookup(f.Name)
			existing.Value, existing.DefValue, existing.NoOptDefVal, existing.Usage = replacement.Value, replacement.DefValue, replacement.NoOptDefVal, replacement.Usage
			existing.Changed = false
			for _, alias := range f.Aliases {
				if cmd.Flags().Lookup(alias) == nil {
					copyFlag := *existing
					copyFlag.Name = alias
					copyFlag.Shorthand = ""
					cmd.Flags().AddFlag(&copyFlag)
				}
			}
			continue
		}
		addSchemaFlag(cmd.Flags(), f)
	}
	if cmd.Flags().Lookup("transport") == nil {
		addCommandTransportFlags(cmd)
	}
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if legacyRun != nil && useLegacySyntax(s.Path, cmd, args) {
			if legacyArgs != nil {
				return legacyArgs(cmd, args)
			}
			return nil
		}
		min, max := 0, len(s.Args)
		many := false
		for _, a := range s.Args {
			if a.Required {
				min++
			}
			many = many || a.Many
		}
		if many {
			max = -1
		}
		// Legacy research start <notebook> --query remains accepted.
		if len(args) < min || (max >= 0 && len(args) > max) {
			return fmt.Errorf("expected %d..%d positional arguments; see %s --help", min, max, cmd.CommandPath())
		}
		for _, f := range s.Flags {
			if f.Required && !cmd.Flags().Changed(f.Name) {
				return fmt.Errorf("--%s is required", f.Name)
			}
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if legacyRun != nil && useLegacySyntax(s.Path, cmd, args) {
			return legacyRun(cmd, args)
		}
		return runCommand(cmd, s, args)
	}
}
func useLegacySyntax(path string, cmd *cobra.Command, args []string) bool {
	switch path {
	case "research start":
		return flagString(cmd, "query") != ""
	case "source rename":
		return len(args) == 3
	case "share public":
		return cmd.Flags().Changed("enable") || cmd.Flags().Changed("disable")
	case "share invite":
		return len(args) == 1 && flagString(cmd, "email") != ""
	case "note delete":
		return len(args) > 2
	case "source delete":
		return len(args) > 1
	case "studio delete":
		return len(args) == 1 || len(args) > 2
	}
	return false
}
func addSchemaFlag(fs *pflag.FlagSet, f commandFlag) {
	short := f.Short
	if len(short) != 1 || fs.ShorthandLookup(short) != nil {
		short = ""
	}
	names := append([]string{f.Name}, f.Aliases...)
	switch f.Type {
	case "bool":
		p := new(bool)
		*p, _ = f.Default.(bool)
		for i, n := range names {
			sh := ""
			if i == 0 {
				sh = short
			}
			fs.BoolVarP(p, n, sh, *p, f.Help)
		}
		if f.Negative != "" {
			fs.Var(&negativeBool{p}, f.Negative, f.Help)
			fs.Lookup(f.Negative).NoOptDefVal = "true"
		}
	case "int":
		p := new(int)
		if n, ok := f.Default.(float64); ok {
			*p = int(n)
		}
		for i, n := range names {
			sh := ""
			if i == 0 {
				sh = short
			}
			fs.IntVarP(p, n, sh, *p, f.Help)
		}
	case "float":
		p := new(float64)
		*p, _ = f.Default.(float64)
		for i, n := range names {
			sh := ""
			if i == 0 {
				sh = short
			}
			fs.Float64VarP(p, n, sh, *p, f.Help)
		}
	case "strings":
		p := new([]string)
		for i, n := range names {
			sh := ""
			if i == 0 {
				sh = short
			}
			fs.StringArrayVarP(p, n, sh, nil, f.Help)
		}
	default:
		p := new(string)
		*p, _ = f.Default.(string)
		for i, n := range names {
			sh := ""
			if i == 0 {
				sh = short
			}
			fs.StringVarP(p, n, sh, *p, f.Help)
		}
	}
}

type negativeBool struct{ value *bool }

func (b *negativeBool) String() string { return strconv.FormatBool(!*b.value) }
func (b *negativeBool) Type() string   { return "bool" }
func (b *negativeBool) Set(s string) error {
	v, e := strconv.ParseBool(s)
	if e == nil {
		*b.value = !v
	}
	return e
}
func addCommandTransportFlags(cmd *cobra.Command) {
	for _, f := range []commandFlag{{Name: "transport", Default: "auto"}, {Name: "session-path"}, {Name: "proxy"}, {Name: "chrome-path"}, {Name: "headless", Type: "bool"}} {
		if cmd.Flags().Lookup(f.Name) == nil {
			addSchemaFlag(cmd.Flags(), f)
		}
	}
	if cmd.Flags().Lookup("profile") == nil {
		addSchemaFlag(cmd.Flags(), commandFlag{Name: "profile", Help: "Named account profile"})
	}
}
func flagString(cmd *cobra.Command, name string) string {
	f := cmd.Flags().Lookup(name)
	if f == nil {
		return ""
	}
	return f.Value.String()
}
func flagBool(cmd *cobra.Command, name string) bool {
	v, _ := strconv.ParseBool(flagString(cmd, name))
	return v
}
func flagInt(cmd *cobra.Command, name string, fallback int) int {
	v, e := strconv.Atoi(flagString(cmd, name))
	if e != nil {
		return fallback
	}
	return v
}
func flagStrings(cmd *cobra.Command, name string) []string {
	if f := cmd.Flags().Lookup(name); f != nil {
		if v, ok := f.Value.(pflag.SliceValue); ok {
			return v.GetSlice()
		}
		if s := f.Value.String(); s != "" {
			return []string{s}
		}
	}
	return nil
}
func csvValues(s string) []string {
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

type commandInput struct {
	cmd    *cobra.Command
	schema commandSchema
	args   []string
}

func (in commandInput) arg(names ...string) string {
	for _, name := range names {
		for i, a := range in.schema.Args {
			if a.Name == name && i < len(in.args) {
				return in.args[i]
			}
		}
	}
	return ""
}
func (in commandInput) val(names ...string) string {
	for _, name := range names {
		if s := in.arg(name); s != "" {
			return s
		}
		if s := flagString(in.cmd, strings.ReplaceAll(name, "_", "-")); s != "" {
			return s
		}
		aliases := map[string][]string{"source_id": {"source"}, "source_ids": {"source"}, "artifact_id": {"artifact"}, "url": {"url_arg"}, "text": {"text_arg"}, "drive": {"document_id"}, "notebook_id": {"notebook"}}
		for _, alias := range aliases[name] {
			if s := in.arg(alias); s != "" {
				return s
			}
		}
	}
	return ""
}
