// Package agent delegates test case authoring to a coding agent CLI such as
// Claude Code or Codex.
//
// Rather than a network protocol (A2A/MCP), hc runs the agent's headless mode
// as a subprocess in the workspace, with a prompt that points at the hc
// skills installed in the workspace. The agent writes files and
// calls back into hc (`hc cases build`, `hc validate --run`) to verify them.
package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/skill"
)

// Skills lists the skills the prompts rely on. Every context's skill set
// has them, and hc installs the workspace's set before it starts an agent.
var Skills = []string{skill.Problem, skill.Cases}

// Agent is a configured agent command.
type Agent struct {
	Name string
	// Argv is the command line; "{prompt}" is replaced by the prompt.
	Argv []string
	// SkillDir is the directory of the skill copies the agent reads,
	// relative to the workspace root.
	SkillDir string
}

// Presets maps agent names to their headless command lines. Both keep writes
// inside the workspace (and temporary directories), so hc must prepare what
// the commands it runs need elsewhere before it starts them. Claude: Write/Edit are left out of the allowlist so
// acceptEdits approves them only in the working directory, and the sandbox
// confines what Bash commands (python3, go) can write. Codex: --full-auto
// runs in its workspace-write sandbox.
var Presets = map[string][]string{
	"claude": {
		"claude", "-p", "{prompt}",
		"--permission-mode", "acceptEdits",
		"--allowedTools", "Read,Glob,Grep,Bash(hc:*),Bash(python3:*),Bash(go:*),Bash(ls:*),Bash(cat:*),Bash(head:*),Bash(wc:*)",
		"--settings", `{"sandbox":{"enabled":true,"autoAllowBashIfSandboxed":true,"allowUnsandboxedCommands":false}}`,
	},
	"codex": {"codex", "exec", "--full-auto", "{prompt}"},
}

// Resolve returns the agent for a preset name or a custom command. Claude
// Code reads the skills in .claude/skills; Codex, and any other command,
// those in .agents/skills.
func Resolve(name string, command []string) (*Agent, error) {
	if len(command) > 0 {
		return &Agent{Name: command[0], Argv: command, SkillDir: skillDir(filepath.Base(command[0]))}, nil
	}
	if name == "" {
		name = "claude"
	}
	argv, ok := Presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (presets: claude, codex; or set agent.command in hc.yaml)", name)
	}
	return &Agent{Name: name, Argv: argv, SkillDir: skillDir(name)}, nil
}

func skillDir(command string) string {
	if command == "claude" {
		return skill.ClaudeDir
	}
	return skill.AgentsDir
}

// Run executes the agent in dir with the prompt, streaming its output. env
// adds to the agent's environment.
func (a *Agent) Run(ctx context.Context, dir, prompt string, env []string) error {
	argv := make([]string, len(a.Argv))
	replaced := false
	for i, arg := range a.Argv {
		if strings.Contains(arg, "{prompt}") {
			arg = strings.ReplaceAll(arg, "{prompt}", prompt)
			replaced = true
		}
		argv[i] = arg
	}
	if !replaced {
		argv = append(argv, prompt)
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return fmt.Errorf("agent %q not found in PATH: %w", argv[0], err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Make sure the agent's `hc` calls resolve to this binary and workspace.
	env = append(append(os.Environ(), env...), "HC_WORKSPACE="+dir)
	if exe, err := os.Executable(); err == nil {
		env = append(env, "PATH="+filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	cmd.Env = env
	return cmd.Run()
}

// skillPath is the path of a skill's SKILL.md in the agent's copy.
func (a *Agent) skillPath(name string) string {
	return path.Join(a.SkillDir, name, skill.MainFile)
}

// useSkill is the sentence that starts a prompt.
func (a *Agent) useSkill(name, task string) string {
	return fmt.Sprintf("Use the %s skill (read %s) to %s.\n\n", name, a.skillPath(name), task)
}

// definitionSkill is the sentence that sends the agent to the hc-problem
// skill when the problem itself, not its cases, is wrong.
func (a *Agent) definitionSkill() string {
	return fmt.Sprintf("If the problem's definition itself is wrong (its signature, structure inputs, statement or templates), fix it with the %s skill (read %s) first.\n", skill.Problem, a.skillPath(skill.Problem))
}

// GeneratePrompt asks the agent to write a reference solution and cases for
// the problem ref (pack/slug) in problemDir, relative to the workspace.
func (a *Agent) GeneratePrompt(ref, problemDir string, count int, extra string) string {
	var b strings.Builder
	b.WriteString(a.useSkill(skill.Cases, fmt.Sprintf("generate test cases for the hc problem %s in %s", ref, problemDir)))
	fmt.Fprintf(&b, "Name the problem %s in hc commands.\n", ref)
	fmt.Fprintf(&b, "Follow its \"generate test cases\" workflow: decide the judge rule from the statement, write the reference solutions, add the cases, and finish when `hc validate %s --run` passes.\n", ref)
	if count > 0 {
		fmt.Fprintf(&b, "Target about %d cases in total.\n", count)
	}
	b.WriteString(a.definitionSkill())
	b.WriteString("Do not touch the solutions/ directory.\n")
	if extra != "" {
		fmt.Fprintf(&b, "\nAdditional instructions from the user: %s\n", extra)
	}
	return b.String()
}

// FixPrompt asks the agent to investigate and fix test cases of the problem
// ref (pack/slug) in problemDir.
func (a *Agent) FixPrompt(ref, problemDir string, cases []int, complaint, solutionPath string) string {
	var b strings.Builder
	b.WriteString(a.useSkill(skill.Cases, fmt.Sprintf("fix test cases of the hc problem %s in %s", ref, problemDir)))
	fmt.Fprintf(&b, "Name the problem %s in hc commands.\n", ref)
	if len(cases) > 0 {
		fmt.Fprintf(&b, "Suspicious case number(s): %v (inspect with `hc cases list %s --case N`).\n", cases, ref)
	}
	if complaint != "" {
		fmt.Fprintf(&b, "The user reports: %s\n", complaint)
	}
	if solutionPath != "" {
		fmt.Fprintf(&b, "The user's solution is %s — read it only to understand the dispute, never modify it.\n", solutionPath)
	}
	b.WriteString("Follow its \"fix a test case\" workflow. If the test case is correct and the user's solution is wrong, explain why instead of changing anything.\n")
	b.WriteString(a.definitionSkill())
	return b.String()
}
