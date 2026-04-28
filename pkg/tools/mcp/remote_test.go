package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoteClientCustomHeaders verifies that custom headers passed to the remote
// MCP client are actually applied to HTTP requests sent to the MCP server.
func TestRemoteClientCustomHeaders(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	requestCaptured := make(chan bool, 1)

	// Create a test SSE server that captures the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r

		// Send a minimal SSE response to satisfy the client
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event: endpoint\ndata: {\"uri\":\"/message\"}\n\n")

		select {
		case requestCaptured <- true:
		default:
		}
	}))
	defer server.Close()

	// Create remote client WITH custom headers
	expectedHeaders := map[string]string{
		"X-Test-Header": "test-value",
		"X-API-Key":     "secret-key-12345",
		"Authorization": "Bearer custom-token",
	}

	client := newRemoteClient(server.URL, "sse", expectedHeaders, NewInMemoryTokenStore(), nil)

	// Try to initialize (which will make the HTTP request)
	// We don't care if it succeeds or fails, we just need it to make the request
	_, _ = client.Initialize(t.Context(), nil)

	// Wait for the request to be captured
	select {
	case <-requestCaptured:
		// Verify that custom headers were applied
		for key, expectedValue := range expectedHeaders {
			actualValue := capturedRequest.Header.Get(key)
			assert.Equal(t, expectedValue, actualValue,
				"Expected header %s to have value %q, but got %q",
				key, expectedValue, actualValue)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Server did not receive request within timeout")
	}
}

// TestRemoteClientHeadersWithStreamable verifies that custom headers work with streamable transport
func TestRemoteClientHeadersWithStreamable(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	requestCaptured := make(chan bool, 1)

	// Create a test server for streamable transport
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r

		// Send a minimal response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"protocolVersion":"1.0.0","capabilities":{},"serverInfo":{"name":"test","version":"1.0.0"}},"id":1}`)

		select {
		case requestCaptured <- true:
		default:
		}
	}))
	defer server.Close()

	// Create remote client WITH custom headers using streamable transport
	expectedHeaders := map[string]string{
		"X-Custom-Auth": "custom-auth-value",
	}

	client := newRemoteClient(server.URL, "streamable", expectedHeaders, NewInMemoryTokenStore(), nil)

	// Try to initialize
	_, _ = client.Initialize(t.Context(), nil)

	// Wait for the request to be captured
	select {
	case <-requestCaptured:
		// Verify that custom headers were applied
		actualValue := capturedRequest.Header.Get("X-Custom-Auth")
		assert.Equal(t, "custom-auth-value", actualValue,
			"Expected header X-Custom-Auth to have value %q, but got %q",
			"custom-auth-value", actualValue)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Server did not receive request within timeout")
	}
}

// TestRemoteClientNoHeaders verifies that the client works correctly even with no headers
func TestRemoteClientNoHeaders(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	requestCaptured := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event: endpoint\ndata: {\"uri\":\"/message\"}\n\n")

		select {
		case requestCaptured <- true:
		default:
		}
	}))
	defer server.Close()

	// Create remote client without custom headers (nil)
	client := newRemoteClient(server.URL, "sse", nil, NewInMemoryTokenStore(), nil)

	_, _ = client.Initialize(t.Context(), nil)

	// Wait for request
	select {
	case <-requestCaptured:
		// Just verify we got the request - no custom headers should be present
		require.NotNil(t, capturedRequest, "Request should have been captured")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Server did not receive request within timeout")
	}
}

// TestRemoteClientEmptyHeaders verifies that the client works correctly with an empty map
func TestRemoteClientEmptyHeaders(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	requestCaptured := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event: endpoint\ndata: {\"uri\":\"/message\"}\n\n")

		select {
		case requestCaptured <- true:
		default:
		}
	}))
	defer server.Close()

	// Create remote client with empty headers map
	client := newRemoteClient(server.URL, "sse", map[string]string{}, NewInMemoryTokenStore(), nil)

	_, _ = client.Initialize(t.Context(), nil)

	// Wait for request
	select {
	case <-requestCaptured:
		// Just verify we got the request
		require.NotNil(t, capturedRequest, "Request should have been captured")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Server did not receive request within timeout")
	}
}

// TestInitialize_SurfacesServerErrorInReturnedError verifies that when an
// MCP server rejects the initialize call with a 4xx carrying a JSON-RPC
// error body, the error returned by Initialize contains the server's own
// explanation — not just the generic "Bad Request" from http.StatusText.
//
// Regression test for: Slack's MCP endpoint answering
//
//	400 Bad Request
//	{"jsonrpc":"2.0","id":null,"error":{"code":-32600,
//	 "message":"App is not enabled for Slack MCP server access. ..."}}
//
// where the bubbled-up error previously read only "...: Bad Request" and
// the user had no way to learn what was actually wrong.
func TestInitialize_SurfacesServerErrorInReturnedError(t *testing.T) {
	t.Parallel()

	const msg = "App is not enabled for Slack MCP server access."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":%q}}`, msg)
	}))
	defer server.Close()

	// Pre-populate a token so the transport doesn't try to trigger OAuth on
	// the 401 path — we want to exercise the "server rejected us with a
	// non-auth failure" code path.
	store := NewInMemoryTokenStore()
	require.NoError(t, store.StoreToken(server.URL, &OAuthToken{AccessToken: "at", TokenType: "Bearer"}))

	client := newRemoteClient(server.URL, "streamable", nil, store, nil)

	_, err := client.Initialize(t.Context(), nil)
	require.Error(t, err, "Initialize should fail against a server that rejects initialize")
	assert.Contains(t, err.Error(), msg,
		"Initialize error must surface the server's JSON-RPC error message (%q), got: %v", msg, err)
	assert.Contains(t, err.Error(), "400",
		"Initialize error should include the HTTP status code so the user knows it was a server rejection, got: %v", err)
}

// TestInitialize_NonInteractiveCtxDefersOAuthAndDoesNotBlock verifies that
// when Initialize runs against a server that requires OAuth (responds with
// 401 + WWW-Authenticate) under a context flagged with
// WithoutInteractivePrompts, the call:
//
//   - returns promptly,
//   - returns an error that satisfies IsAuthorizationRequired,
//   - never opens a callback HTTP server (i.e. doesn't try to bind a port).
//
// Regression test for: "docker agent run ./examples/slack.yaml" hanging
// during startup. The TUI was not yet ready to render the OAuth dialog,
// the elicitation goroutine was blocked on a synchronous channel send,
// and Ctrl-C couldn't reach it.
func TestInitialize_NonInteractiveCtxDefersOAuthAndDoesNotBlock(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource="https://example.test/.well-known/oauth-protected-resource"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newRemoteClient(server.URL, "streamable", nil, NewInMemoryTokenStore(), nil)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	nonInteractiveCtx := WithoutInteractivePrompts(ctx)

	done := make(chan error, 1)
	go func() {
		_, err := client.Initialize(nonInteractiveCtx, nil)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err, "Initialize should fail with a deferred-auth error in non-interactive ctx")
		assert.True(t, IsAuthorizationRequired(err),
			"non-interactive Initialize should return IsAuthorizationRequired, got: %v", err)
	case <-ctx.Done():
		t.Fatalf("Initialize blocked for too long; non-interactive ctx must short-circuit OAuth: %v", ctx.Err())
	}
}
