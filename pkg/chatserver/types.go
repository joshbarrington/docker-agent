package chatserver

import (
	"encoding/json"
	"errors"

	"github.com/openai/openai-go/v3"
)

// This file declares the OpenAI-compatible request/response types used by
// /v1/chat/completions and /v1/models. We hand-roll most of them instead of
// borrowing from github.com/openai/openai-go/v3 because the SDK's response
// structs are deserialised through its internal `apijson` package and don't
// have `omitempty` JSON tags; marshalling them with stdlib `encoding/json`
// produces noisy responses full of empty audio/tool_call/refusal
// placeholders. `openai.Model` round-trips cleanly with stdlib json, so
// /v1/models reuses it.

// --- Request --------------------------------------------------------------

// ChatCompletionRequest is the body of a /v1/chat/completions call. We
// declare every field commonly sent by OpenAI clients so they are accepted
// without surprise. Whether each field is *acted on* is documented inline.
type ChatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []ChatCompletionMessage `json:"messages"`
	Stream   bool                    `json:"stream,omitempty"`

	// Temperature is parsed and range-checked but not yet plumbed through
	// to the runtime/model layer (no per-request override exists today).
	// Set on the agent's YAML configuration to control sampling.
	Temperature *float64 `json:"temperature,omitempty"`
	// TopP is parsed and range-checked but not yet plumbed through.
	TopP *float64 `json:"top_p,omitempty"`
	// MaxTokens is the maximum number of tokens the model may generate in
	// the response. Parsed and validated; runtime plumbing is tracked for
	// a follow-up.
	MaxTokens *int64 `json:"max_tokens,omitempty"`
	// Stop is one or more substrings that, if produced, end generation.
	// Accepted as either a single string or an array of strings, matching
	// the OpenAI schema. Validated; not yet enforced.
	Stop StopSequences `json:"stop,omitempty"`
}

// StopSequences is a JSON-flexible field that accepts either a single
// string or an array of strings. OpenAI's API uses both shapes
// interchangeably; clients in the wild send both.
type StopSequences []string

func (s *StopSequences) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*s = nil
		return nil
	}
	switch data[0] {
	case '"':
		var one string
		if err := json.Unmarshal(data, &one); err != nil {
			return err
		}
		*s = []string{one}
		return nil
	case '[':
		var many []string
		if err := json.Unmarshal(data, &many); err != nil {
			return err
		}
		*s = many
		return nil
	default:
		return errors.New("stop must be a string or array of strings")
	}
}

// ChatCompletionMessage is a single message in the conversation. Multi-modal
// content (image parts, audio, etc.) is not supported and falls back to the
// `Content` string.
type ChatCompletionMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// --- Non-streaming response -----------------------------------------------

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// ChatCompletionUsage reports approximate token counts. Best-effort: when
// the underlying provider doesn't report usage we omit the field entirely.
type ChatCompletionUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// --- Streaming response ---------------------------------------------------

// ChatCompletionStreamResponse is one SSE chunk emitted when the client
// requests stream: true.
type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

type ChatCompletionStreamChoice struct {
	Index        int                       `json:"index"`
	Delta        ChatCompletionStreamDelta `json:"delta"`
	FinishReason string                    `json:"finish_reason,omitempty"`
}

type ChatCompletionStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// --- Models endpoint ------------------------------------------------------

// ModelsResponse is the body returned by /v1/models. Each agent in the team
// is exposed as one entry.
type ModelsResponse struct {
	Object string         `json:"object"`
	Data   []openai.Model `json:"data"`
}

// --- Errors ---------------------------------------------------------------

// ErrorResponse is the OpenAI-style error envelope returned on 4xx/5xx.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
