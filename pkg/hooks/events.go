package hooks

// StdoutPolicy describes how plain stdout from a successful hook is
// interpreted when it is not JSON. Most events ignore it; context events
// route it to Result.AdditionalContext.
type StdoutPolicy int

const (
	StdoutIgnored StdoutPolicy = iota
	StdoutAdditionalContext
)

// EventSpec is the single place that describes hook-event semantics.
// Keep behavior here rather than scattering event-specific conditionals
// through the executor.
type EventSpec struct {
	Type         EventType
	ToolScoped   bool
	FailClosed   bool
	StdoutPolicy StdoutPolicy
}

var eventSpecs = []EventSpec{
	{Type: EventPreToolUse, ToolScoped: true, FailClosed: true},
	{Type: EventPostToolUse, ToolScoped: true, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventPermissionRequest, ToolScoped: true},
	{Type: EventSessionStart, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventUserPromptSubmit, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventTurnStart, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventBeforeLLMCall},
	{Type: EventAfterLLMCall},
	{Type: EventSessionEnd},
	{Type: EventPreCompact, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventSubagentStop},
	{Type: EventOnUserInput},
	{Type: EventStop, StdoutPolicy: StdoutAdditionalContext},
	{Type: EventNotification},
	{Type: EventOnError},
	{Type: EventOnMaxIterations},
}

var eventSpecByType = func() map[EventType]EventSpec {
	m := make(map[EventType]EventSpec, len(eventSpecs))
	for _, spec := range eventSpecs {
		m[spec.Type] = spec
	}
	return m
}()

// EventSpecs returns the known hook event specifications in stable order.
func EventSpecs() []EventSpec {
	return append([]EventSpec(nil), eventSpecs...)
}

func eventSpec(event EventType) EventSpec {
	if spec, ok := eventSpecByType[event]; ok {
		return spec
	}
	return EventSpec{Type: event}
}
