package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/cpp"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Setting describes a personal setting, shared by presentation and validation.
// Pack manifests and installed-pack inventory deliberately are not settings.
type Setting struct {
	Key, Label, Category, Description string
	Choices                           []string
	YAMLOnly                          bool
}

func Settings() []Setting {
	return []Setting{
		{Key: "language", Label: "Solution language", Category: "General", Description: "Preferred solution language. Unavailable preferences fall back without changing the saved value.", Choices: problem.Languages(problem.Protocol)},
		{Key: "locale", Label: "Statement locale", Category: "General", Description: "Preferred statement language (for example en or zh-CN). This does not change the pack's translations."},
		{Key: "editor", Label: "Editor", Category: "General", Description: "Editor command, such as code --wait. Empty uses VISUAL, then EDITOR, then vi. Used the next time you open a file."},
		{Key: "theme", Label: "Theme", Category: "Display", Description: "Statement colors. Auto follows terminal theme reports; changes apply immediately.", Choices: []string{"auto", "dark", "light"}},
		{Key: "images", Label: "Statement images", Category: "Display", Description: "How statement images are drawn. Changes apply immediately.", Choices: []string{"auto", "kitty", "blocks", "none"}},
		{Key: "timeout", Label: "Run timeout", Category: "Execution & tools", Description: "Positive duration, such as 10s or 2m. Applies to the next run; separate from reference-based time limits."},
		{Key: "execution.backend", Label: "Execution backend", Category: "Execution & tools", Description: "Code runs locally. Docker is not implemented.", Choices: []string{"local"}},
		{Key: "go.command", Label: "Go compiler", Category: "Execution & tools", Description: "Executable name or workspace-relative path. Used by the next build; hc does not install Go."},
		{Key: "go.debugger", Label: "Go debugger", Category: "Execution & tools", Description: "Delve executable name or workspace-relative path. Used by the next debugging session."},
		{Key: "cpp.command", Label: "C++ compiler", Category: "Execution & tools", Description: "Local g++ on Linux or clang++ on macOS. Executable name or workspace-relative path."},
		{Key: "cpp.debugger", Label: "C++ debugger", Category: "Execution & tools", Description: "Terminal GDB or LLDB executable. Editor debugging is not supported for C++."},
		{Key: "cpp.standard", Label: "C++ standard", Category: "Execution & tools", Description: "C++23 by default; newer standard modes require support from the selected compiler."},
		{Key: "cpp.sanitizers", Label: "C++ sanitizers", Category: "Execution & tools", Description: "Optional address/undefined behavior diagnostics. References use the same options; leak reporting is disabled because each case owns its process.", Choices: []string{"none", "address", "undefined", "address,undefined"}},
		{Key: "python.interpreter", Label: "Python interpreter", Category: "Execution & tools", Description: "Empty keeps hc's managed .hc/venv. A command or path selects an external Python and bypasses management. Editing never sets up an environment."},
		{Key: "agent.name", Label: "Coding agent", Category: "Coding agent", Description: "Preset for the next generation or repair. A nonempty agent.command overrides this preset.", Choices: []string{"claude", "codex"}},
		{Key: "agent.command", Label: "Custom agent command", Category: "Coding agent", Description: "An argv array, not a shell command. Edit hc.yaml for custom arguments; reset to [] to use the preset.", YAMLOnly: true},
		{Key: "leetcode.site", Label: "LeetCode site", Category: "LeetCode", Description: "Default for fetch and login. Online submissions use the currently signed-in account.", Choices: []string{"com", "cn"}},
	}
}

// SettingValue separates the saved preference from its effective value.
type SettingValue struct {
	Setting
	Saved, Effective, Source, Note string
	Inherited                      bool
}

func settingValues(c workspace.Config) map[string]string {
	command := c.Agent.Command
	if command == nil {
		command = []string{}
	}
	argv, _ := json.Marshal(command)
	return map[string]string{
		"language": c.Language, "locale": c.Locale, "editor": c.Editor, "theme": c.Theme, "images": c.Images, "timeout": c.Timeout,
		"execution.backend": c.Execution.Backend, "go.command": c.Go.Command, "go.debugger": c.Go.Debugger,
		"cpp.command": c.CPP.Command, "cpp.debugger": c.CPP.Debugger, "cpp.standard": c.CPP.Standard, "cpp.sanitizers": c.CPP.Sanitizers,
		"python.interpreter": c.Python.Interpreter, "agent.name": c.Agent.Name, "agent.command": string(argv), "leetcode.site": c.LeetCode.Site,
	}
}

// Preferences returns workspace settings when scope is empty, otherwise only
// the two per-pack overrides. Reading never initializes a language environment.
func (a *App) Preferences(scope string) ([]SettingValue, error) {
	values := settingValues(a.WS.Config)
	var k *workspace.Pack
	if scope != "" {
		var err error
		k, err = a.WS.Pack(scope)
		if err != nil {
			return nil, err
		}
	}
	var result []SettingValue
	for _, def := range Settings() {
		if scope != "" && def.Key != "language" && def.Key != "locale" {
			continue
		}
		row := SettingValue{Setting: def, Saved: values[def.Key], Effective: values[def.Key], Source: "Workspace"}
		if scope != "" {
			pref := a.WS.Config.PackSettings[scope]
			row.Saved = pref.Language
			if def.Key == "locale" {
				row.Saved = pref.Locale
			}
			row.Inherited = row.Saved == ""
			if !row.Inherited {
				row.Source = "This pack"
				row.Effective = row.Saved
			}
			if def.Key == "language" {
				row.Choices = k.Languages()
				l, note, err := a.PackLanguage(k, "")
				if err != nil {
					row.Note = err.Error()
				} else {
					row.Effective = l.ID()
					row.Note = note
				}
			} else {
				row.Choices = k.Manifest.Locales
				preferred := a.preferredLocale(k)
				row.Effective = k.Manifest.DefaultLocale()
				if slices.Contains(k.Manifest.Locales, preferred) {
					row.Effective = preferred
				}
				row.Note = a.LocaleFallback(k, row.Effective)
				if row.Effective == "" {
					row.Note = "This pack has one unnamed statement language."
				}
			}
		}
		if def.Key == "agent.name" && len(a.WS.Config.Agent.Command) > 0 {
			row.Note = "Overridden by agent.command; reset the custom command to use this preset."
		}
		result = append(result, row)
	}
	return result, nil
}

// SavePreference validates a single edit before persisting it. Reset writes a
// built-in default at workspace scope and deletes a pack override at pack scope.
// A missing executable is a warning, never a reason to install tools or reject
// a path the user plans to provide later.
func (a *App) SavePreference(scope, key, value string, reset bool) (warning string, err error) {
	// Every write rereads hc.yaml under its lock. Even a pack-only edit may
	// pick up tool paths that another process changed in the meantime.
	defer func() {
		if err == nil {
			a.Langs = Languages(a.WS)
		}
	}()
	defs := Settings()
	i := slices.IndexFunc(defs, func(s Setting) bool { return s.Key == key })
	if i < 0 {
		return "", fmt.Errorf("unknown setting %q", key)
	}
	def := defs[i]
	if scope != "" {
		if key != "language" && key != "locale" {
			return "", fmt.Errorf("%s is a workspace setting", key)
		}
		k, err := a.WS.Pack(scope)
		if err != nil {
			return "", err
		}
		if reset {
			return "", a.WS.ResetPackSetting(scope, key)
		}
		if key == "language" {
			return "", a.SetLanguage(k, value)
		}
		if err := CheckLocale(k, value); err != nil {
			return "", err
		}
		return "", a.WS.SavePackSetting(scope, key, value)
	}
	if reset {
		value = settingValues(workspace.DefaultConfig())[key]
	}
	if def.YAMLOnly {
		if !reset {
			return "", fmt.Errorf("edit %s in hc.yaml", key)
		}
		return "", a.WS.SaveGlobalSetting(key, []string{})
	}
	if key == "language" {
		id, err := LanguageID(value)
		if err != nil {
			return "", err
		}
		value = id
	}
	if len(def.Choices) > 0 && !slices.Contains(def.Choices, value) {
		return "", fmt.Errorf("%s must be one of %s", key, strings.Join(def.Choices, ", "))
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("%s must be a single line", key)
	}
	switch key {
	case "locale":
		if err := problem.CheckLocales([]string{value}); err != nil {
			return "", err
		}
	case "timeout":
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return "", fmt.Errorf("timeout must be a positive duration, such as 10s")
		}
	case "cpp.standard":
		if err := cpp.CheckStandard(value); err != nil {
			return "", err
		}
	case "go.command", "go.debugger", "cpp.command", "cpp.debugger":
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("%s must name an executable", key)
		}
	}
	if err := a.WS.SaveGlobalSetting(key, value); err != nil {
		return "", err
	}
	if key == "cpp.command" || key == "cpp.debugger" || key == "go.command" || key == "go.debugger" || key == "python.interpreter" && value != "" {
		if _, err := lang.ResolveExecutable(lang.ExecutableName(a.WS.Root, value)); err != nil {
			return "Saved; executable not found or not executable. Run doctor to inspect the environment.", nil
		}
	}
	return "", nil
}
