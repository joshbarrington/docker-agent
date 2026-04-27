package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       []Opt
		wantHeader string
		wantValue  string
	}{
		{
			name:       "WithModel sets X-Cagent-Model",
			opts:       []Opt{WithModel("gpt-4o")},
			wantHeader: "X-Cagent-Model",
			wantValue:  "gpt-4o",
		},
		{
			name:       "WithModelName sets X-Cagent-Model-Name",
			opts:       []Opt{WithModelName("my-fast-model")},
			wantHeader: "X-Cagent-Model-Name",
			wantValue:  "my-fast-model",
		},
		{
			name:       "WithModelName skips header when empty",
			opts:       []Opt{WithModelName("")},
			wantHeader: "X-Cagent-Model-Name",
			wantValue:  "",
		},
		{
			name:       "WithProvider sets X-Cagent-Provider",
			opts:       []Opt{WithProvider("openai")},
			wantHeader: "X-Cagent-Provider",
			wantValue:  "openai",
		},
		{
			name:       "compression is disabled to support SSE streaming",
			wantHeader: "Accept-Encoding",
			wantValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headers := doRequest(t, tt.opts...)

			if tt.wantValue != "" {
				assert.Equal(t, tt.wantValue, headers.Get(tt.wantHeader))
			} else {
				assert.Empty(t, headers.Get(tt.wantHeader))
			}
		})
	}
}

// doRequest creates an HTTP client with the given options, sends a GET request
// to a test server, and returns the headers the server received.
func doRequest(t *testing.T, opts ...Opt) http.Header {
	t.Helper()

	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
	}))
	defer srv.Close()

	client := NewHTTPClient(t.Context(), opts...)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	return capturedHeaders
}
