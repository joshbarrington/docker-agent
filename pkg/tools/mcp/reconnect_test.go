package mcp

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/tools"
)

// listenTCPRetry retries Listen on addr until it succeeds or a short
// deadline passes (the port may be momentarily busy after a previous
// server shutdown). The returned listener should be closed by the caller.
func listenTCPRetry(t *testing.T, addr string) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	var ln net.Listener
	require.Eventually(t, func() bool {
		var err error
		ln, err = lc.Listen(t.Context(), "tcp", addr)
		return err == nil
	}, 2*time.Second, 50*time.Millisecond, "port %s not available in time", addr)
	return ln
}

// startMCPServer creates a minimal MCP server on addr with the given tools
// and returns a function to shut it down.
func startMCPServer(t *testing.T, addr string, mcpTools ...*gomcp.Tool) (shutdown func()) {
	t.Helper()

	s := gomcp.NewServer(&gomcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	for _, tool := range mcpTools {
		s.AddTool(tool, func(_ context.Context, _ *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "ok-" + tool.Name}},
			}, nil
		})
	}

	srvLn := listenTCPRetry(t, addr)
	srv := &http.Server{
		Handler: gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return s }, nil),
	}
	go func() { _ = srv.Serve(srvLn) }()

	return func() { _ = srv.Close() }
}

// allocateAddr returns a free TCP address on localhost.
func allocateAddr(t *testing.T) string {
	t.Helper()
	ln := listenTCPRetry(t, "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// TestRemoteReconnectAfterServerRestart verifies that a Toolset backed by a
// real remote (streamable-HTTP) MCP server transparently recovers when the
// server is restarted.
//
// The scenario:
//  1. Start a minimal MCP server with a "ping" tool.
//  2. Connect a Toolset, call "ping" — succeeds.
//  3. Shut down the server (simulates crash / restart).
//  4. Start a **new** server on the same address.
//  5. Call "ping" again — this must succeed after automatic reconnection.
//
// Without the ErrSessionMissing recovery logic the second call would fail
// because the new server does not know the old session ID.
func TestRemoteReconnectAfterServerRestart(t *testing.T) {
	t.Parallel()

	addr := allocateAddr(t)

	var callCount atomic.Int32

	// startServer creates a minimal MCP server on addr with a "ping" tool
	// and returns a function to shut it down.
	startServer := func(t *testing.T) (shutdown func()) {
		t.Helper()

		s := gomcp.NewServer(&gomcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
		s.AddTool(&gomcp.Tool{
			Name:        "ping",
			InputSchema: &jsonschema.Schema{Type: "object"},
		}, func(_ context.Context, _ *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			n := callCount.Add(1)
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: fmt.Sprintf("pong-%d", n)}},
			}, nil
		})

		// Retry Listen until the port is available (e.g. after a server shutdown).
		srvLn := listenTCPRetry(t, addr)

		srv := &http.Server{
			Handler: gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return s }, nil),
		}
		go func() { _ = srv.Serve(srvLn) }()

		return func() { _ = srv.Close() }
	}

	callPing := func(t *testing.T, ts *Toolset) string {
		t.Helper()
		result, callErr := ts.callTool(t.Context(), tools.ToolCall{
			Function: tools.FunctionCall{Name: "ping", Arguments: "{}"},
		})
		require.NoError(t, callErr)
		return result.Output
	}

	// --- Step 1–2: Start first server, connect toolset ---
	shutdown1 := startServer(t)

	ts := NewRemoteToolset("test", fmt.Sprintf("http://%s/mcp", addr), "streamable-http", nil, nil)
	require.NoError(t, ts.Start(t.Context()))

	toolList, err := ts.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, toolList, 1)
	assert.Equal(t, "test_ping", toolList[0].Name)

	// --- Step 3: Call succeeds on original server ---
	assert.Equal(t, "pong-1", callPing(t, ts))

	// --- Step 4: Shut down the server ---
	shutdown1()

	// Capture the current restarted channel before the reconnect
	ts.mu.Lock()
	restartedCh := ts.restarted
	ts.mu.Unlock()

	// --- Step 5–6: Start a fresh server, call again ---
	shutdown2 := startServer(t)
	t.Cleanup(func() {
		_ = ts.Stop(t.Context())
		shutdown2()
	})

	// This call triggers ErrSessionMissing recovery and must succeed transparently.
	assert.Equal(t, "pong-2", callPing(t, ts))

	// Verify that watchConnection actually restarted the connection by checking
	// that the restarted channel was closed (signaling reconnect completion).
	select {
	case <-restartedCh:
		// Success: the channel was closed, meaning reconnect happened
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reconnect did not complete: restarted channel was not closed")
	}
}

// TestRemoteReconnectRefreshesTools verifies that after a remote MCP server
// restarts with a different set of tools, the Toolset picks up the new tools
// and notifies the runtime via the toolsChangedHandler.
//
// This is the scenario from https://github.com/docker/docker-agent/issues/2244:
//   - Server v1 exposes tools [alpha, shared].
//   - Client connects and caches [alpha, shared].
//   - Server v1 shuts down; server v2 starts with tools [beta, shared].
//   - A tool call to "shared" triggers reconnection.
//   - After reconnection, Tools() must return [beta, shared], not the stale [alpha, shared].
//   - The toolsChangedHandler must be called so the runtime refreshes its own state.
func TestRemoteReconnectRefreshesTools(t *testing.T) {
	t.Parallel()

	addr := allocateAddr(t)

	// "shared" exists on both servers so we can call it to trigger reconnect.
	sharedTool := &gomcp.Tool{Name: "shared", InputSchema: &jsonschema.Schema{Type: "object"}}
	alphaTool := &gomcp.Tool{Name: "alpha", InputSchema: &jsonschema.Schema{Type: "object"}}
	betaTool := &gomcp.Tool{Name: "beta", InputSchema: &jsonschema.Schema{Type: "object"}}

	// --- Start server v1 with tools "alpha" + "shared" ---
	shutdown1 := startMCPServer(t, addr, alphaTool, sharedTool)

	ts := NewRemoteToolset("ns", fmt.Sprintf("http://%s/mcp", addr), "streamable-http", nil, nil)

	// Track toolsChangedHandler invocations.
	toolsChangedCh := make(chan struct{}, 1)
	ts.SetToolsChangedHandler(func() {
		select {
		case toolsChangedCh <- struct{}{}:
		default:
		}
	})

	require.NoError(t, ts.Start(t.Context()))

	// Verify initial tools.
	toolList, err := ts.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, toolList, 2)
	toolNames := []string{toolList[0].Name, toolList[1].Name}
	assert.Contains(t, toolNames, "ns_alpha")
	assert.Contains(t, toolNames, "ns_shared")

	// --- Shut down server v1, start server v2 with tools "beta" + "shared" ---
	shutdown1()

	shutdown2 := startMCPServer(t, addr, betaTool, sharedTool)
	t.Cleanup(func() {
		_ = ts.Stop(t.Context())
		shutdown2()
	})

	// Call "shared" to trigger ErrSessionMissing → reconnect.
	result, callErr := ts.callTool(t.Context(), tools.ToolCall{
		Function: tools.FunctionCall{Name: "shared", Arguments: "{}"},
	})
	require.NoError(t, callErr)
	assert.Equal(t, "ok-shared", result.Output)

	// Wait for the toolsChangedHandler to be called (signals reconnect + refresh).
	select {
	case <-toolsChangedCh:
		// Good — the handler was called.
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for toolsChangedHandler after reconnect")
	}

	// Verify the toolset now reports the new server's tools.
	toolList, err = ts.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, toolList, 2, "expected exactly two tools from the new server")
	toolNames = []string{toolList[0].Name, toolList[1].Name}
	assert.Contains(t, toolNames, "ns_beta", "expected the new server's tool, got stale tool")
	assert.Contains(t, toolNames, "ns_shared")
	assert.NotContains(t, toolNames, "ns_alpha", "stale tool from old server should not be present")
}

// failingInitClient is a mock mcpClient whose Initialize method returns a
// configurable error for the first N calls, then succeeds.
type failingInitClient struct {
	mu          sync.Mutex
	initErr     error // error to return from Initialize
	failsLeft   int   // how many more times Initialize should fail
	initCalls   int   // total Initialize calls
	waitCh      chan struct{}
	toolsToList []*gomcp.Tool
}

func (m *failingInitClient) Initialize(_ context.Context, _ *gomcp.InitializeRequest) (*gomcp.InitializeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCalls++
	if m.failsLeft > 0 {
		m.failsLeft--
		return nil, m.initErr
	}
	if m.waitCh != nil {
		m.waitCh = make(chan struct{})
	}
	return &gomcp.InitializeResult{}, nil
}

func (m *failingInitClient) ListTools(_ context.Context, _ *gomcp.ListToolsParams) iter.Seq2[*gomcp.Tool, error] {
	m.mu.Lock()
	t := m.toolsToList
	m.mu.Unlock()
	return func(yield func(*gomcp.Tool, error) bool) {
		for _, tool := range t {
			if !yield(tool, nil) {
				return
			}
		}
	}
}

func (m *failingInitClient) CallTool(context.Context, *gomcp.CallToolParams) (*gomcp.CallToolResult, error) {
	return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: "ok"}}}, nil
}

func (m *failingInitClient) ListPrompts(context.Context, *gomcp.ListPromptsParams) iter.Seq2[*gomcp.Prompt, error] {
	return func(func(*gomcp.Prompt, error) bool) {}
}

func (m *failingInitClient) GetPrompt(context.Context, *gomcp.GetPromptParams) (*gomcp.GetPromptResult, error) {
	return &gomcp.GetPromptResult{}, nil
}

func (m *failingInitClient) SetElicitationHandler(tools.ElicitationHandler) {}
func (m *failingInitClient) SetOAuthSuccessHandler(func())                  {}
func (m *failingInitClient) SetManagedOAuth(bool)                           {}
func (m *failingInitClient) SetToolListChangedHandler(func())               {}
func (m *failingInitClient) SetPromptListChangedHandler(func())             {}

func (m *failingInitClient) Wait() error {
	m.mu.Lock()
	ch := m.waitCh
	m.mu.Unlock()
	if ch == nil {
		select {}
	}
	<-ch
	return nil
}

func (m *failingInitClient) Close(context.Context) error {
	m.mu.Lock()
	if m.waitCh != nil {
		select {
		case <-m.waitCh:
		default:
			close(m.waitCh)
		}
	}
	m.mu.Unlock()
	return nil
}

// TestStdioStartReturnsErrorWhenServerUnavailable verifies that a stdio toolset
// propagates errServerUnavailable when Initialize returns io.EOF, and that
// started remains false so the runtime can retry.
func TestStdioStartReturnsErrorWhenServerUnavailable(t *testing.T) {
	t.Parallel()

	mock := &failingInitClient{
		initErr:   io.EOF,
		failsLeft: 1,
	}

	ts := &Toolset{
		name:      "test-stdio",
		mcpClient: mock,
		logID:     "test-cmd",
	}

	err := ts.Start(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, errServerUnavailable)

	ts.mu.Lock()
	started := ts.started
	ts.mu.Unlock()
	assert.False(t, started, "stdio toolset must not be marked as started when server is unavailable")
}

// TestStdioStartReturnsErrorWhenBinaryNotFound verifies that exec.ErrNotFound
// from Initialize is treated the same as io.EOF for stdio toolsets.
func TestStdioStartReturnsErrorWhenBinaryNotFound(t *testing.T) {
	t.Parallel()

	mock := &failingInitClient{
		initErr:   fmt.Errorf("start command: %w", exec.ErrNotFound),
		failsLeft: 1,
	}

	ts := &Toolset{
		name:      "test-stdio",
		mcpClient: mock,
		logID:     "missing-binary",
	}

	err := ts.Start(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, errServerUnavailable)

	ts.mu.Lock()
	started := ts.started
	ts.mu.Unlock()
	assert.False(t, started, "stdio toolset must not be marked as started when binary is not found")
}

// TestStdioLazyRetrySucceedsWhenBinaryAppears verifies the end-to-end retry
// scenario: turn 1 fails with EOF (binary not yet available), turn 2 succeeds
// once the binary "appears" (mock stops failing).
func TestStdioLazyRetrySucceedsWhenBinaryAppears(t *testing.T) {
	t.Parallel()

	pingTool := &gomcp.Tool{Name: "ping"}
	mock := &failingInitClient{
		initErr:     io.EOF,
		failsLeft:   1,
		toolsToList: []*gomcp.Tool{pingTool},
		waitCh:      make(chan struct{}),
	}

	ts := &Toolset{
		name:      "test-stdio",
		mcpClient: mock,
		logID:     "lazy-binary",
	}

	// Turn 1: Start fails — binary not available yet.
	err := ts.Start(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, errServerUnavailable)

	// Turn 2: Binary has "appeared" (mock will succeed).
	err = ts.Start(t.Context())
	require.NoError(t, err)

	ts.mu.Lock()
	started := ts.started
	ts.mu.Unlock()
	assert.True(t, started, "stdio toolset must be started after successful retry")

	toolList, err := ts.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, toolList, 1)
	assert.Equal(t, "test-stdio_ping", toolList[0].Name)

	_ = ts.Stop(t.Context())
}

// TestRemoteStartRetriesWhenUnavailable verifies that a remote toolset also
// returns an error and stays un-started when the server is unavailable (EOF),
// confirming retry-on-next-turn applies to all toolset types.
func TestRemoteStartRetriesWhenUnavailable(t *testing.T) {
	t.Parallel()

	mock := &failingInitClient{
		initErr:   io.EOF,
		failsLeft: 1,
	}

	ts := &Toolset{
		name:      "test-remote",
		mcpClient: mock,
		logID:     "remote-server",
	}

	err := ts.Start(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, errServerUnavailable)

	ts.mu.Lock()
	started := ts.started
	ts.mu.Unlock()
	assert.False(t, started, "remote toolset must not be marked as started when server is unavailable")
}

// TestStartableToolSetRetryAcrossTurns is a full integration test using
// tools.NewStartable to wrap an MCP Toolset. It verifies that when a stdio
// toolset fails N turns, the StartableToolSet keeps retrying and succeeds
// on turn N+1.
func TestStartableToolSetRetryAcrossTurns(t *testing.T) {
	t.Parallel()

	const failTurns = 3

	pingTool := &gomcp.Tool{Name: "ping"}
	mock := &failingInitClient{
		initErr:     fmt.Errorf("command not found: %w", os.ErrNotExist),
		failsLeft:   failTurns,
		toolsToList: []*gomcp.Tool{pingTool},
		waitCh:      make(chan struct{}),
	}

	mcpToolset := &Toolset{
		name:      "retry-test",
		mcpClient: mock,
		logID:     "retry-binary",
	}

	startable := tools.NewStartable(mcpToolset)

	// Turns 1..N: Start fails, IsStarted stays false.
	for turn := 1; turn <= failTurns; turn++ {
		err := startable.Start(t.Context())
		require.Error(t, err, "turn %d should fail", turn)
		assert.False(t, startable.IsStarted(), "turn %d: should not be started", turn)
	}

	// Turn N+1: binary is now available, Start succeeds.
	err := startable.Start(t.Context())
	require.NoError(t, err)
	assert.True(t, startable.IsStarted())

	toolList, err := mcpToolset.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, toolList, 1)
	assert.Equal(t, "retry-test_ping", toolList[0].Name)

	_ = startable.Stop(t.Context())
}
