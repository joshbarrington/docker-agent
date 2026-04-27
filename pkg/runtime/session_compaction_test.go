package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/compaction"
	"github.com/docker/docker-agent/pkg/session"
)

func TestExtractMessagesToCompact(t *testing.T) {
	newMsg := func(role chat.MessageRole, content string) session.Item {
		return session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: role, Content: content},
		})
	}

	tests := []struct {
		name                     string
		messages                 []session.Item
		contextLimit             int64
		additionalPrompt         string
		wantConversationMsgCount int
	}{
		{
			name:                     "empty session returns system and user prompt only",
			messages:                 nil,
			contextLimit:             100_000,
			wantConversationMsgCount: 0,
		},
		{
			name: "system messages are filtered out",
			messages: []session.Item{
				newMsg(chat.MessageRoleSystem, "system instruction"),
				newMsg(chat.MessageRoleUser, "hello"),
				newMsg(chat.MessageRoleAssistant, "hi"),
			},
			contextLimit:             100_000,
			wantConversationMsgCount: 2,
		},
		{
			name: "messages fit within context limit",
			messages: []session.Item{
				newMsg(chat.MessageRoleUser, "msg1"),
				newMsg(chat.MessageRoleAssistant, "msg2"),
				newMsg(chat.MessageRoleUser, "msg3"),
				newMsg(chat.MessageRoleAssistant, "msg4"),
			},
			contextLimit:             100_000,
			wantConversationMsgCount: 4,
		},
		{
			name: "truncation when context limit is very small",
			messages: []session.Item{
				newMsg(chat.MessageRoleUser, "first message with lots of content that takes tokens"),
				newMsg(chat.MessageRoleAssistant, "first response with lots of content that takes tokens"),
				newMsg(chat.MessageRoleUser, "second message"),
				newMsg(chat.MessageRoleAssistant, "second response"),
			},
			// Set context limit so small that after subtracting maxSummaryTokens + prompt overhead,
			// not all messages fit.
			contextLimit:             maxSummaryTokens + 50,
			wantConversationMsgCount: 0,
		},
		{
			name: "additional prompt is appended",
			messages: []session.Item{
				newMsg(chat.MessageRoleUser, "hello"),
			},
			contextLimit:             100_000,
			additionalPrompt:         "focus on code quality",
			wantConversationMsgCount: 1,
		},
		{
			name: "cost and cache control are cleared",
			messages: []session.Item{
				session.NewMessageItem(&session.Message{
					Message: chat.Message{
						Role:         chat.MessageRoleUser,
						Content:      "hello",
						Cost:         1.5,
						CacheControl: true,
					},
				}),
			},
			contextLimit:             100_000,
			wantConversationMsgCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := session.New(session.WithMessages(tt.messages))

			a := agent.New("test", "test prompt")
			result, _ := extractMessagesToCompact(sess, a, tt.contextLimit, tt.additionalPrompt, time.Now)

			assert.GreaterOrEqual(t, len(result), tt.wantConversationMsgCount+2)
			assert.Equal(t, chat.MessageRoleSystem, result[0].Role)
			assert.Equal(t, compaction.SystemPrompt, result[0].Content)

			last := result[len(result)-1]
			assert.Equal(t, chat.MessageRoleUser, last.Role)
			expectedPrompt := compaction.UserPrompt
			if tt.additionalPrompt != "" {
				expectedPrompt += "\n\n" + tt.additionalPrompt
			}
			assert.Equal(t, expectedPrompt, last.Content)

			// Conversation messages are all except first (system) and last (user prompt)
			assert.Equal(t, tt.wantConversationMsgCount, len(result)-2)

			// Verify cost and cache control are cleared on conversation messages
			for i := 1; i < len(result)-1; i++ {
				assert.Zero(t, result[i].Cost)
				assert.False(t, result[i].CacheControl)
			}
		})
	}
}

func TestSplitIndexForKeep(t *testing.T) {
	msg := func(role chat.MessageRole, content string) chat.Message {
		return chat.Message{Role: role, Content: content}
	}

	tests := []struct {
		name      string
		messages  []chat.Message
		maxTokens int64
		wantSplit int // expected split index
	}{
		{
			name:      "empty messages",
			messages:  nil,
			maxTokens: 1000,
			wantSplit: 0,
		},
		{
			name: "all messages fit in keep budget - compact everything",
			messages: []chat.Message{
				msg(chat.MessageRoleUser, "short"),
				msg(chat.MessageRoleAssistant, "short"),
			},
			maxTokens: 100_000,
			wantSplit: 2, // all fit → compact everything
		},
		{
			name: "recent messages kept, older ones compacted",
			messages: []chat.Message{
				msg(chat.MessageRoleUser, strings.Repeat("a", 40000)),      // ~10005 tokens
				msg(chat.MessageRoleAssistant, strings.Repeat("b", 40000)), // ~10005 tokens
				msg(chat.MessageRoleUser, strings.Repeat("c", 40000)),      // ~10005 tokens
				msg(chat.MessageRoleAssistant, strings.Repeat("d", 40000)), // ~10005 tokens
				msg(chat.MessageRoleUser, strings.Repeat("e", 40000)),      // ~10005 tokens
				msg(chat.MessageRoleAssistant, strings.Repeat("f", 40000)), // ~10005 tokens
			},
			maxTokens: 20_100, // enough for exactly 2 messages
			wantSplit: 4,      // last 2 messages are kept
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitIndexForKeep(tt.messages, tt.maxTokens)
			assert.Equal(t, tt.wantSplit, got)
		})
	}
}

func TestExtractMessagesToCompact_KeepsRecentMessages(t *testing.T) {
	// Create a session with many messages, some large enough that the last
	// ~20k tokens are kept aside.
	var items []session.Item
	for range 10 {
		items = append(items, session.NewMessageItem(&session.Message{
			Message: chat.Message{
				Role:    chat.MessageRoleUser,
				Content: strings.Repeat("x", 20000), // ~5k tokens each
			},
		}), session.NewMessageItem(&session.Message{
			Message: chat.Message{
				Role:    chat.MessageRoleAssistant,
				Content: strings.Repeat("y", 20000), // ~5k tokens each
			},
		}))
	}

	sess := session.New(session.WithMessages(items))
	a := agent.New("test", "test prompt")

	result, firstKeptEntry := extractMessagesToCompact(sess, a, 200_000, "", time.Now)

	// The kept messages should not appear in the compaction result
	// (only system + compacted messages + user prompt).
	// Total: 20 messages × ~5k tokens = ~100k tokens.
	// Keep budget: 20k tokens → ~4 messages kept.
	// So compacted messages should be 20 - 4 = 16.
	compactedMsgCount := len(result) - 2 // minus system and user prompt
	assert.Less(t, compactedMsgCount, 20, "some messages should have been kept aside")
	assert.Positive(t, compactedMsgCount, "some messages should be compacted")

	// firstKeptEntry should point into sess.Messages
	assert.Positive(t, firstKeptEntry, "firstKeptEntry should be > 0")
	assert.Less(t, firstKeptEntry, len(sess.Messages), "firstKeptEntry should be within bounds")
}

func TestSessionGetMessages_WithFirstKeptEntry(t *testing.T) {
	// Build a session with some messages, then add a summary with FirstKeptEntry.
	items := []session.Item{
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleUser, Content: "m1"},
		}),
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleAssistant, Content: "m2"},
		}),
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleUser, Content: "m3"},
		}),
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleAssistant, Content: "m4"},
		}),
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleUser, Content: "m5"},
		}),
	}

	// Add summary that says "first kept entry is index 3" (m4).
	// So we expect: [system...] + [summary] + [m4, m5]
	items = append(items, session.Item{
		Summary:        "This is a summary of m1-m3",
		FirstKeptEntry: 3, // index of m4 in the Messages slice
	})

	sess := session.New(session.WithMessages(items))
	a := agent.New("test", "test instruction")

	messages := sess.GetMessages(a)

	// Extract just the non-system messages
	var conversationMessages []chat.Message
	for _, msg := range messages {
		if msg.Role != chat.MessageRoleSystem {
			conversationMessages = append(conversationMessages, msg)
		}
	}

	// Should have: summary (as user message), m4, m5
	require.Len(t, conversationMessages, 3, "expected summary + 2 kept messages")
	assert.Contains(t, conversationMessages[0].Content, "Session Summary:")
	assert.Equal(t, "m4", conversationMessages[1].Content)
	assert.Equal(t, "m5", conversationMessages[2].Content)
}

func TestSessionGetMessages_SummaryWithoutFirstKeptEntry(t *testing.T) {
	// Backward compatibility: summary without FirstKeptEntry should work as before.
	items := []session.Item{
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleUser, Content: "m1"},
		}),
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleAssistant, Content: "m2"},
		}),
		{Summary: "This is a summary"},
		session.NewMessageItem(&session.Message{
			Message: chat.Message{Role: chat.MessageRoleUser, Content: "m3"},
		}),
	}

	sess := session.New(session.WithMessages(items))
	a := agent.New("test", "test instruction")

	messages := sess.GetMessages(a)

	var conversationMessages []chat.Message
	for _, msg := range messages {
		if msg.Role != chat.MessageRoleSystem {
			conversationMessages = append(conversationMessages, msg)
		}
	}

	// Should have: summary + m3 (messages after the summary)
	require.Len(t, conversationMessages, 2)
	assert.Contains(t, conversationMessages[0].Content, "Session Summary:")
	assert.Equal(t, "m3", conversationMessages[1].Content)
}
