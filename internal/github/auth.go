package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// TokenProvider manages GitHub App installation tokens with caching
type TokenProvider struct {
	transport   *ghinstallation.Transport
	cachedToken string
	expiresAt   time.Time
	mu          sync.RWMutex
}

// NewTokenProvider creates a new TokenProvider with the given GitHub App credentials
func NewTokenProvider(appID, installationID int64, privateKey []byte) (*TokenProvider, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	return &TokenProvider{
		transport: transport,
	}, nil
}

// GetToken returns a valid installation token, refreshing if necessary
// Tokens are cached with a ~55 minute TTL (GitHub tokens are valid for 1 hour)
func (p *TokenProvider) GetToken() (string, error) {
	p.mu.RLock()
	if p.cachedToken != "" && time.Now().Before(p.expiresAt) {
		token := p.cachedToken
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	// Need to refresh token
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.cachedToken != "" && time.Now().Before(p.expiresAt) {
		return p.cachedToken, nil
	}

	token, err := p.transport.Token(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}

	p.cachedToken = token
	// Set expiry to 55 minutes (tokens are valid for 1 hour)
	p.expiresAt = time.Now().Add(55 * time.Minute)

	return token, nil
}

// AuthenticatedURL transforms a GitHub URL to include the access token
// Input: https://github.com/owner/repo.git
// Output: https://x-access-token:TOKEN@github.com/owner/repo.git
func (p *TokenProvider) AuthenticatedURL(originalURL string) (string, error) {
	token, err := p.GetToken()
	if err != nil {
		return "", err
	}

	return InjectToken(originalURL, token)
}

// InjectToken inserts an access token into a GitHub URL
// Input: https://github.com/owner/repo.git, token
// Output: https://x-access-token:TOKEN@github.com/owner/repo.git
func InjectToken(originalURL, token string) (string, error) {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Only modify HTTPS URLs
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("token injection only supported for HTTPS URLs, got: %s", parsed.Scheme)
	}

	// Set the authentication credentials
	parsed.User = url.UserPassword("x-access-token", token)

	return parsed.String(), nil
}

// IsGitHubURL checks if a URL points to GitHub
func IsGitHubURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "github.com" || strings.HasSuffix(host, ".github.com")
}
