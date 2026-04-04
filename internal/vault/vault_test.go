package vault

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── NewClient ─────────────────────────────────────────────────────────────

func TestNewClient_FieldsSet(t *testing.T) {
	c := NewClient("https://vault.example.com", "client-id", "secret")

	if c.BaseURL != "https://vault.example.com" {
		t.Errorf("BaseURL not set correctly: %q", c.BaseURL)
	}
	if c.ClientID != "client-id" {
		t.Errorf("ClientID not set correctly: %q", c.ClientID)
	}
	if c.ClientSecret != "secret" {
		t.Errorf("ClientSecret not set correctly: %q", c.ClientSecret)
	}
	if c.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
}

func TestNewClient_HasHTTPTimeout(t *testing.T) {
	c := NewClient("https://vault.example.com", "id", "secret")
	if c.httpClient.Timeout == 0 {
		t.Error("httpClient should have a non-zero timeout to prevent hangs")
	}
}

// ── getToken ─────────────────────────────────────────────────────────────

func TestGetToken_CachesToken(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok-123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")

	tok1, err := c.getToken()
	if err != nil {
		t.Fatalf("first getToken failed: %v", err)
	}
	tok2, err := c.getToken()
	if err != nil {
		t.Fatalf("second getToken failed: %v", err)
	}

	if tok1 != "tok-123" || tok2 != "tok-123" {
		t.Errorf("expected 'tok-123', got %q / %q", tok1, tok2)
	}
	if callCount != 1 {
		t.Errorf("token endpoint should be called once (cached), got %d calls", callCount)
	}
}

func TestGetToken_RefreshesAfterExpiry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok-fresh"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	// Pre-set an expired token
	c.cachedToken = "tok-expired"
	c.tokenExpiry = time.Now().Add(-1 * time.Minute)

	tok, err := c.getToken()
	if err != nil {
		t.Fatalf("getToken failed: %v", err)
	}
	if tok != "tok-fresh" {
		t.Errorf("expected fresh token, got %q", tok)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call to refresh, got %d", callCount)
	}
}

func TestGetToken_ServerErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-client", "wrong-secret")
	_, err := c.getToken()
	if err == nil {
		t.Error("getToken should return error on non-200 response")
	}
}

// ── GetSecret ─────────────────────────────────────────────────────────────

func TestGetSecret_FindsSecretByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/sync":
			json.NewEncoder(w).Encode(syncResponse{
				Ciphers: []cipher{
					{Type: 1, Name: "AGENT_TOKEN_abc123", Login: &login{Password: "secret-value"}},
					{Type: 1, Name: "OTHER_SECRET", Login: &login{Password: "other"}},
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	val, err := c.GetSecret("AGENT_TOKEN_abc123")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if val != "secret-value" {
		t.Errorf("expected 'secret-value', got %q", val)
	}
}

func TestGetSecret_NotFoundReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/sync":
			json.NewEncoder(w).Encode(syncResponse{Ciphers: []cipher{}})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	_, err := c.GetSecret("NONEXISTENT")
	if err == nil {
		t.Error("GetSecret should return error when secret is not found")
	}
}

func TestGetSecret_FallsBackToDataField(t *testing.T) {
	// Some Vaultwarden versions return Data.Password instead of Login.Password
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/sync":
			json.NewEncoder(w).Encode(syncResponse{
				Ciphers: []cipher{
					{Type: 1, Name: "MY_SECRET", Data: &data{Password: "data-password"}},
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	val, err := c.GetSecret("MY_SECRET")
	if err != nil {
		t.Fatalf("GetSecret with Data field failed: %v", err)
	}
	if val != "data-password" {
		t.Errorf("expected 'data-password', got %q", val)
	}
}

// ── StoreSecret ─────────────────────────────────────────────────────────────

func TestStoreSecret_SuccessfulFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/ciphers":
			json.NewEncoder(w).Encode(map[string]string{"id": "cipher-uuid-123"})
		default:
			// PUT /api/ciphers/{id}/collections
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	err := c.StoreSecret("TEST_SECRET", "value", "org-id", "col-id")
	if err != nil {
		t.Errorf("StoreSecret should succeed, got: %v", err)
	}
}

// TestStoreSecret_UnmarshalErrorHandled verifies that if the cipher creation
// response cannot be parsed, an error is returned.
//
// RED: This test currently exposes the bug at vault.go:185 where
// json.Unmarshal error is not checked — if the response body is empty/invalid,
// created.ID will be "" and the function returns "could not get cipher ID".
// The existing behavior actually catches this via the created.ID == "" check,
// but the json.Unmarshal error itself is silently discarded.
func TestStoreSecret_InvalidCipherResponseReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/ciphers":
			// Return 200 but with invalid JSON (not a cipher object)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json`))
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	err := c.StoreSecret("TEST", "val", "org", "col")
	if err == nil {
		t.Error("StoreSecret should return error when cipher response JSON is invalid")
	}
}

func TestStoreSecret_ServerErrorOnCreateReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity/connect/token":
			json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok"})
		case "/api/ciphers":
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "client", "secret")
	err := c.StoreSecret("TEST", "val", "org", "col")
	if err == nil {
		t.Error("StoreSecret should return error on non-200 from server")
	}
}
