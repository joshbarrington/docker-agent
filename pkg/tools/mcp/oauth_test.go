package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestExchangeCodeForToken_PreservesClientCredentials verifies that
// ExchangeCodeForToken stores the client_id and client_secret on the
// returned OAuthToken so they are available for subsequent refresh calls.
func TestExchangeCodeForToken_PreservesClientCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("client_id"); got != "my-client" {
			t.Errorf("client_id = %q, want %q", got, "my-client")
		}
		if got := r.FormValue("client_secret"); got != "my-secret" {
			t.Errorf("client_secret = %q, want %q", got, "my-secret")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-new",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "rt-new",
		})
	}))
	defer srv.Close()

	token, err := ExchangeCodeForToken(t.Context(), srv.URL, "code", "verifier", "my-client", "my-secret", "http://localhost/callback")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken: %v", err)
	}

	if token.ClientID != "my-client" {
		t.Errorf("ClientID = %q, want %q", token.ClientID, "my-client")
	}
	if token.ClientSecret != "my-secret" {
		t.Errorf("ClientSecret = %q, want %q", token.ClientSecret, "my-secret")
	}
}

// TestRefreshAccessToken_PreservesClientCredentials verifies that
// RefreshAccessToken carries the client credentials through to the new token.
func TestRefreshAccessToken_PreservesClientCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("client_id"); got != "cid" {
			t.Errorf("client_id = %q, want %q", got, "cid")
		}
		if got := r.FormValue("client_secret"); got != "csec" {
			t.Errorf("client_secret = %q, want %q", got, "csec")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at-refreshed",
			"token_type":   "Bearer",
			"expires_in":   7200,
			// Server does NOT return a new refresh_token – old one should be preserved.
		})
	}))
	defer srv.Close()

	token, err := RefreshAccessToken(t.Context(), srv.URL, "old-rt", "cid", "csec")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}

	if token.AccessToken != "at-refreshed" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "at-refreshed")
	}
	if token.RefreshToken != "old-rt" {
		t.Errorf("RefreshToken = %q, want %q (should be preserved)", token.RefreshToken, "old-rt")
	}
	if token.ClientID != "cid" {
		t.Errorf("ClientID = %q, want %q", token.ClientID, "cid")
	}
	if token.ClientSecret != "csec" {
		t.Errorf("ClientSecret = %q, want %q", token.ClientSecret, "csec")
	}
}

// TestGetValidToken_UsesStoredCredentialsForRefresh verifies that the
// oauthTransport.getValidToken method sends the stored client credentials
// when silently refreshing an expired token.
func TestGetValidToken_UsesStoredCredentialsForRefresh(t *testing.T) {
	var receivedClientID, receivedClientSecret string

	// Use a mux so we can reference srv.URL in closures (srv is assigned before handlers run).
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"token_endpoint":         srv.URL + "/token",
			"authorization_endpoint": srv.URL + "/authorize",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		receivedClientID = r.FormValue("client_id")
		receivedClientSecret = r.FormValue("client_secret")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "fresh-at",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "fresh-rt",
		})
	})

	// Pre-populate an expired token with stored client credentials.
	store := NewInMemoryTokenStore()
	expiredToken := &OAuthToken{
		AccessToken:  "old-at",
		TokenType:    "Bearer",
		RefreshToken: "old-rt",
		ExpiresAt:    time.Now().Add(-1 * time.Hour), // expired
		ClientID:     "stored-cid",
		ClientSecret: "stored-csec",
	}
	if err := store.StoreToken(srv.URL, expiredToken); err != nil {
		t.Fatal(err)
	}

	transport := &oauthTransport{
		base:       http.DefaultTransport,
		tokenStore: store,
		baseURL:    srv.URL,
	}

	got := transport.getValidToken(t.Context())
	if got == nil {
		t.Fatal("getValidToken returned nil, expected refreshed token")
	}
	if got.AccessToken != "fresh-at" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "fresh-at")
	}
	if receivedClientID != "stored-cid" {
		t.Errorf("token endpoint received client_id = %q, want %q", receivedClientID, "stored-cid")
	}
	if receivedClientSecret != "stored-csec" {
		t.Errorf("token endpoint received client_secret = %q, want %q", receivedClientSecret, "stored-csec")
	}

	// Verify the refreshed token also carries the credentials forward.
	updated, err := store.GetToken(srv.URL)
	if err != nil {
		t.Fatalf("GetToken after refresh: %v", err)
	}
	if updated.ClientID != "stored-cid" {
		t.Errorf("stored ClientID = %q, want %q", updated.ClientID, "stored-cid")
	}
	if updated.ClientSecret != "stored-csec" {
		t.Errorf("stored ClientSecret = %q, want %q", updated.ClientSecret, "stored-csec")
	}
}

// TestGetValidToken_UsesStoredAuthServerForRefresh verifies that silent
// refresh uses the discovered auth server rather than assuming the MCP
// server URL also hosts the OAuth metadata.
func TestGetValidToken_UsesStoredAuthServerForRefresh(t *testing.T) {
	var refreshRequests int

	authMux := http.NewServeMux()
	authSrv := httptest.NewServer(authMux)
	defer authSrv.Close()

	authMux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 authSrv.URL,
			"token_endpoint":         authSrv.URL + "/token",
			"authorization_endpoint": authSrv.URL + "/authorize",
		})
	})
	authMux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		refreshRequests++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("refresh_token"); got != "old-rt" {
			t.Fatalf("refresh_token = %q, want %q", got, "old-rt")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "fresh-at",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "fresh-rt",
		})
	})

	mcpSrv := httptest.NewServer(http.NotFoundHandler())
	defer mcpSrv.Close()

	store := NewInMemoryTokenStore()
	expiredToken := &OAuthToken{
		AccessToken:  "old-at",
		TokenType:    "Bearer",
		RefreshToken: "old-rt",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		ClientID:     "stored-cid",
		ClientSecret: "stored-csec",
		AuthServer:   authSrv.URL,
	}
	if err := store.StoreToken(mcpSrv.URL, expiredToken); err != nil {
		t.Fatal(err)
	}

	transport := &oauthTransport{
		base:       http.DefaultTransport,
		tokenStore: store,
		baseURL:    mcpSrv.URL,
	}

	got := transport.getValidToken(t.Context())
	if got == nil {
		t.Fatal("getValidToken returned nil, expected refreshed token")
	}
	if got.AccessToken != "fresh-at" {
		t.Fatalf("AccessToken = %q, want %q", got.AccessToken, "fresh-at")
	}
	if got.AuthServer != authSrv.URL {
		t.Fatalf("AuthServer = %q, want %q", got.AuthServer, authSrv.URL)
	}
	if refreshRequests != 1 {
		t.Fatalf("refreshRequests = %d, want 1", refreshRequests)
	}
}

// TestOAuthTokenClientCredentials_JSONRoundTrip verifies that ClientID and
// ClientSecret survive JSON serialization (important for keyring storage).
func TestOAuthTokenClientCredentials_JSONRoundTrip(t *testing.T) {
	token := &OAuthToken{
		AccessToken:  "at",
		TokenType:    "Bearer",
		RefreshToken: "rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		ClientID:     "cid",
		ClientSecret: "csec",
		AuthServer:   "https://auth.example.com",
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got OAuthToken
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ClientID != "cid" {
		t.Errorf("ClientID = %q, want %q", got.ClientID, "cid")
	}
	if got.ClientSecret != "csec" {
		t.Errorf("ClientSecret = %q, want %q", got.ClientSecret, "csec")
	}
	if got.AuthServer != "https://auth.example.com" {
		t.Errorf("AuthServer = %q, want %q", got.AuthServer, "https://auth.example.com")
	}
}

// TestOAuthTokenClientCredentials_OmittedWhenEmpty verifies the omitempty
// tag works so tokens without client credentials don't leak empty fields.
func TestOAuthTokenClientCredentials_OmittedWhenEmpty(t *testing.T) {
	token := &OAuthToken{
		AccessToken: "at",
		TokenType:   "Bearer",
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	if _, ok := raw["client_id"]; ok {
		t.Error("client_id should be omitted when empty")
	}
	if _, ok := raw["client_secret"]; ok {
		t.Error("client_secret should be omitted when empty")
	}
}

// TestRefreshAccessToken_SendsEmptyClientIDWhenNotStored ensures that when
// no client credentials were stored (legacy tokens), the refresh still
// sends whatever was provided (empty string), matching the old behavior.
func TestRefreshAccessToken_SendsEmptyClientIDWhenNotStored(t *testing.T) {
	var receivedForm url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		receivedForm = r.Form

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-at",
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	_, err := RefreshAccessToken(t.Context(), srv.URL, "rt", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// client_id is always sent (even empty) per the current implementation.
	if got := receivedForm.Get("client_id"); got != "" {
		t.Errorf("client_id = %q, want empty", got)
	}
	// client_secret should NOT be sent when empty.
	if receivedForm.Has("client_secret") {
		t.Error("client_secret should not be sent when empty")
	}
}

// TestCallbackServer_RejectsCallbackBeforeStateSet verifies that a callback
// arriving before SetExpectedState is called is rejected (CSRF protection).
func TestCallbackServer_RejectsCallbackBeforeStateSet(t *testing.T) {
	cs, err := NewCallbackServer()
	if err != nil {
		t.Fatal(err)
	}
	if err := cs.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Shutdown(t.Context()) }()

	// Send a callback before SetExpectedState has been called.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, cs.GetRedirectURI()+"?code=authcode&state=anything", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d — callback accepted without expected state set", resp.StatusCode)
	}
}
