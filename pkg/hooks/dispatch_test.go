package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExecutorHasIsGeneric exercises the generic Has API across all
// configured event kinds to ensure the eventTable is wired correctly. It
// guards against regressions where adding a new event to the Config
// struct silently fails to surface in Has/Dispatch.
func TestExecutorHasIsGeneric(t *testing.T) {
	t.Parallel()

	// Empty config: no event has hooks.
	empty := NewExecutor(nil, "/tmp", nil)
	for _, ev := range []EventType{
		EventPreToolUse, EventPostToolUse, EventSessionStart, EventTurnStart,
		EventBeforeLLMCall, EventAfterLLMCall,
		EventSessionEnd, EventOnUserInput, EventStop, EventNotification,
		EventOnError, EventOnMaxIterations,
	} {
		assert.Falsef(t, empty.Has(ev), "empty executor must not report Has(%s)", ev)
	}

	// Each event populated in turn — the cross-product test ensures that
	// configuring event X never makes Has(Y) true for Y != X.
	cases := []struct {
		event  EventType
		config *Config
	}{
		{EventPreToolUse, &Config{PreToolUse: []MatcherConfig{{Matcher: "*", Hooks: []Hook{{Type: HookTypeCommand, Command: "true"}}}}}},
		{EventPostToolUse, &Config{PostToolUse: []MatcherConfig{{Matcher: "*", Hooks: []Hook{{Type: HookTypeCommand, Command: "true"}}}}}},
		{EventSessionStart, &Config{SessionStart: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventTurnStart, &Config{TurnStart: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventBeforeLLMCall, &Config{BeforeLLMCall: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventAfterLLMCall, &Config{AfterLLMCall: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventSessionEnd, &Config{SessionEnd: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventOnUserInput, &Config{OnUserInput: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventStop, &Config{Stop: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventNotification, &Config{Notification: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventOnError, &Config{OnError: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
		{EventOnMaxIterations, &Config{OnMaxIterations: []Hook{{Type: HookTypeCommand, Command: "true"}}}},
	}

	for _, tc := range cases {
		exec := NewExecutor(tc.config, "/tmp", nil)
		for _, other := range []EventType{
			EventPreToolUse, EventPostToolUse, EventSessionStart, EventTurnStart,
			EventBeforeLLMCall, EventAfterLLMCall,
			EventSessionEnd, EventOnUserInput, EventStop, EventNotification,
			EventOnError, EventOnMaxIterations,
		} {
			if other == tc.event {
				assert.Truef(t, exec.Has(other), "configuring %s must light up Has(%s)", tc.event, other)
			} else {
				assert.Falsef(t, exec.Has(other), "configuring %s must NOT light up Has(%s)", tc.event, other)
			}
		}
	}

	// Unknown events fall through cleanly rather than panicking.
	assert.False(t, NewExecutor(nil, "/tmp", nil).Has(EventType("does-not-exist")))
}
