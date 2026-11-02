package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// KeepAgentLogs is how many logs of agent runs hc keeps per problem.
const KeepAgentLogs = 10

// AgentLogDir is the directory of the logs of the agent runs (hc gen, hc
// fix) on p, in the workspace's state directory.
func (a *App) AgentLogDir(p *problem.Problem) string {
	return filepath.Join(a.WS.StatePath(), "agent", p.Pack, p.Slug)
}

// CreateAgentLog creates the log of an agent run of task (gen, fix) on p,
// named after the time it starts, and removes the oldest logs of p beyond
// KeepAgentLogs.
func (a *App) CreateAgentLog(p *problem.Problem, task string) (*os.File, error) {
	dir := a.AgentLogDir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := time.Now().Format("20060102-150405.000") + "-" + task + ".jsonl"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	pruneAgentLogs(dir)
	return f, nil
}

// pruneAgentLogs removes the oldest logs in dir beyond KeepAgentLogs. Their
// names start with the time, so they sort by it.
func pruneAgentLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			logs = append(logs, e.Name())
		}
	}
	slices.Sort(logs)
	for _, name := range logs[:max(len(logs)-KeepAgentLogs, 0)] {
		_ = os.Remove(filepath.Join(dir, name))
	}
}
