package hooks

import (
	"github.com/docker/docker-agent/pkg/config/latest"
)

// The hooks package and the persisted [latest.HooksConfig] used to
// declare two parallel struct hierarchies with identical fields, just
// to attach a single helper method (GetTimeout) and a typed Type
// string. The cost was a 12-events-listed-in-four-places translation
// layer that grew every time a new event was added.
//
// Today they're the same types: aliases below give the runtime the
// short names it always used, while the single source of truth (and
// the YAML/JSON tags, the validate() method, and IsEmpty) lives next
// to the schema in pkg/config/latest. Adding a new event is a one-line
// change on [latest.HooksConfig] plus one line in compileEvents.
type (
	// Config is the hooks configuration for an agent.
	Config = latest.HooksConfig
	// Hook is a single hook entry. The Type field is one of the
	// HookType* constants below; unrecognised values are rejected by
	// the executor at registry lookup.
	Hook = latest.HookDefinition
	// MatcherConfig pairs a tool-name regex with the hooks to run when
	// it matches (used by EventPreToolUse, EventPostToolUse, and
	// EventPermissionRequest).
	MatcherConfig = latest.HookMatcherConfig
)

// HookType values populate [Hook.Type]. It is an alias for string so
// hooks authored in YAML round-trip through [latest.HookDefinition]
// without any conversion; the executor validates the value at registry
// lookup time.
type HookType = string

const (
	// HookTypeCommand runs a shell command.
	HookTypeCommand HookType = "command"
	// HookTypeBuiltin dispatches to a named in-process Go function
	// registered via [Registry.RegisterBuiltin]. The name is stored in
	// [Hook.Command].
	HookTypeBuiltin HookType = "builtin"
	// HookTypeModel asks an LLM and translates the reply into the hook's
	// native [Output] shape via a [ResponseShape]. It is registered by
	// the runtime ([RegisterModelFactory]) because it depends on the
	// runtime's model provider stack.
	HookTypeModel HookType = "model"
)
