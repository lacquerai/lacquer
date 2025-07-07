package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
)

// AuthProvider handles authentication for MCP transports
type AuthProvider interface {
	GetAuthHeader(ctx context.Context) (string, error)
}

// NoAuthProvider provides no authentication
type NoAuthProvider struct{}

func (p *NoAuthProvider) GetAuthHeader(ctx context.Context) (string, error) {
	return "", nil
}

// APIKeyAuthProvider provides API key authentication
type APIKeyAuthProvider struct {
	apiKey string
}

func NewAPIKeyAuthProvider(apiKey string) *APIKeyAuthProvider {
	return &APIKeyAuthProvider{apiKey: apiKey}
}

func (p *APIKeyAuthProvider) GetAuthHeader(ctx context.Context) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("API key is empty")
	}
	return "Bearer " + p.apiKey, nil
}

// BasicAuthProvider provides basic authentication
type BasicAuthProvider struct {
	username string
	password string
}

func NewBasicAuthProvider(username, password string) *BasicAuthProvider {
	return &BasicAuthProvider{
		username: username,
		password: password,
	}
}

func (p *BasicAuthProvider) GetAuthHeader(ctx context.Context) (string, error) {
	if p.username == "" {
		return "", fmt.Errorf("username is empty")
	}
	auth := p.username + ":" + p.password
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))
	return "Basic " + encoded, nil
}

// OAuth2Token represents an OAuth2 access token
type OAuth2Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	ExpiresAt   time.Time `json:"-"`
}

// OAuth2AuthProvider provides OAuth2 authentication
type OAuth2AuthProvider struct {
	clientID     string
	clientSecret string
	tokenURL     string
	scopes       string
	token        *OAuth2Token
	mu           sync.RWMutex
	httpClient   *http.Client
}

func NewOAuth2AuthProvider(clientID, clientSecret, tokenURL, scopes string) *OAuth2AuthProvider {
	return &OAuth2AuthProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     tokenURL,
		scopes:       scopes,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *OAuth2AuthProvider) GetAuthHeader(ctx context.Context) (string, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return "", err
	}

	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return tokenType + " " + token.AccessToken, nil
}

func (p *OAuth2AuthProvider) getToken(ctx context.Context) (*OAuth2Token, error) {
	p.mu.RLock()
	if p.token != nil && time.Now().Before(p.token.ExpiresAt) {
		token := p.token
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	// Need to refresh token
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.token != nil && time.Now().Before(p.token.ExpiresAt) {
		return p.token, nil
	}

	// Request new token
	token, err := p.requestToken(ctx)
	if err != nil {
		return nil, err
	}

	p.token = token
	return token, nil
}

func (p *OAuth2AuthProvider) requestToken(ctx context.Context) (*OAuth2Token, error) {
	// Prepare request data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", p.clientID)
	data.Set("client_secret", p.clientSecret)
	if p.scopes != "" {
		data.Set("scope", p.scopes)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var token OAuth2Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expiration time (with 5 minute buffer)
	expiresIn := time.Duration(token.ExpiresIn) * time.Second
	if expiresIn > 5*time.Minute {
		expiresIn -= 5 * time.Minute
	}
	token.ExpiresAt = time.Now().Add(expiresIn)

	return &token, nil
}

// CreateAuthProvider creates an auth provider based on the config
func CreateAuthProvider(config *ast.MCPAuthConfig) (AuthProvider, error) {
	if config == nil {
		return &NoAuthProvider{}, nil
	}

	switch config.Type {
	case "none":
		return &NoAuthProvider{}, nil

	case "api_key":
		if config.APIKey == "" {
			return nil, fmt.Errorf("API key is required for api_key auth")
		}
		return NewAPIKeyAuthProvider(config.APIKey), nil

	case "basic":
		if config.Username == "" {
			return nil, fmt.Errorf("username is required for basic auth")
		}
		return NewBasicAuthProvider(config.Username, config.Password), nil

	case "oauth2":
		if config.ClientID == "" {
			return nil, fmt.Errorf("client_id is required for oauth2 auth")
		}
		if config.ClientSecret == "" {
			return nil, fmt.Errorf("client_secret is required for oauth2 auth")
		}
		if config.TokenURL == "" {
			return nil, fmt.Errorf("token_url is required for oauth2 auth")
		}
		return NewOAuth2AuthProvider(
			config.ClientID,
			config.ClientSecret,
			config.TokenURL,
			config.Scopes,
		), nil

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", config.Type)
	}
}
