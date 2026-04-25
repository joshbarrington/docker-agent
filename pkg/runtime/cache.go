package runtime

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/session"
)

// tryReplayCachedResponse looks up the latest user message in the agent's
// response cache. On a hit it replays the cached answer as the assistant
// message, fires stop hooks, and returns replayed=true so the caller can
// short-circuit the run. On a miss it returns the question text so the
// caller can later store the freshly produced response under that key.
//
// It returns ("", false) when caching is disabled or the session has no
// user message to key on.
func (r *LocalRuntime) tryReplayCachedResponse(
	ctx context.Context,
	sess *session.Session,
	a *agent.Agent,
	events chan Event,
) (question string, replayed bool) {
	c := a.Cache()
	if c == nil {
		return "", false
	}
	question = sess.GetLastUserMessageContent()
	if question == "" {
		return "", false
	}
	cached, ok := c.Lookup(question)
	if !ok || cached == "" {
		return question, false
	}

	slog.Debug("Response cache hit; replaying cached answer",
		"agent", a.Name(), "session_id", sess.ID)
	modelID := a.Model().ID()
	events <- AgentInfo(a.Name(), modelID, a.Description(), a.WelcomeMessage())
	msg := chat.Message{
		Role:      chat.MessageRoleAssistant,
		Content:   cached,
		CreatedAt: time.Now().Format(time.RFC3339),
		Model:     modelID,
	}
	addAgentMessage(sess, a, &msg, events)
	r.executeStopHooks(ctx, sess, a, cached, events)
	return question, true
}

// cacheTurnResponse stores the assistant's response in the agent's cache
// under question, then clears *question so subsequent stops in the same
// RunStream (e.g. after a follow-up) are not also cached. It is a no-op
// when caching is disabled, the question is empty, or the response has
// no visible content.
func (r *LocalRuntime) cacheTurnResponse(a *agent.Agent, question *string, response string) {
	c := a.Cache()
	if c == nil || *question == "" || strings.TrimSpace(response) == "" {
		return
	}
	c.Store(*question, response)
	*question = ""
}
