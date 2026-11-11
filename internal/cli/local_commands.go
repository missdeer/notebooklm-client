package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/session"
	"go.yaml.in/yaml/v3"
)

type cliState struct {
	Aliases map[string]string   `json:"aliases"`
	Config  map[string]any      `json:"config"`
	Tags    map[string][]string `json:"tags"`
}

func readCLIState() (cliState, error) {
	s := cliState{map[string]string{}, map[string]any{}, map[string][]string{}}
	b, err := os.ReadFile(filepath.Join(rpc.HomeDir(), "cli.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("invalid CLI configuration: %w", err)
	}
	if s.Aliases == nil {
		s.Aliases = map[string]string{}
	}
	if s.Config == nil {
		s.Config = map[string]any{}
	}
	if s.Tags == nil {
		s.Tags = map[string][]string{}
	}
	return s, nil
}

func saveCLIState(s cliState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicLocal(filepath.Join(rpc.HomeDir(), "cli.json"), b)
}

func writeAtomicLocal(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".cli-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

var safeIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func runLocalCommand(in commandInput) (bool, error) {
	path := in.schema.Canonical
	local := strings.HasPrefix(path, "alias ") || strings.HasPrefix(path, "config ") || strings.HasPrefix(path, "tag ") || strings.HasPrefix(path, "login") || strings.HasPrefix(path, "auth ") || strings.HasPrefix(path, "doctor") || strings.HasPrefix(path, "pipeline ") && path != "pipeline run"
	if !local {
		return false, nil
	}
	state, err := readCLIState()
	if err != nil {
		return true, err
	}
	var result any
	mutate := false
	switch path {
	case "alias set":
		name := in.val("name")
		if name == "" {
			return true, fmt.Errorf("alias name is empty")
		}
		state.Aliases[name] = in.val("value")
		mutate = true
		result = state.Aliases[name]
	case "alias get":
		v, ok := state.Aliases[in.val("name")]
		if !ok {
			return true, fmt.Errorf("alias not found")
		}
		result = v
	case "alias list":
		result = state.Aliases
	case "alias delete":
		name := in.val("name")
		if _, ok := state.Aliases[name]; !ok {
			return true, fmt.Errorf("alias not found")
		}
		delete(state.Aliases, name)
		mutate = true
	case "config show":
		result = state.Config
	case "config get":
		v, ok := state.Config[in.val("key")]
		if !ok {
			return true, fmt.Errorf("config key not found")
		}
		result = v
	case "config set":
		var value any
		text := in.val("value")
		if json.Unmarshal([]byte(text), &value) != nil {
			value = text
		}
		state.Config[in.val("key")] = value
		mutate = true
	case "tag list":
		result = state.Tags
	case "tag add", "tag remove":
		id, err := resolveResource(in.val("notebook_id"))
		if err != nil {
			return true, err
		}
		tags := csvValues(in.val("tags"))
		current := state.Tags[id]
		for _, tag := range tags {
			if path == "tag add" {
				if !containsString(current, tag) {
					current = append(current, tag)
				}
			} else {
				filtered := []string{}
				for _, v := range current {
					if v != tag {
						filtered = append(filtered, v)
					}
				}
				current = filtered
			}
		}
		state.Tags[id] = current
		result = current
		mutate = true
	case "tag select":
		wanted := strings.Fields(strings.ReplaceAll(strings.ToLower(in.val("query")), ",", " "))
		out := []string{}
		for id, tags := range state.Tags {
			match := false
			for _, t := range wanted {
				match = match || containsString(tags, t)
			}
			if match {
				out = append(out, id)
			}
		}
		sort.Strings(out)
		result = out
	case "pipeline create":
		name := in.val("name")
		if !safeIdentifier.MatchString(name) || strings.Contains(name, "..") {
			return true, fmt.Errorf("invalid pipeline name")
		}
		b, err := os.ReadFile(in.val("file"))
		if err != nil {
			return true, err
		}
		var p pipelineDefinition
		if err := yaml.Unmarshal(b, &p); err != nil {
			return true, err
		}
		p.Name = name
		if in.val("description") != "" {
			p.Description = in.val("description")
		}
		if err := validatePipeline(p); err != nil {
			return true, err
		}
		data, _ := yaml.Marshal(p)
		return true, writeAtomicLocal(filepath.Join(rpc.HomeDir(), "pipelines", name+".yaml"), data)
	case "pipeline list":
		all, err := loadPipelines()
		if err != nil {
			return true, err
		}
		result = all
	case "login profile list":
		names, err := savedProfiles()
		if err != nil {
			return true, err
		}
		result = names
	case "login profile delete":
		name := in.val("profile", "name")
		path, err := profileSessionPath(name)
		if err != nil {
			return true, err
		}
		if name == "default" {
			return true, fmt.Errorf("cannot delete the default profile")
		}
		if err := session.RemoveStoredSession(path); err != nil {
			return true, err
		}
		if state.Config["default_profile"] == name {
			state.Config["default_profile"] = "default"
		}
		mutate = true
	case "login profile rename":
		oldName, newName := in.val("old_name"), in.val("new_name")
		src, err := profileSessionPath(oldName)
		if err != nil {
			return true, err
		}
		dst, err := profileSessionPath(newName)
		if err != nil {
			return true, err
		}
		if oldName == "default" || newName == "default" {
			return true, fmt.Errorf("cannot rename default profile")
		}
		if _, err := os.Stat(filepath.Dir(dst)); err == nil {
			return true, fmt.Errorf("destination profile already exists")
		}
		if err := os.Rename(filepath.Dir(src), filepath.Dir(dst)); err != nil {
			return true, err
		}
		if state.Config["default_profile"] == oldName {
			state.Config["default_profile"] = newName
			mutate = true
		}
	case "login switch":
		name := in.val("profile", "name")
		if name == "" {
			name = in.val("profile_name")
		}
		p, err := profileSessionPath(name)
		if err != nil {
			return true, err
		}
		sess, err := session.Load(p)
		if err != nil {
			return true, err
		}
		if sess == nil {
			return true, fmt.Errorf("profile has no session")
		}
		state.Config["default_profile"] = name
		mutate = true
		result = name
	case "login":
		return true, loginProfile(in, state)
	case "auth refresh":
		return true, refreshProfile(in, state)
	case "auth storage status":
		names, err := savedProfiles()
		if err != nil {
			return true, err
		}
		out := []any{}
		for _, name := range names {
			if chosen := in.val("profile"); chosen != "" && chosen != name {
				continue
			}
			p, _ := profileSessionPath(name)
			mode, err := session.StorageMode(p)
			if err != nil {
				return true, err
			}
			out = append(out, map[string]string{"profile": name, "mode": mode})
		}
		result = out
	case "auth storage set", "auth storage resolve":
		if path == "auth storage resolve" && flagBool(in.cmd, "clear-marker") && in.val("choice") == "" {
			return true, emitResult(in.cmd, "No pending storage operation markers.")
		}
		mode := in.val("mode", "choice")
		if mode == "" {
			mode = "file"
		}
		names := []string{selectedProfile(in, state)}
		if flagBool(in.cmd, "all") {
			var err error
			names, err = savedProfiles()
			if err != nil {
				return true, err
			}
		}
		for _, name := range names {
			p, err := profileSessionPath(name)
			if err != nil {
				return true, err
			}
			if path == "auth storage resolve" && flagBool(in.cmd, "discard-inaccessible") {
				if !flagBool(in.cmd, "yes") {
					return true, fmt.Errorf("--discard-inaccessible requires --yes")
				}
				if err := session.QuarantineInaccessibleSession(p); err != nil {
					return true, err
				}
				continue
			}
			if err := session.SetStorageMode(p, mode); err != nil {
				return true, err
			}
		}
		result = map[string]any{"profiles": names, "mode": mode}
	case "auth storage relocate":
		names, err := savedProfiles()
		if err != nil {
			return true, err
		}
		for _, name := range names {
			p, _ := profileSessionPath(name)
			if _, err := session.Load(p); err != nil {
				return true, fmt.Errorf("profile %s: %w", name, err)
			}
		}
		result = "Storage verified at " + rpc.HomeDir() + "; keyring identities survive directory moves."
	case "doctor":
		names, err := savedProfiles()
		if err != nil {
			return true, err
		}
		result = map[string]any{"home": rpc.HomeDir(), "profiles": names, "version": rootCmd.Version}
	case "doctor auth-replay":
		return true, refreshProfile(in, state)
	default:
		return true, fmt.Errorf("unhandled local command %s", path)
	}
	if mutate {
		if err := saveCLIState(state); err != nil {
			return true, err
		}
	}
	if result == nil {
		result = map[string]bool{"success": true}
	}
	return true, emitResult(in.cmd, result)
}
