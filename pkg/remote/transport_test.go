package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/desktop"
)

func TestNewTransport_UsesDesktopProxyWhenAvailable(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Create a transport
	transport := NewTransport(ctx)
	require.NotNil(t, transport)

	// Verify that it's an http.Transport
	httpTransport, ok := transport.(*http.Transport)
	require.True(t, ok, "transport should be *http.Transport")

	// If Docker Desktop is running, verify proxy is configured
	if desktop.IsDockerDesktopRunning(ctx) {
		assert.NotNil(t, httpTransport.Proxy, "proxy should be configured when Docker Desktop is running")
		assert.NotNil(t, httpTransport.DialContext, "custom DialContext should be set when Docker Desktop is running")
	}
}

func TestNewTransport_WorksWithoutDesktopProxy(t *testing.T) {
	t.Parallel()

	// Create a test server to simulate a registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := t.Context()

	// Create a transport (should work whether Desktop is running or not)
	transport := NewTransport(ctx)
	require.NotNil(t, transport)

	// Make a simple HTTP request to verify the transport works
	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
