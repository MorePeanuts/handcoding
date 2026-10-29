package agent

import (
	"strings"
	"testing"
)

// TestSkillDir checks that each agent's prompt cites the skill copy that
// agent reads.
func TestSkillDir(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command []string
		want    string
	}{
		{"claude", nil, ".claude/skills/hc-cases/SKILL.md"},
		{"codex", nil, ".agents/skills/hc-cases/SKILL.md"},
		{"", []string{"/usr/local/bin/claude", "-p"}, ".claude/skills/hc-cases/SKILL.md"},
		{"", []string{"my-agent"}, ".agents/skills/hc-cases/SKILL.md"},
	} {
		a, err := Resolve(tt.name, tt.command)
		if err != nil {
			t.Fatal(err)
		}
		for _, prompt := range []string{
			a.GeneratePrompt("local/two-sum", "packs/local/problems/0001-two-sum", 0, ""),
			a.FixPrompt("local/two-sum", "packs/local/problems/0001-two-sum", nil, "", ""),
		} {
			if !strings.Contains(prompt, "(read "+tt.want+")") || !strings.Contains(prompt, strings.Replace(tt.want, "hc-cases", "hc-problem", 1)) {
				t.Errorf("%s %v: prompt does not cite %s:\n%s", tt.name, tt.command, tt.want, prompt)
			}
			// hc translates nothing: the agent keeps to the default locale.
			if !strings.Contains(prompt, "never write, edit or translate a statement in another locale") {
				t.Errorf("%s %v: prompt does not keep the agent to description.md:\n%s", tt.name, tt.command, prompt)
			}
		}
	}
}
