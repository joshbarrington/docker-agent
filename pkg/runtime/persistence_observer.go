package runtime

import (
	"context"
	"log/slog"
	"strings"

	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/session"
)

// PersistenceObserver is the stock [EventObserver] that mirrors the
// runtime's event stream to a [session.Store]. It encapsulates every
// persistence-related side effect that used to live in the now-deleted
// PersistentRuntime decorator:
//
//   - persists the initial session row on [EventObserver.OnRunStart]
//     for non-sub-session runs;
//   - tracks streaming assistant content (AgentChoice and
//     AgentChoiceReasoning) into a single growing message row, and
//     finalises it on [MessageAddedEvent];
//   - persists user messages, sub-session attachments, summaries,
//     token usage, and session-title updates as they fly past.
//
// Sub-session filtering and SessionScoped-mismatch filtering live
// inside [OnEvent] so callers don't have to think about them.
//
// The runtime auto-registers one of these in NewLocalRuntime against
// the runtime's configured store. Tests and embedders that want to
// drive persistence themselves can pass a no-op session.Store (or
// override via [WithEventObserver] for additional sinks).
type PersistenceObserver struct {
	store session.Store

	// streaming holds the in-flight assistant message under
	// construction during a streaming response. Reset on every
	// UserMessageEvent / MessageAddedEvent. Per-RunStream state, not
	// shared across observers, so no mutex needed: OnEvent runs
	// synchronously from the runtime's forwarding goroutine.
	streaming streamingState
}

// streamingState tracks the accumulated content for a streaming
// assistant message. Held inside a [PersistenceObserver]; see its
// fields' use sites for the per-event semantics.
type streamingState struct {
	content          strings.Builder
	reasoningContent strings.Builder
	agentName        string
	messageID        int64 // ID of the in-flight streaming row, 0 for none.
}

// newPersistenceObserver returns an observer that persists to store.
// Returns nil when store is nil so the constructor can call
// [WithEventObserver] unconditionally without a guard.
func newPersistenceObserver(store session.Store) *PersistenceObserver {
	if store == nil {
		return nil
	}
	return &PersistenceObserver{store: store}
}

// OnRunStart persists the session row before the run loop starts.
// Sub-sessions skip this: the parent session's store will absorb them
// via the SubSessionCompletedEvent handling in OnEvent.
func (p *PersistenceObserver) OnRunStart(ctx context.Context, sess *session.Session) {
	if sess.IsSubSession() {
		return
	}
	if err := p.store.UpdateSession(ctx, sess); err != nil {
		slog.Warn("Failed to persist initial session", "session_id", sess.ID, "error", err)
	}
}

// OnRunEnd is currently a no-op — every persistent side effect is
// already journaled by the time the run drains. Kept for symmetry with
// OnRunStart and to give future observers a place to flush buffered
// state.
func (p *PersistenceObserver) OnRunEnd(_ context.Context, _ *session.Session) {}

// OnEvent applies the per-event-type persistence rules. Filters two
// classes of event the store should never see:
//
//   - sub-session events: persistence runs only on the parent session,
//     and the sub-session is attached as a unit on
//     [SubSessionCompletedEvent];
//   - cross-session events: a sub-agent's streaming events are
//     forwarded through the parent runtime's channel, but tagged with
//     the sub-session's ID via [SessionScoped]; persisting them under
//     the parent would corrupt the parent's transcript.
func (p *PersistenceObserver) OnEvent(ctx context.Context, sess *session.Session, event Event) {
	if sess.IsSubSession() {
		return
	}
	if scoped, ok := event.(SessionScoped); ok && scoped.GetSessionID() != sess.ID {
		return
	}

	switch e := event.(type) {
	case *AgentChoiceEvent:
		p.streaming.content.WriteString(e.Content)
		p.streaming.agentName = e.AgentName
		p.persistStreamingContent(ctx, sess.ID)

	case *AgentChoiceReasoningEvent:
		p.streaming.reasoningContent.WriteString(e.Content)
		p.streaming.agentName = e.AgentName
		p.persistStreamingContent(ctx, sess.ID)

	case *UserMessageEvent:
		p.resetStreaming()
		if _, err := p.store.AddMessage(ctx, e.SessionID, session.UserMessage(e.Message, e.MultiContent...)); err != nil {
			slog.Warn("Failed to persist user message", "session_id", e.SessionID, "error", err)
		}

	case *MessageAddedEvent:
		// Finalise the streaming row (if any) with the canonical
		// MessageAddedEvent payload, then reset for the next stream.
		if p.streaming.messageID != 0 {
			if err := p.store.UpdateMessage(ctx, p.streaming.messageID, e.Message); err != nil {
				slog.Warn("Failed to finalize streaming message",
					"session_id", e.SessionID, "message_id", p.streaming.messageID, "error", err)
			}
		} else {
			if _, err := p.store.AddMessage(ctx, e.SessionID, e.Message); err != nil {
				slog.Warn("Failed to persist message", "session_id", e.SessionID, "error", err)
			}
		}
		p.resetStreaming()

	case *SubSessionCompletedEvent:
		if subSess, ok := e.SubSession.(*session.Session); ok {
			if err := p.store.AddSubSession(ctx, e.ParentSessionID, subSess); err != nil {
				slog.Warn("Failed to persist sub-session", "parent_id", e.ParentSessionID, "error", err)
			}
		}

	case *SessionSummaryEvent:
		if err := p.store.AddSummary(ctx, e.SessionID, e.Summary, e.FirstKeptEntry); err != nil {
			slog.Warn("Failed to persist summary", "session_id", e.SessionID, "error", err)
		}

	case *TokenUsageEvent:
		// Sub-session events flow through but must not overwrite the
		// parent session's token counts. The SessionScoped check above
		// already filters those; this guard is belt-and-braces against
		// future sub-session events that might not implement
		// SessionScoped.
		if e.Usage != nil && e.SessionID == sess.ID {
			if err := p.store.UpdateSessionTokens(ctx, sess.ID, e.Usage.InputTokens, e.Usage.OutputTokens, e.Usage.Cost); err != nil {
				slog.Warn("Failed to persist token usage", "session_id", sess.ID, "error", err)
			}
		}

	case *SessionTitleEvent:
		if err := p.store.UpdateSessionTitle(ctx, sess.ID, e.Title); err != nil {
			slog.Warn("Failed to persist session title", "session_id", sess.ID, "error", err)
		}
	}
}

// persistStreamingContent creates or updates the streaming assistant
// message row. The runtime emits one AgentChoice / AgentChoiceReasoning
// event per delta chunk, so this fires repeatedly during a streaming
// response; we keep one row open and update it in place rather than
// creating a row per chunk.
func (p *PersistenceObserver) persistStreamingContent(ctx context.Context, sessionID string) {
	msg := &session.Message{
		AgentName: p.streaming.agentName,
		Message: chat.Message{
			Role:             chat.MessageRoleAssistant,
			Content:          p.streaming.content.String(),
			ReasoningContent: p.streaming.reasoningContent.String(),
		},
	}

	if p.streaming.messageID == 0 {
		id, err := p.store.AddMessage(ctx, sessionID, msg)
		if err != nil {
			slog.Warn("Failed to create streaming message", "session_id", sessionID, "error", err)
			return
		}
		p.streaming.messageID = id
		slog.Debug("[PERSIST] Created streaming message",
			"session_id", sessionID, "message_id", id, "agent", p.streaming.agentName)
		return
	}

	if err := p.store.UpdateMessage(ctx, p.streaming.messageID, msg); err != nil {
		slog.Warn("Failed to update streaming message",
			"session_id", sessionID, "message_id", p.streaming.messageID, "error", err)
	}
}

// resetStreaming clears the in-flight streaming state so the next
// streaming response (or non-streamed message) starts fresh.
func (p *PersistenceObserver) resetStreaming() {
	p.streaming.content.Reset()
	p.streaming.reasoningContent.Reset()
	p.streaming.agentName = ""
	p.streaming.messageID = 0
}
