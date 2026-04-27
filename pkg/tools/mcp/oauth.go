package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/tools"
)

// resourceMetadataFromWWWAuth extracts resource metadata URL from WWW-Authenticate header
var re = regexp.MustCompile(`resource="([^"]+)"`)

// oauth is a simple struct for compatibility with existing code
type oauth struct {
	metadataClient *http.Client
}

// protectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC 8707)
type protectedResourceMetadata struct {
	Resource                          string   `json:"resource"`
	AuthorizationServers              []string `json:"authorization_servers"`
	ResourceName                      string   `json:"resource_name,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported            []string `json:"bearer_methods_supported,omitempty"`
	ResourceSigningAlgValuesSupported []string `json:"resource_signing_alg_values_supported,omitempty"`
}

// AuthorizationServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414)
type AuthorizationServerMetadata struct {
	Issuer                                 string   `json:"issuer"`
	AuthorizationEndpoint                  string   `json:"authorization_endpoint"`
	TokenEndpoint                          string   `json:"token_endpoint"`
	RegistrationEndpoint                   string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                     string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint                  string   `json:"introspection_endpoint,omitempty"`
	JwksURI                                string   `json:"jwks_uri,omitempty"`
	ScopesSupported                        []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported                 []string `json:"response_types_supported"`
	ResponseModesSupported                 []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported                    []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported          []string `json:"code_challenge_methods_supported,omitempty"`
}

func (o *oauth) getAuthorizationServerMetadata(ctx context.Context, authServerURL string) (*AuthorizationServerMetadata, error) {
	// Build well-known metadata URL
	metadataURL := authServerURL
	if !strings.HasSuffix(authServerURL, "/.well-known/oauth-authorization-server") {
		metadataURL = strings.TrimSuffix(authServerURL, "/") + "/.well-known/oauth-authorization-server"
	}

	// Attempt OAuth authorization server discovery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := o.metadataClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Try OpenID Connect discovery as fallback
		openIDURL := strings.Replace(metadataURL, "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration", 1)
		oidcReq, err := http.NewRequestWithContext(ctx, http.MethodGet, openIDURL, http.NoBody)
		if err != nil {
			return nil, err
		}
		oidcReq.Header.Set("Accept", "application/json")

		oidcResp, err := o.metadataClient.Do(oidcReq)
		if err != nil {
			return nil, err
		}
		defer oidcResp.Body.Close()

		if oidcResp.StatusCode != http.StatusOK {
			// Return default metadata if all discovery fails
			return createDefaultMetadata(authServerURL), nil
		}

		var metadata AuthorizationServerMetadata
		if err := json.NewDecoder(oidcResp.Body).Decode(&metadata); err != nil {
			return nil, fmt.Errorf("failed to decode metadata from %s: %w", openIDURL, err)
		}
		return validateAndFillDefaults(&metadata, authServerURL), nil
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, metadataURL)
	}

	var metadata AuthorizationServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata from %s: %w", metadataURL, err)
	}

	return validateAndFillDefaults(&metadata, authServerURL), nil
}

// validateAndFillDefaults validates required fields and fills in defaults
func validateAndFillDefaults(metadata *AuthorizationServerMetadata, authServerURL string) *AuthorizationServerMetadata {
	metadata.Issuer = cmp.Or(metadata.Issuer, authServerURL)
	if len(metadata.ResponseTypesSupported) == 0 {
		metadata.ResponseTypesSupported = []string{"code"}
	}

	if len(metadata.ResponseModesSupported) == 0 {
		metadata.ResponseModesSupported = []string{"query", "fragment"}
	}
	if len(metadata.GrantTypesSupported) == 0 {
		metadata.GrantTypesSupported = []string{"authorization_code", "implicit"}
	}
	if len(metadata.TokenEndpointAuthMethodsSupported) == 0 {
		metadata.TokenEndpointAuthMethodsSupported = []string{"client_secret_basic"}
	}
	if len(metadata.RevocationEndpointAuthMethodsSupported) == 0 {
		metadata.RevocationEndpointAuthMethodsSupported = []string{"client_secret_basic"}
	}

	metadata.AuthorizationEndpoint = cmp.Or(metadata.AuthorizationEndpoint, authServerURL+"/authorize")
	metadata.TokenEndpoint = cmp.Or(metadata.TokenEndpoint, authServerURL+"/token")
	// Do NOT fabricate a registration_endpoint — if the server doesn't
	// advertise one, dynamic client registration is not supported.

	return metadata
}

// createDefaultMetadata creates minimal metadata when discovery fails
func createDefaultMetadata(authServerURL string) *AuthorizationServerMetadata {
	return &AuthorizationServerMetadata{
		Issuer:                                 authServerURL,
		AuthorizationEndpoint:                  authServerURL + "/authorize",
		TokenEndpoint:                          authServerURL + "/token",
		ResponseTypesSupported:                 []string{"code"},
		ResponseModesSupported:                 []string{"query", "fragment"},
		GrantTypesSupported:                    []string{"authorization_code"},
		TokenEndpointAuthMethodsSupported:      []string{"client_secret_basic"},
		RevocationEndpointAuthMethodsSupported: []string{"client_secret_basic"},
		CodeChallengeMethodsSupported:          []string{"S256"},
	}
}

func resourceMetadataFromWWWAuth(wwwAuth string) string {
	matches := re.FindStringSubmatch(wwwAuth)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

// callbackRedirectURLFrom is a nil-safe accessor for the optional
// CallbackRedirectURL field on a RemoteOAuthConfig.
func callbackRedirectURLFrom(c *latest.RemoteOAuthConfig) string {
	if c == nil {
		return ""
	}
	return c.CallbackRedirectURL
}

// oauthTransport wraps an HTTP transport with OAuth support
type oauthTransport struct {
	base http.RoundTripper
	// TODO(rumpl): remove client reference, we need to find a better way to send elicitation requests
	client      *remoteMCPClient
	tokenStore  OAuthTokenStore
	baseURL     string
	managed     bool
	oauthConfig *latest.RemoteOAuthConfig

	// mu protects refreshFailedAt from concurrent access.
	mu sync.Mutex
	// refreshFailedAt tracks the last time a silent token refresh failed,
	// so we avoid retrying on every request.
	refreshFailedAt time.Time
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req, false)
}

func (t *oauthTransport) roundTrip(req *http.Request, isRetry bool) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	reqClone := req.Clone(req.Context())

	// Attach a valid token if available, silently refreshing if expired.
	if token := t.getValidToken(req.Context()); token != nil {
		reqClone.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := t.base.RoundTrip(reqClone)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized && !isRetry {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "" {
			resp.Body.Close()

			authServer := req.URL.Scheme + "://" + req.URL.Host
			if err := t.handleOAuthFlow(req.Context(), authServer, wwwAuth); err != nil {
				return nil, fmt.Errorf("OAuth flow failed: %w", err)
			}

			if len(bodyBytes) > 0 {
				req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
			}

			return t.roundTrip(req, true)
		}
	}

	return resp, nil
}

// getValidToken returns a non-expired token for the server, silently refreshing
// an expired token when a refresh token is available. Returns nil if no usable
// token can be obtained.
func (t *oauthTransport) getValidToken(ctx context.Context) *OAuthToken {
	token, err := t.tokenStore.GetToken(t.baseURL)
	if err != nil {
		return nil
	}

	if !token.IsExpired() {
		return token
	}

	if token.RefreshToken == "" {
		return nil
	}

	// Avoid hammering the token endpoint if a recent refresh already failed.
	const refreshBackoff = 30 * time.Second
	t.mu.Lock()
	failedAt := t.refreshFailedAt
	t.mu.Unlock()
	if !failedAt.IsZero() && time.Since(failedAt) < refreshBackoff {
		return nil
	}

	slog.Debug("Attempting silent token refresh", "url", t.baseURL)

	o := &oauth{metadataClient: &http.Client{Timeout: 5 * time.Second}}
	authServer := cmp.Or(token.AuthServer, t.baseURL)
	metadata, err := o.getAuthorizationServerMetadata(ctx, authServer)
	if err != nil {
		slog.Debug("Failed to fetch auth server metadata for refresh", "auth_server", authServer, "error", err)
		return nil
	}

	newToken, err := RefreshAccessToken(ctx, metadata.TokenEndpoint, token.RefreshToken, token.ClientID, token.ClientSecret)
	if err != nil {
		slog.Debug("Token refresh failed, will require interactive auth", "error", err)
		t.mu.Lock()
		t.refreshFailedAt = time.Now()
		t.mu.Unlock()
		return nil
	}
	newToken.AuthServer = authServer

	t.mu.Lock()
	t.refreshFailedAt = time.Time{} // reset on success
	t.mu.Unlock()

	if err := t.tokenStore.StoreToken(t.baseURL, newToken); err != nil {
		slog.Warn("Failed to store refreshed token", "error", err)
	}

	slog.Debug("Token refreshed successfully", "url", t.baseURL)
	return newToken
}

// handleOAuthFlow performs the OAuth flow when a 401 response is received
func (t *oauthTransport) handleOAuthFlow(ctx context.Context, authServer, wwwAuth string) error {
	if t.managed {
		return t.handleManagedOAuthFlow(ctx, authServer, wwwAuth)
	}

	return t.handleUnmanagedOAuthFlow(ctx, authServer, wwwAuth)
}

func (t *oauthTransport) handleManagedOAuthFlow(ctx context.Context, authServer, wwwAuth string) error {
	slog.Debug("Starting OAuth flow for server", "url", t.baseURL)

	resourceURL := cmp.Or(resourceMetadataFromWWWAuth(wwwAuth), authServer+"/.well-known/oauth-protected-resource")

	resourceReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(resourceReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		_, _ = io.ReadAll(resp.Body)
		return errors.New("failed to fetch protected resource metadata")
	}
	var resourceMetadata protectedResourceMetadata
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&resourceMetadata); err != nil {
			return err
		}
	}

	if len(resourceMetadata.AuthorizationServers) == 0 {
		slog.Debug("No authorization servers in resource metadata, using auth server from WWW-Authenticate header")
		resourceMetadata.AuthorizationServers = []string{authServer}
	}

	oauth := &oauth{metadataClient: &http.Client{Timeout: 5 * time.Second}}
	authServerMetadata, err := oauth.getAuthorizationServerMetadata(ctx, resourceMetadata.AuthorizationServers[0])
	if err != nil {
		return fmt.Errorf("failed to fetch authorization server metadata: %w", err)
	}

	slog.Debug("Creating OAuth callback server")
	var callbackPort int
	if t.oauthConfig != nil {
		callbackPort = t.oauthConfig.CallbackPort
	}
	callbackServer, err := NewCallbackServerOnPort(callbackPort)
	if err != nil {
		return fmt.Errorf("failed to create callback server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown callback server", "error", err)
		}
	}()

	if err := callbackServer.Start(); err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	redirectURI := callbackServer.resolveRedirectURI(callbackRedirectURLFrom(t.oauthConfig))
	slog.Debug("Using redirect URI", "uri", redirectURI)

	var clientID string
	var clientSecret string
	var scopes []string

	switch {
	case t.oauthConfig != nil && t.oauthConfig.ClientID != "":
		// Use explicit credentials from config
		slog.Debug("Using explicit OAuth credentials from config")
		clientID = t.oauthConfig.ClientID
		clientSecret = t.oauthConfig.ClientSecret
		scopes = t.oauthConfig.Scopes
	case authServerMetadata.RegistrationEndpoint != "":
		slog.Debug("Attempting dynamic client registration")
		clientID, clientSecret, err = RegisterClient(ctx, authServerMetadata, redirectURI, nil)
		if err != nil {
			slog.Debug("Dynamic registration failed", "error", err)
			// TODO(rumpl): fall back to requesting client ID from user
			return err
		}
	default:
		// TODO(rumpl): fall back to requesting client ID from user
		return errors.New("authorization server does not support dynamic client registration and no explicit OAuth credentials configured")
	}

	state, err := GenerateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	callbackServer.SetExpectedState(state)
	verifier := GeneratePKCEVerifier()

	authURL := BuildAuthorizationURL(
		authServerMetadata.AuthorizationEndpoint,
		clientID,
		redirectURI,
		state,
		oauth2.S256ChallengeFromVerifier(verifier),
		t.baseURL,
		scopes,
	)

	result, err := t.client.requestElicitation(ctx, &mcpsdk.ElicitParams{
		Message:         fmt.Sprintf("The MCP server at %s requires OAuth authorization. Do you want to proceed?", t.baseURL),
		RequestedSchema: nil,
		Meta: map[string]any{
			"cagent/type":       "oauth_flow",
			"cagent/server_url": t.baseURL,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send elicitation request: %w", err)
	}

	slog.Debug("Elicitation response received", "result", result)

	if result.Action != tools.ElicitationActionAccept {
		return errors.New("user declined OAuth authorization")
	}

	slog.Debug("Requesting authorization code", "url", authURL)

	code, receivedState, err := RequestAuthorizationCode(ctx, authURL, callbackServer, state)
	if err != nil {
		return fmt.Errorf("failed to get authorization code: %w", err)
	}

	if receivedState != state {
		return errors.New("state mismatch in authorization response")
	}

	slog.Debug("Exchanging authorization code for token")
	token, err := ExchangeCodeForToken(
		ctx,
		authServerMetadata.TokenEndpoint,
		code,
		verifier,
		clientID,
		clientSecret,
		redirectURI,
	)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}

	token.ClientID = clientID
	token.ClientSecret = clientSecret
	token.AuthServer = resourceMetadata.AuthorizationServers[0]

	if err := t.tokenStore.StoreToken(t.baseURL, token); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Notify the runtime that the OAuth flow was successful
	t.client.oauthSuccess()

	slog.Debug("OAuth flow completed successfully")
	return nil
}

// handleUnmanagedOAuthFlow performs the OAuth flow for remote/unmanaged scenarios
// where the client handles the OAuth interaction instead of us
func (t *oauthTransport) handleUnmanagedOAuthFlow(ctx context.Context, authServer, wwwAuth string) error {
	slog.Debug("Starting unmanaged OAuth flow for server", "url", t.baseURL)

	// Extract resource URL from WWW-Authenticate header
	resourceURL := cmp.Or(resourceMetadataFromWWWAuth(wwwAuth), authServer+"/.well-known/oauth-protected-resource")

	resourceReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(resourceReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		_, _ = io.ReadAll(resp.Body)
		return errors.New("failed to fetch protected resource metadata")
	}
	var resourceMetadata protectedResourceMetadata
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&resourceMetadata); err != nil {
			return err
		}
	}

	if len(resourceMetadata.AuthorizationServers) == 0 {
		slog.Debug("No authorization servers in resource metadata, using auth server from WWW-Authenticate header")
		resourceMetadata.AuthorizationServers = []string{authServer}
	}

	oauth := &oauth{metadataClient: &http.Client{Timeout: 5 * time.Second}}
	authServerMetadata, err := oauth.getAuthorizationServerMetadata(ctx, resourceMetadata.AuthorizationServers[0])
	if err != nil {
		return fmt.Errorf("failed to fetch authorization server metadata: %w", err)
	}

	slog.Debug("Sending OAuth elicitation request to client")

	result, err := t.client.requestElicitation(ctx, &mcpsdk.ElicitParams{
		Message:         "OAuth authorization required for " + t.baseURL,
		RequestedSchema: nil,
		Meta: map[string]any{
			"cagent/type":          "oauth_flow",
			"cagent/server_url":    t.baseURL,
			"auth_server":          resourceMetadata.AuthorizationServers[0],
			"auth_server_metadata": authServerMetadata,
			"resource_metadata":    resourceMetadata,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send elicitation request: %w", err)
	}

	slog.Debug("Received elicitation response from client", "action", result.Action)

	if result.Action != tools.ElicitationActionAccept {
		return errors.New("OAuth flow declined or cancelled by client")
	}
	if result.Content == nil {
		return errors.New("no token received from client")
	}

	tokenData := result.Content

	token := &OAuthToken{}

	if accessToken, ok := tokenData["access_token"].(string); ok {
		token.AccessToken = accessToken
	} else {
		return errors.New("access_token missing or invalid in client response")
	}

	if tokenType, ok := tokenData["token_type"].(string); ok {
		token.TokenType = tokenType
	}

	if expiresIn, ok := tokenData["expires_in"].(float64); ok {
		token.ExpiresIn = int(expiresIn)
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	token.AuthServer = resourceMetadata.AuthorizationServers[0]

	if refreshToken, ok := tokenData["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}
	if err := t.tokenStore.StoreToken(t.baseURL, token); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Notify the runtime that the OAuth flow was successful
	t.client.oauthSuccess()

	slog.Debug("Managed OAuth flow completed successfully")
	return nil
}
