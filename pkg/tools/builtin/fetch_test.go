package builtin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/tools"
)

func TestFetchToolWithOptions(t *testing.T) {
	customTimeout := 60 * time.Second

	tool := NewFetchTool(WithTimeout(customTimeout))

	assert.Equal(t, customTimeout, tool.handler.timeout)
}

func TestFetchTool_Tools(t *testing.T) {
	tool := NewFetchTool()

	allTools, err := tool.Tools(t.Context())
	require.NoError(t, err)
	require.Len(t, allTools, 1)
	for _, tool := range allTools {
		assert.NotNil(t, tool.Handler)
		assert.Equal(t, "fetch", tool.Category)
	}

	fetchTool := allTools[0]
	assert.Equal(t, "fetch", fetchTool.Name)

	schema, err := json.Marshal(fetchTool.Parameters)
	require.NoError(t, err)
	assert.JSONEq(t, `{
	"type": "object",
	"properties": {
		"format": {
			"description": "The format to return the content in (text, markdown, or html)",
			"enum": [
				"text",
				"markdown",
				"html"
			],
			"type": "string"
		},
		"timeout": {
			"description": "Request timeout in seconds (default: 30)",
			"maximum": 300,
			"minimum": 1,
			"type": "integer"
		},
		"urls": {
			"description": "Array of URLs to fetch",
			"items": {
				"type": "string"
			},
			"minItems": 1,
			"type": "array"
		}
	},
	"required": [
		"urls",
		"format"
	]
}`, string(schema))
}

func TestFetchTool_Instructions(t *testing.T) {
	tool := NewFetchTool()

	instructions := tools.GetInstructions(tool)

	assert.Contains(t, instructions, "Fetch Tool")
}

func TestFetchTool_StartStop(t *testing.T) {
	// FetchTool doesn't need to implement Startable -
	// it has no initialization or cleanup requirements
	tool := NewFetchTool()

	// Verify it implements ToolSet but not necessarily Startable
	_, ok := any(tool).(tools.Startable)
	assert.False(t, ok, "FetchTool should not implement Startable")
}

func TestFetch_Call_Success(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Hello, World!")
	})

	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{URLs: []string{url}})
	require.NoError(t, err)

	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "Status: 200")
	assert.Contains(t, result.Output, "Length: 13 bytes")
	assert.Contains(t, result.Output, "Hello, World!")
}

func TestFetch_Call_MultipleURLs(t *testing.T) {
	url1 := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "Server 1")
	})
	url2 := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "Server 2")
	})

	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{URLs: []string{url1, url2}})
	require.NoError(t, err)

	var results []FetchResult
	err = json.Unmarshal([]byte(result.Output), &results)
	require.NoError(t, err)

	require.Len(t, results, 2)
	assert.Equal(t, "Server 1", results[0].Body)
	assert.Equal(t, "Server 2", results[1].Body)
}

func TestFetch_Call_InvalidURL(t *testing.T) {
	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs: []string{
			"invalid-url",
		},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
}

func TestFetch_Call_UnsupportedProtocol(t *testing.T) {
	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs: []string{
			"ftp://example.com",
		},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "only HTTP and HTTPS URLs are supported")
}

func TestFetch_Call_NoURLs(t *testing.T) {
	tool := NewFetchTool()

	_, err := tool.handler.CallTool(t.Context(), FetchToolArgs{})
	require.ErrorContains(t, err, "at least one URL is required")
}

func TestFetch_Markdown(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<h1>Hello docker agent</h1>")
	})

	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url},
		Format: "markdown",
	})
	require.NoError(t, err)

	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "Status: 200")
	assert.Contains(t, result.Output, "Length: 20 bytes")
	assert.Contains(t, result.Output, "# Hello docker agent")
}

func TestFetch_Text(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<h1>Hello docker agent</h1>")
	})

	tool := NewFetchTool()

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url},
		Format: "text",
	})
	require.NoError(t, err)

	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "Status: 200")
	assert.Contains(t, result.Output, "Length: 18 bytes")
	assert.Contains(t, result.Output, "Hello docker agent")
}

func runHTTPServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server.URL
}

func TestFetch_RobotsAllowed(t *testing.T) {
	robotsContent := "User-agent: *\nAllow: /"

	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, robotsContent)
			return
		}
		if r.URL.Path == "/allowed" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "Content allowed by robots")
			return
		}
		http.NotFound(w, r)
	})

	tool := NewFetchTool()
	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/allowed"},
		Format: "text",
	})

	require.NoError(t, err)
	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "Content allowed by robots")
}

func TestFetch_RobotsBlocked(t *testing.T) {
	robotsContent := "User-agent: *\nDisallow: /blocked"

	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, robotsContent)
			return
		}
		if r.URL.Path == "/blocked" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "This should not be fetched")
			return
		}
		http.NotFound(w, r)
	})

	tool := NewFetchTool()
	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/blocked"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "URL blocked by robots.txt")
}

func TestFetch_RobotsMissing(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/content" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "Content without robots.txt")
			return
		}
		http.NotFound(w, r)
	})

	tool := NewFetchTool()
	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/content"},
		Format: "text",
	})

	require.NoError(t, err)
	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "Content without robots.txt")
}

func TestFetch_RobotsCachePerHost_MultipleURLs(t *testing.T) {
	// Regression test: robots.txt should be fetched once per host,
	// but each URL's path must be evaluated individually.
	robotsContent := "User-agent: *\nDisallow: /secret\nAllow: /"

	robotsRequests := 0
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			robotsRequests++
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, robotsContent)
		case "/public":
			fmt.Fprint(w, "public content")
		case "/secret/data":
			fmt.Fprint(w, "secret content")
		default:
			http.NotFound(w, r)
		}
	})

	tool := NewFetchTool()
	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/public", url + "/secret/data"},
		Format: "text",
	})
	require.NoError(t, err)

	var results []FetchResult
	err = json.Unmarshal([]byte(result.Output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First URL should succeed
	assert.Equal(t, 200, results[0].StatusCode)
	assert.Equal(t, "public content", results[0].Body)

	// Second URL should be blocked
	assert.Contains(t, results[1].Error, "URL blocked by robots.txt")

	// robots.txt should have been fetched exactly once
	assert.Equal(t, 1, robotsRequests, "robots.txt should be fetched once per host")
}

func TestFetch_RobotsUnexpectedStatus(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "content")
	})

	tool := NewFetchTool()
	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/page"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "robots.txt check failed")
	assert.Contains(t, result.Output, "unexpected status 500")
}

func TestFetchTool_OutputSchema(t *testing.T) {
	tool := NewFetchTool()

	allTools, err := tool.Tools(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, allTools)

	for _, tool := range allTools {
		assert.NotNil(t, tool.OutputSchema)
	}
}

func TestFetchTool_ParametersAreObjects(t *testing.T) {
	tool := NewFetchTool()

	allTools, err := tool.Tools(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, allTools)

	for _, tool := range allTools {
		m, err := tools.SchemaToMap(tool.Parameters)

		require.NoError(t, err)
		assert.Equal(t, "object", m["type"])
	}
}

func TestFetchTool_WithAllowedDomainsOption(t *testing.T) {
	tool := NewFetchTool(WithAllowedDomains([]string{"example.com"}))

	assert.Equal(t, []string{"example.com"}, tool.handler.allowedDomains)
	assert.Empty(t, tool.handler.blockedDomains)
}

func TestFetchTool_WithBlockedDomainsOption(t *testing.T) {
	tool := NewFetchTool(WithBlockedDomains([]string{"evil.example.com"}))

	assert.Equal(t, []string{"evil.example.com"}, tool.handler.blockedDomains)
	assert.Empty(t, tool.handler.allowedDomains)
}

func TestFetchTool_AllowedDomainsAppearInInstructions(t *testing.T) {
	tool := NewFetchTool(WithAllowedDomains([]string{"docker.com", "github.com"}))

	instructions := tools.GetInstructions(tool)

	assert.Contains(t, instructions, "restricted to these domains")
	assert.Contains(t, instructions, "docker.com")
	assert.Contains(t, instructions, "github.com")
}

func TestFetchTool_BlockedDomainsAppearInInstructions(t *testing.T) {
	tool := NewFetchTool(WithBlockedDomains([]string{"169.254.169.254"}))

	instructions := tools.GetInstructions(tool)

	assert.Contains(t, instructions, "must not fetch")
	assert.Contains(t, instructions, "169.254.169.254")
}

func TestMatchesDomain(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		pattern string
		want    bool
	}{
		{"exact match", "example.com", "example.com", true},
		{"case insensitive", "Example.COM", "example.com", true},
		{"subdomain match", "docs.example.com", "example.com", true},
		{"deep subdomain match", "a.b.example.com", "example.com", true},
		{"unrelated suffix does NOT match", "badexample.com", "example.com", false},
		{"different domain", "other.com", "example.com", false},
		{"leading dot pattern excludes apex", "example.com", ".example.com", false},
		{"leading dot pattern allows subdomain", "docs.example.com", ".example.com", true},
		{"empty host", "", "example.com", false},
		{"empty pattern", "example.com", "", false},
		{"only-dot pattern matches nothing", "example.com", ".", false},
		{"whitespace tolerated", " example.com ", " example.com ", true},
		{"ip address exact", "169.254.169.254", "169.254.169.254", true},
		// FQDN trailing dot (regression: must not bypass the matcher).
		{"trailing dot host matches apex pattern", "example.com.", "example.com", true},
		{"trailing dot host matches subdomain pattern", "docs.example.com.", "example.com", true},
		{"trailing dot pattern matches apex host", "example.com", "example.com.", true},
		{"trailing dot host matches strict-subdomain pattern", "docs.example.com.", ".example.com", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, matchesDomain(tc.host, tc.pattern))
		})
	}
}

func TestFetch_AllowedDomains_DeniesUnknownHost(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "should not be reached")
	})

	// httptest servers run on 127.0.0.1; an allow-list that does not include
	// it must short-circuit the request.
	tool := NewFetchTool(WithAllowedDomains([]string{"example.com"}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/whatever"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "is not in allowed_domains")
}

func TestFetch_AllowedDomains_PermitsKnownHost(t *testing.T) {
	requests := 0
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "hello")
	})

	tool := NewFetchTool(WithAllowedDomains([]string{"127.0.0.1"}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/page"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Successfully fetched")
	assert.Contains(t, result.Output, "hello")
	assert.Positive(t, requests, "the upstream should have been hit when the host is allow-listed")
}

func TestFetch_BlockedDomains_DeniesMatchingHost(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "should not be reached")
	})

	tool := NewFetchTool(WithBlockedDomains([]string{"127.0.0.1"}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/page"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "is blocked by blocked_domains")
}

func TestFetch_BlockedDomains_DeniesIgnoringRobots(t *testing.T) {
	// The deny check must happen before robots.txt is fetched, so a server
	// that errors on /robots.txt should still produce a clear domain error.
	robotsRequested := false
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			robotsRequested = true
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "should not be reached")
	})

	tool := NewFetchTool(WithBlockedDomains([]string{"127.0.0.1"}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/page"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "is blocked by blocked_domains")
	assert.False(t, robotsRequested, "blocked URLs must not trigger any network call, including robots.txt")
}

// TestFetch_AllowedDomains_RejectsRedirectToBlockedHost is a regression test for an
// SSRF-style bypass: an allow-listed origin returning a redirect to a host
// that is NOT in the allow-list must be rejected before the redirect is
// followed, otherwise the policy is hollow.
func TestFetch_AllowedDomains_RejectsRedirectToBlockedHost(t *testing.T) {
	redirected := false
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		redirected = true
		http.Redirect(w, r, "http://attacker.example.com/secret", http.StatusFound)
	})
	parsed, err := neturl.Parse(url)
	require.NoError(t, err)

	// Allow only the test server's host. The redirect target must be
	// rejected without any network call to attacker.example.com.
	tool := NewFetchTool(WithAllowedDomains([]string{parsed.Hostname()}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/start"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.True(t, redirected, "the test server should have been hit at least once to issue the redirect")
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "attacker.example.com", "the error should mention the rejected redirect target")
	assert.Contains(t, result.Output, "is not in allowed_domains")
}

// TestFetch_BlockedDomains_RejectsRedirectToBlockedHost mirrors the allow-list
// regression test for the deny-list path: a redirect to a deny-listed host
// must not be followed.
func TestFetch_BlockedDomains_RejectsRedirectToBlockedHost(t *testing.T) {
	url := runHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "http://169.254.169.254/metadata", http.StatusFound)
	})

	tool := NewFetchTool(WithBlockedDomains([]string{"169.254.169.254"}))

	result, err := tool.handler.CallTool(t.Context(), FetchToolArgs{
		URLs:   []string{url + "/innocent"},
		Format: "text",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Error fetching")
	assert.Contains(t, result.Output, "is blocked by blocked_domains")
	assert.Contains(t, result.Output, "169.254.169.254")
}
