package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/tools"
	"github.com/docker/docker-agent/pkg/tools/builtin"
)

// handleRunSkill executes a skill as an isolated sub-agent. The skill's
// SKILL.md content (with command expansions) becomes the system prompt, and
// the caller-provided task becomes the implicit user message. The sub-agent
// runs in a child session using the current agent's model and tools, and
// its final response is returned as the tool result.
//
// All skill-specific business rules (lookup, fork-mode validation, content
// expansion) live in (*builtin.SkillsToolset).PrepareForkSubSession; this
// handler keeps only the runtime-private orchestration: sub-session creation,
// OpenTelemetry tracing, and event forwarding.
//
// This implements the `context: fork` behaviour from the SKILL.md frontmatter,
// following the same convention as Claude Code.
func (r *LocalRuntime) handleRunSkill(ctx context.Context, sess *session.Session, toolCall tools.ToolCall, evts chan Event) (*tools.ToolCallResult, error) {
	var args builtin.RunSkillArgs
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	st := r.CurrentAgentSkillsToolset()
	if st == nil {
		return tools.ResultError("no skills are available for the current agent"), nil
	}

	prepared, errResult := st.PrepareForkSubSession(ctx, args)
	if errResult != nil {
		return errResult, nil
	}

	a := r.CurrentAgent()
	ca := a.Name()

	ctx, span := r.startSpan(ctx, "runtime.run_skill", trace.WithAttributes(
		attribute.String("agent", ca),
		attribute.String("skill", prepared.SkillName),
		attribute.String("session.id", sess.ID),
	))
	defer span.End()

	slog.Debug("Running skill as sub-agent",
		"agent", ca,
		"skill", prepared.SkillName,
		"task", prepared.Task,
	)

	// If the skill declares a model override, apply it for the duration of
	// the sub-session. WithAgentModel handles every accepted form (named
	// model, alloy, inline provider/model, inline alloy) and returns a
	// CAS-safe restore func that is always non-nil; on failure we log a
	// warning and fall back to the agent's currently-active model.
	if prepared.Model != "" {
		restore, err := r.WithAgentModel(ctx, ca, prepared.Model)
		defer restore()
		if err != nil {
			slog.Warn("Failed to apply skill model override; using current model",
				"agent", ca,
				"skill", prepared.SkillName,
				"model", prepared.Model,
				"error", err,
			)
		}
	}

	cfg := SubSessionConfig{
		Task:                prepared.Task,
		SystemMessage:       prepared.Content,
		ImplicitUserMessage: prepared.Task,
		AgentName:           ca,
		Title:               "Skill: " + prepared.SkillName,
		ToolsApproved:       sess.ToolsApproved,
		ExcludedTools:       []string{builtin.ToolNameRunSkill},
	}

	s := newSubSession(sess, cfg, a)
	return r.runSubSessionForwarding(ctx, sess, s, span, evts, ca)
}
