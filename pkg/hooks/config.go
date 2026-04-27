package hooks

import (
	"github.com/docker/docker-agent/pkg/config/latest"
)

// FromConfig converts a latest.HooksConfig to a hooks.Config
func FromConfig(cfg *latest.HooksConfig) *Config {
	if cfg == nil {
		return nil
	}

	result := &Config{
		PreToolUse:      convertMatchers(cfg.PreToolUse),
		PostToolUse:     convertMatchers(cfg.PostToolUse),
		SessionStart:    convertDefinitions(cfg.SessionStart),
		TurnStart:       convertDefinitions(cfg.TurnStart),
		BeforeLLMCall:   convertDefinitions(cfg.BeforeLLMCall),
		AfterLLMCall:    convertDefinitions(cfg.AfterLLMCall),
		SessionEnd:      convertDefinitions(cfg.SessionEnd),
		OnUserInput:     convertDefinitions(cfg.OnUserInput),
		Stop:            convertDefinitions(cfg.Stop),
		Notification:    convertDefinitions(cfg.Notification),
		OnError:         convertDefinitions(cfg.OnError),
		OnMaxIterations: convertDefinitions(cfg.OnMaxIterations),
	}
	return result
}

// convertMatchers converts a slice of [latest.HookMatcherConfig] entries into
// the internal [MatcherConfig] shape. Returns nil for an empty input so the
// caller's per-event slice stays nil-typed when nothing is configured.
func convertMatchers(in []latest.HookMatcherConfig) []MatcherConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]MatcherConfig, 0, len(in))
	for _, matcher := range in {
		out = append(out, MatcherConfig{
			Matcher: matcher.Matcher,
			Hooks:   convertDefinitions(matcher.Hooks),
		})
	}
	return out
}

// convertDefinitions converts a slice of [latest.HookDefinition] into the
// internal [Hook] shape. Returns nil for an empty input.
func convertDefinitions(in []latest.HookDefinition) []Hook {
	if len(in) == 0 {
		return nil
	}
	out := make([]Hook, 0, len(in))
	for _, h := range in {
		out = append(out, Hook{
			Type:    HookType(h.Type),
			Command: h.Command,
			Args:    h.Args,
			Timeout: h.Timeout,
		})
	}
	return out
}
