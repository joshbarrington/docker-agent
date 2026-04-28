// Package builtins contains the stock in-process hook implementations
// shipped with docker-agent.
//
// Available builtins:
//
//   - add_date              (turn_start)      — today's date
//   - add_environment_info  (session_start)   — cwd, git, OS, arch
//   - add_prompt_files      (turn_start)      — contents of prompt files
//   - add_git_status        (turn_start)      — `git status --short --branch`
//   - add_git_diff          (turn_start)      — `git diff --stat` (or full)
//   - add_directory_listing (session_start)   — top-level entries of cwd
//   - add_user_info         (session_start)   — current OS user and host
//   - add_recent_commits    (session_start)   — `git log --oneline -n N`
//   - max_iterations        (before_llm_call) — hard stop after N model calls
//
// Reference any of them from a hook YAML entry as
// `{type: builtin, command: "<name>"}`. The runtime additionally
// auto-injects add_date / add_environment_info / add_prompt_files
// from the matching agent flags.
//
// turn_start builtins recompute every turn (date, git state).
// session_start builtins run once per session for context that's
// stable for its duration. max_iterations is stateful: its
// per-session counter lives on the [State] returned by [Register];
// the runtime clears it via [State.ClearSession] from session_end.
//
// LLM-as-a-judge hooks are NOT shipped here: write `type: model` with
// `schema: pre_tool_use_decision` instead — see
// pkg/hooks/shape_pre_tool_use_decision.go and examples/llm_judge.yaml.
package builtins

import (
	"errors"

	"github.com/docker/docker-agent/pkg/hooks"
)

// State holds the per-runtime state of the stateful builtins.
// It is returned by [Register] so callers can clear per-session
// entries on session_end. Stateless builtins don't appear here.
type State struct {
	maxIterations *maxIterationsBuiltin
}

// ClearSession drops per-session state from every stateful builtin.
// A nil receiver is a no-op.
func (s *State) ClearSession(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.maxIterations.clearSession(sessionID)
}

// Register installs the stock builtin hooks on r and returns a [State]
// handle the caller must use to clear per-session state on session_end.
func Register(r *hooks.Registry) (*State, error) {
	state := &State{
		maxIterations: newMaxIterations(),
	}
	if err := errors.Join(
		r.RegisterBuiltin(AddDate, addDate),
		r.RegisterBuiltin(AddEnvironmentInfo, addEnvironmentInfo),
		r.RegisterBuiltin(AddPromptFiles, addPromptFiles),
		r.RegisterBuiltin(AddGitStatus, addGitStatus),
		r.RegisterBuiltin(AddGitDiff, addGitDiff),
		r.RegisterBuiltin(AddDirectoryListing, addDirectoryListing),
		r.RegisterBuiltin(AddUserInfo, addUserInfo),
		r.RegisterBuiltin(AddRecentCommits, addRecentCommits),
		r.RegisterBuiltin(MaxIterations, state.maxIterations.hook),
	); err != nil {
		return nil, err
	}
	return state, nil
}

// AgentDefaults captures the agent-level flags that map onto stock
// builtin hook entries. Pass each AgentConfig.AddXxx flag as-is.
type AgentDefaults struct {
	AddDate            bool
	AddEnvironmentInfo bool
	AddPromptFiles     []string
}

// ApplyAgentDefaults appends the stock builtin hook entries implied by
// d to cfg. A nil cfg is treated as empty. Returns nil iff no hook
// (user-configured or auto-injected) is present.
func ApplyAgentDefaults(cfg *hooks.Config, d AgentDefaults) *hooks.Config {
	if cfg == nil {
		cfg = &hooks.Config{}
	}
	if d.AddDate {
		cfg.TurnStart = append(cfg.TurnStart, builtinHook(AddDate))
	}
	if len(d.AddPromptFiles) > 0 {
		cfg.TurnStart = append(cfg.TurnStart, builtinHook(AddPromptFiles, d.AddPromptFiles...))
	}
	if d.AddEnvironmentInfo {
		cfg.SessionStart = append(cfg.SessionStart, builtinHook(AddEnvironmentInfo))
	}
	if cfg.IsEmpty() {
		return nil
	}
	return cfg
}

// builtinHook returns a hook entry that dispatches to the named builtin.
func builtinHook(name string, args ...string) hooks.Hook {
	return hooks.Hook{Type: hooks.HookTypeBuiltin, Command: name, Args: args}
}
