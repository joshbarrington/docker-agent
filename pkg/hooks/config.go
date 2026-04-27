package hooks

import (
	"github.com/docker/docker-agent/pkg/config/latest"
)

// FromConfig converts a [latest.HooksConfig] into the runtime [Config].
func FromConfig(cfg *latest.HooksConfig) *Config {
	if cfg == nil {
		return nil
	}
	return &Config{
		PreToolUse:      convertMatchers(cfg.PreToolUse),
		PostToolUse:     convertMatchers(cfg.PostToolUse),
		SessionStart:    convertHooks(cfg.SessionStart),
		TurnStart:       convertHooks(cfg.TurnStart),
		BeforeLLMCall:   convertHooks(cfg.BeforeLLMCall),
		AfterLLMCall:    convertHooks(cfg.AfterLLMCall),
		SessionEnd:      convertHooks(cfg.SessionEnd),
		OnUserInput:     convertHooks(cfg.OnUserInput),
		Stop:            convertHooks(cfg.Stop),
		Notification:    convertHooks(cfg.Notification),
		OnError:         convertHooks(cfg.OnError),
		OnMaxIterations: convertHooks(cfg.OnMaxIterations),
	}
}

func convertMatchers(in []latest.HookMatcherConfig) []MatcherConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]MatcherConfig, len(in))
	for i, m := range in {
		out[i] = MatcherConfig{Matcher: m.Matcher, Hooks: convertHooks(m.Hooks)}
	}
	return out
}

func convertHooks(in []latest.HookDefinition) []Hook {
	if len(in) == 0 {
		return nil
	}
	out := make([]Hook, len(in))
	for i, h := range in {
		out[i] = Hook{
			Type:    HookType(h.Type),
			Command: h.Command,
			Args:    h.Args,
			Timeout: h.Timeout,
		}
	}
	return out
}
