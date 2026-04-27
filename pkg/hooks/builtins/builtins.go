// Package builtins contains the stock in-process hook implementations
// shipped with docker-agent.
//
// Available builtins:
//
//   - add_date              (turn_start)    — today's date
//   - add_environment_info  (session_start) — cwd, git, OS, arch
//   - add_prompt_files      (turn_start)    — contents of prompt files
//   - add_git_status        (turn_start)    — `git status --short --branch`
//   - add_git_diff          (turn_start)    — `git diff --stat` (or full)
//   - add_directory_listing (session_start) — top-level entries of cwd
//   - add_user_info         (session_start) — current OS user and host
//   - add_recent_commits    (session_start) — `git log --oneline -n N`
//
// They can be referenced explicitly from a hook YAML entry using
// `{type: builtin, command: "<name>"}`. The runtime also auto-injects
// add_date / add_environment_info / add_prompt_files when the matching
// agent flags are set.
//
// turn_start builtins recompute every turn (date, git state). session_start
// builtins run once per session for context that's stable for its duration.
// Each builtin lives in its own file along with its registered-name
// constant; this file holds the shared registration plumbing.
package builtins

import (
	"errors"

	"github.com/docker/docker-agent/pkg/hooks"
)

// Register installs the stock builtin hooks on r.
func Register(r *hooks.Registry) error {
	return errors.Join(
		r.RegisterBuiltin(AddDate, addDate),
		r.RegisterBuiltin(AddEnvironmentInfo, addEnvironmentInfo),
		r.RegisterBuiltin(AddPromptFiles, addPromptFiles),
		r.RegisterBuiltin(AddGitStatus, addGitStatus),
		r.RegisterBuiltin(AddGitDiff, addGitDiff),
		r.RegisterBuiltin(AddDirectoryListing, addDirectoryListing),
		r.RegisterBuiltin(AddUserInfo, addUserInfo),
		r.RegisterBuiltin(AddRecentCommits, addRecentCommits),
	)
}

// AgentDefaults captures the agent-level flags that map onto stock
// builtin hook entries. Pass each AgentConfig.AddXxx flag as-is.
type AgentDefaults struct {
	AddDate            bool
	AddEnvironmentInfo bool
	AddPromptFiles     []string
}

// ApplyAgentDefaults appends the stock builtin hook entries implied by
// d to cfg, returning the (possibly mutated) config.
//
// A nil cfg is treated as empty; the returned value is non-nil iff at
// least one hook (user-configured or auto-injected) is present.
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
