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
// This implements the `context: fork` behaviour from the SKILL.md
// frontmatter, following the same convention as Claude Code.
func (r *LocalRuntime) handleRunSkill(ctx context.Context, sess *session.Session, toolCall tools.ToolCall, evts chan Event) (*tools.ToolCallResult, error) {
	var params builtin.RunSkillArgs
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	st := r.CurrentAgentSkillsToolset()
	if st == nil {
		return tools.ResultError("no skills are available for the current agent"), nil
	}

	skill := st.FindSkill(params.Name)
	if skill == nil {
		return tools.ResultError(fmt.Sprintf("skill %q not found", params.Name)), nil
	}

	if !skill.IsFork() {
		return tools.ResultError(fmt.Sprintf(
			"skill %q is not configured for sub-agent execution (missing context: fork in SKILL.md frontmatter); use read_skill instead",
			params.Name,
		)), nil
	}

	// Load and expand the skill content for the system prompt.
	skillContent, err := st.ReadSkillContent(ctx, params.Name)
	if err != nil {
		return tools.ResultError(fmt.Sprintf("failed to read skill content: %s", err)), nil
	}

	a := r.CurrentAgent()
	ca := a.Name()

	ctx, span := r.startSpan(ctx, "runtime.run_skill", trace.WithAttributes(
		attribute.String("agent", ca),
		attribute.String("skill", params.Name),
		attribute.String("session.id", sess.ID),
	))
	defer span.End()

	slog.Debug("Running skill as sub-agent",
		"agent", ca,
		"skill", params.Name,
		"task", params.Task,
	)

	// If the skill declares a model override, apply it for the duration of
	// the sub-session and restore the previous override when done.
	// SetAgentModel handles every accepted form (named model, alloy, inline
	// provider/model, inline alloy); on failure we just log a warning and
	// fall back to the agent's currently-active model.
	//
	// We snapshot before and after the apply so the deferred restore can
	// CAS back to the previous value only if no concurrent caller (e.g. the
	// TUI model picker) changed the override in the meantime — otherwise
	// the user's choice would be silently reverted on skill completion.
	if skill.Model != "" {
		prev := a.SnapshotModelOverride()
		if err := r.SetAgentModel(ctx, ca, skill.Model); err != nil {
			slog.Warn("Failed to apply skill model override; using current model",
				"agent", ca,
				"skill", params.Name,
				"model", skill.Model,
				"error", err,
			)
		} else {
			ours := a.SnapshotModelOverride()
			defer a.RestoreModelOverride(prev, ours)
		}
	}

	cfg := SubSessionConfig{
		Task:                params.Task,
		SystemMessage:       skillContent,
		ImplicitUserMessage: params.Task,
		AgentName:           ca,
		Title:               "Skill: " + params.Name,
		ToolsApproved:       sess.ToolsApproved,
		ExcludedTools:       []string{builtin.ToolNameRunSkill},
	}

	s := newSubSession(sess, cfg, a)
	return r.runSubSessionForwarding(ctx, sess, s, span, evts, ca)
}
