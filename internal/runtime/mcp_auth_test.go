package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoAuthProvider(t *testing.T) {
	provider := &NoAuthProvider{}
	header, err := provider.GetAuthHeader(context.Background())
	require.NoError(t, err)
	assert.Empty(t, header)
}

func TestAPIKeyAuthProvider(t *testing.T) {
	t.Run("Valid API Key", func(t *testing.T) {
		provider := NewAPIKeyAuthProvider("test-api-key")
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Bearer test-api-key", header)
	})

	t.Run("Empty API Key", func(t *testing.T) {
		provider := NewAPIKeyAuthProvider("")
		header, err := provider.GetAuthHeader(context.Background())
		assert.Error(t, err)
		assert.Empty(t, header)
	})
}

func TestBasicAuthProvider(t *testing.T) {
	t.Run("Valid Credentials", func(t *testing.T) {
		provider := NewBasicAuthProvider("user", "pass")
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Basic dXNlcjpwYXNz", header) // base64("user:pass")
	})

	t.Run("Empty Username", func(t *testing.T) {
		provider := NewBasicAuthProvider("", "pass")
		header, err := provider.GetAuthHeader(context.Background())
		assert.Error(t, err)
		assert.Empty(t, header)
	})

	t.Run("Empty Password Allowed", func(t *testing.T) {
		provider := NewBasicAuthProvider("user", "")
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Basic dXNlcjo=", header) // base64("user:")
	})
}

func TestOAuth2AuthProvider(t *testing.T) {
	// Create mock OAuth2 server
	tokenRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++

		// Check request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)

		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "test-client", r.Form.Get("client_id"))
		assert.Equal(t, "test-secret", r.Form.Get("client_secret"))
		assert.Equal(t, "read write", r.Form.Get("scope"))

		// Return token
		token := OAuth2Token{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	provider := NewOAuth2AuthProvider(
		"test-client",
		"test-secret",
		server.URL,
		"read write",
	)

	t.Run("Get Token", func(t *testing.T) {
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Bearer test-access-token", header)
		assert.Equal(t, 1, tokenRequests)
	})

	t.Run("Token Caching", func(t *testing.T) {
		// Second call should use cached token
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Bearer test-access-token", header)
		assert.Equal(t, 1, tokenRequests) // No new request
	})

	t.Run("Token Expiry", func(t *testing.T) {
		// Force token expiry
		provider.mu.Lock()
		provider.token.ExpiresAt = time.Now().Add(-1 * time.Hour)
		provider.mu.Unlock()

		// Should request new token
		header, err := provider.GetAuthHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Bearer test-access-token", header)
		assert.Equal(t, 2, tokenRequests)
	})
}

func TestOAuth2AuthProvider_Errors(t *testing.T) {
	t.Run("Server Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := NewOAuth2AuthProvider("client", "secret", server.URL, "")
		header, err := provider.GetAuthHeader(context.Background())
		assert.Error(t, err)
		assert.Empty(t, header)
	})

	t.Run("Invalid Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		provider := NewOAuth2AuthProvider("client", "secret", server.URL, "")
		header, err := provider.GetAuthHeader(context.Background())
		assert.Error(t, err)
		assert.Empty(t, header)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		provider := NewOAuth2AuthProvider("client", "secret", "not-a-url", "")
		header, err := provider.GetAuthHeader(context.Background())
		assert.Error(t, err)
		assert.Empty(t, header)
	})
}

func TestCreateAuthProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      *ast.MCPAuthConfig
		expectType  string
		expectError bool
	}{
		{
			name:       "No Config",
			config:     nil,
			expectType: "*runtime.NoAuthProvider",
		},
		{
			name: "None Type",
			config: &ast.MCPAuthConfig{
				Type: "none",
			},
			expectType: "*runtime.NoAuthProvider",
		},
		{
			name: "API Key",
			config: &ast.MCPAuthConfig{
				Type:   "api_key",
				APIKey: "test-key",
			},
			expectType: "*runtime.APIKeyAuthProvider",
		},
		{
			name: "API Key Missing",
			config: &ast.MCPAuthConfig{
				Type: "api_key",
			},
			expectError: true,
		},
		{
			name: "Basic Auth",
			config: &ast.MCPAuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			expectType: "*runtime.BasicAuthProvider",
		},
		{
			name: "Basic Auth Missing Username",
			config: &ast.MCPAuthConfig{
				Type:     "basic",
				Password: "pass",
			},
			expectError: true,
		},
		{
			name: "OAuth2",
			config: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientID:     "client",
				ClientSecret: "secret",
				TokenURL:     "https://example.com/token",
				Scopes:       "read write",
			},
			expectType: "*runtime.OAuth2AuthProvider",
		},
		{
			name: "OAuth2 Missing ClientID",
			config: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientSecret: "secret",
				TokenURL:     "https://example.com/token",
			},
			expectError: true,
		},
		{
			name: "OAuth2 Missing ClientSecret",
			config: &ast.MCPAuthConfig{
				Type:     "oauth2",
				ClientID: "client",
				TokenURL: "https://example.com/token",
			},
			expectError: true,
		},
		{
			name: "OAuth2 Missing TokenURL",
			config: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientID:     "client",
				ClientSecret: "secret",
			},
			expectError: true,
		},
		{
			name: "Unknown Type",
			config: &ast.MCPAuthConfig{
				Type: "unknown",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := CreateAuthProvider(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				require.NotNil(t, provider)

				// Check provider type
				actualType := fmt.Sprintf("%T", provider)
				assert.Equal(t, tt.expectType, actualType)
			}
		})
	}
}
