package compactor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/compaction"
	"github.com/docker/docker-agent/pkg/session"
)

func TestExtractMessages(t *testing.T) {
	t.Parallel()

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
			contextLimit:             MaxSummaryTokens + 50,
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
			t.Parallel()
			sess := session.New(session.WithMessages(tt.messages))
			a := agent.New("test", "test prompt")
			result, _ := extractMessages(sess, a, tt.contextLimit, tt.additionalPrompt)

			require.GreaterOrEqual(t, len(result), tt.wantConversationMsgCount+2)
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
			assert.Len(t, result[1:len(result)-1], tt.wantConversationMsgCount)

			// Verify cost and cache control are cleared on conversation messages
			for i := 1; i < len(result)-1; i++ {
				assert.Zero(t, result[i].Cost)
				assert.False(t, result[i].CacheControl)
			}
		})
	}
}

func TestExtractMessages_KeepsRecentMessages(t *testing.T) {
	t.Parallel()

	// Create a session with many messages, some large enough that the last
	// ~MaxKeepTokens are kept aside.
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

	result, firstKeptEntry := extractMessages(sess, a, 200_000, "")

	// 20 messages × ~5k tokens = ~100k. maxKeepTokens=20k → ~4 messages kept.
	compactedMsgCount := len(result) - 2 // minus system and user prompt
	assert.Less(t, compactedMsgCount, 20, "some messages should have been kept aside")
	assert.Positive(t, compactedMsgCount, "some messages should be compacted")

	assert.Positive(t, firstKeptEntry, "firstKeptEntry should be > 0")
	assert.Less(t, firstKeptEntry, len(sess.Messages), "firstKeptEntry should be within bounds")
}

func TestComputeFirstKeptEntry(t *testing.T) {
	t.Parallel()

	a := agent.New("test", "")

	t.Run("empty session returns 0", func(t *testing.T) {
		t.Parallel()
		sess := session.New()
		assert.Equal(t, 0, ComputeFirstKeptEntry(sess, a))
	})

	t.Run("short conversation: split at end (compact everything)", func(t *testing.T) {
		t.Parallel()
		sess := session.New(session.WithMessages([]session.Item{
			session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleSystem, Content: "sys"}}),
			session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleUser, Content: "hi"}}),
			session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleAssistant, Content: "hello"}}),
		}))
		assert.Equal(t, len(sess.Messages), ComputeFirstKeptEntry(sess, a))
	})
}

func TestMapToSessionIndex(t *testing.T) {
	t.Parallel()

	sess := session.New(session.WithMessages([]session.Item{
		session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleSystem, Content: "sys"}}),
		session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleUser, Content: "u1"}}),
		session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleAssistant, Content: "a1"}}),
		session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleSystem, Content: "sys2"}}),
		session.NewMessageItem(&session.Message{Message: chat.Message{Role: chat.MessageRoleUser, Content: "u2"}}),
	}))

	// Filtered list (no system): [u1, a1, u2] → indices 0,1,2
	// Map back to sess.Messages indices: 1, 2, 4
	assert.Equal(t, 1, mapToSessionIndex(sess, 0))
	assert.Equal(t, 2, mapToSessionIndex(sess, 1))
	assert.Equal(t, 4, mapToSessionIndex(sess, 2))
	// Past the end: returns len(sess.Messages)
	assert.Equal(t, len(sess.Messages), mapToSessionIndex(sess, 3))
}

// TestRunLLM_RequiresRunAgent pins the contract that a missing RunAgent
// callback is rejected loudly rather than silently no-oping.
func TestRunLLM_RequiresRunAgent(t *testing.T) {
	t.Parallel()

	sess := session.New()
	a := agent.New("test", "test")

	_, err := RunLLM(t.Context(), LLMArgs{
		Session:      sess,
		Agent:        a,
		ContextLimit: 100_000,
		// RunAgent intentionally nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RunAgent")
}

// TestRunLLM_RequiresContextLimit pins that the LLM strategy refuses
// to run without a real context budget — it would otherwise feed an
// empty conversation to the model.
func TestRunLLM_RequiresContextLimit(t *testing.T) {
	t.Parallel()

	sess := session.New()
	a := agent.New("test", "test")

	_, err := RunLLM(t.Context(), LLMArgs{
		Session:      sess,
		Agent:        a,
		ContextLimit: 0,
		RunAgent: func(context.Context, *agent.Agent, *session.Session) error {
			return errors.New("should not be called")
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ContextLimit")
}
