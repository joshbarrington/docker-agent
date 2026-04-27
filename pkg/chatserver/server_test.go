package chatserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/session"
)

func TestBuildSession_RequiresUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		messages []ChatCompletionMessage
		wantNil  bool
	}{
		{
			name:    "empty list",
			wantNil: true,
		},
		{
			name: "only system messages",
			messages: []ChatCompletionMessage{
				{Role: "system", Content: "be helpful"},
			},
			wantNil: true,
		},
		{
			name: "blank user message is ignored",
			messages: []ChatCompletionMessage{
				{Role: "user", Content: "   "},
			},
			wantNil: true,
		},
		{
			name: "valid user message",
			messages: []ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := buildSession(tc.messages)
			if tc.wantNil {
				assert.Nil(t, sess)
				return
			}
			require.NotNil(t, sess)
			assert.True(t, sess.ToolsApproved)
			assert.True(t, sess.NonInteractive)
		})
	}
}

func TestBuildSession_PreservesHistory(t *testing.T) {
	sess := buildSession([]ChatCompletionMessage{
		{Role: "system", Content: "you are a docker agent"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you?"},
	})
	require.NotNil(t, sess)

	// GetAllMessages omits system messages.
	all := sess.GetAllMessages()
	require.Len(t, all, 3)

	roles := make([]chat.MessageRole, len(all))
	for i, m := range all {
		roles[i] = m.Message.Role
	}
	assert.Equal(t, []chat.MessageRole{
		chat.MessageRoleUser,
		chat.MessageRoleAssistant,
		chat.MessageRoleUser,
	}, roles)

	assert.Equal(t, "how are you?", sess.GetLastUserMessageContent())
	assert.Equal(t, "hi there", sess.GetLastAssistantMessageContent())
}

func TestBuildSession_PreservesToolMessage(t *testing.T) {
	sess := buildSession([]ChatCompletionMessage{
		{Role: "user", Content: "compute 2+2"},
		{Role: "assistant", Content: ""}, // dropped: empty content
		{Role: "tool", Content: "4", ToolCallID: "call_1"},
	})
	require.NotNil(t, sess)

	all := sess.GetAllMessages()
	require.Len(t, all, 2)

	last := all[len(all)-1].Message
	assert.Equal(t, chat.MessageRoleTool, last.Role)
	assert.Equal(t, "4", last.Content)
	assert.Equal(t, "call_1", last.ToolCallID)
}

func TestBuildSession_UnknownRoleTreatedAsUser(t *testing.T) {
	sess := buildSession([]ChatCompletionMessage{
		{Role: "developer", Content: "do this"},
	})
	require.NotNil(t, sess)

	all := sess.GetAllMessages()
	require.Len(t, all, 1)
	assert.Equal(t, chat.MessageRoleUser, all[0].Message.Role)
	assert.Equal(t, "do this", all[0].Message.Content)
}

func TestSessionUsage_OmitsZero(t *testing.T) {
	sess := session.New()
	assert.Nil(t, sessionUsage(sess))

	sess.InputTokens = 5
	sess.OutputTokens = 7
	usage := sessionUsage(sess)
	require.NotNil(t, usage)
	assert.Equal(t, int64(5), usage.PromptTokens)
	assert.Equal(t, int64(7), usage.CompletionTokens)
	assert.Equal(t, int64(12), usage.TotalTokens)
}

func TestAgentPolicy_Pick(t *testing.T) {
	p := agentPolicy{exposed: []string{"root", "reviewer"}, fallback: "root"}

	assert.Equal(t, "reviewer", p.pick("reviewer"))
	assert.Equal(t, "root", p.pick("root"))
	assert.Equal(t, "root", p.pick(""), "empty model falls back")
	assert.Equal(t, "root", p.pick("gpt-4"), "unknown model falls back")
}

func TestErrTypeFor(t *testing.T) {
	assert.Equal(t, "invalid_request_error", errTypeFor(400))
	assert.Equal(t, "invalid_request_error", errTypeFor(404))
	assert.Equal(t, "internal_error", errTypeFor(500))
	assert.Equal(t, "internal_error", errTypeFor(502))
}

func TestNewRouter_CORSDisabledByDefault(t *testing.T) {
	srv, _ := newTestServer("root")
	r := newRouter(srv, Options{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/v1/models", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"no CORS header should be emitted when no origin is configured")
}

func TestNewRouter_CORSAllowsConfiguredOrigin(t *testing.T) {
	srv, _ := newTestServer("root")
	r := newRouter(srv, Options{CORSOrigin: "https://example.com"})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/v1/models", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestBearerAuthMiddleware(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"correct token", "Bearer secret", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newTestServer("root")
			r := newRouter(srv, Options{APIKey: "secret"})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/models", http.NoBody)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func TestBearerAuthMiddleware_AllowsCORSPreflight(t *testing.T) {
	// CORS preflight must succeed without an Authorization header.
	srv, _ := newTestServer("root")
	r := newRouter(srv, Options{APIKey: "secret", CORSOrigin: "https://example.com"})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/v1/models", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestNewRouter_RejectsOversizedBody(t *testing.T) {
	srv, _ := newTestServer("root")
	r := newRouter(srv, Options{MaxRequestBytes: 16})

	body := strings.NewReader(`{"messages":[{"role":"user","content":"this body is far longer than sixteen bytes"}]}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestValidateSamplingParams(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	i := func(v int64) *int64 { return &v }

	cases := []struct {
		name    string
		req     ChatCompletionRequest
		wantErr string
	}{
		{name: "all empty"},
		{name: "valid", req: ChatCompletionRequest{
			Temperature: f(0.7), TopP: f(0.95), MaxTokens: i(256),
			Stop: StopSequences{"\n\n", "END"},
		}},
		{name: "temp negative", req: ChatCompletionRequest{Temperature: f(-0.1)}, wantErr: "temperature"},
		{name: "temp too high", req: ChatCompletionRequest{Temperature: f(2.5)}, wantErr: "temperature"},
		{name: "topp zero", req: ChatCompletionRequest{TopP: f(0)}, wantErr: "top_p"},
		{name: "topp too high", req: ChatCompletionRequest{TopP: f(1.5)}, wantErr: "top_p"},
		{name: "max_tokens zero", req: ChatCompletionRequest{MaxTokens: i(0)}, wantErr: "max_tokens"},
		{name: "empty stop", req: ChatCompletionRequest{Stop: StopSequences{""}}, wantErr: "stop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSamplingParams(&tc.req)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestStopSequences_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []string
		err  bool
	}{
		{"null", `null`, nil, false},
		{"single string", `"END"`, []string{"END"}, false},
		{"array", `["a", "b"]`, []string{"a", "b"}, false},
		{"empty array", `[]`, []string{}, false},
		{"number invalid", `42`, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got StopSequences
			err := got.UnmarshalJSON([]byte(tc.json))
			if tc.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, []string(got))
		})
	}
}

func TestSSEStream_SendError(t *testing.T) {
	rec := httptest.NewRecorder()
	s := newSSEStream(rec, "chatcmpl-x", "root")
	s.sendError(errors.New("model exploded"))
	s.send(ChatCompletionStreamDelta{}, "error")
	s.done()

	body := rec.Body.String()
	// One error envelope.
	assert.Contains(t, body, `"error":{"message":"model exploded"`)
	// One terminating chunk with finish_reason=error (instead of stop).
	assert.Contains(t, body, `"finish_reason":"error"`)
	// And the OpenAI sentinel.
	assert.Contains(t, body, "data: [DONE]")
}

func TestRequestTimeoutMiddleware_AppliesDeadline(t *testing.T) {
	e := echo.New()
	e.Use(requestTimeoutMiddleware(5 * time.Millisecond))

	var gotErr error
	e.GET("/sleep", func(c echo.Context) error {
		select {
		case <-c.Request().Context().Done():
			gotErr = c.Request().Context().Err()
			return c.String(http.StatusOK, "ok")
		case <-time.After(time.Second):
			return c.String(http.StatusOK, "too slow")
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sleep", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Error(t, gotErr)
	assert.ErrorIs(t, gotErr, context.DeadlineExceeded)
}
