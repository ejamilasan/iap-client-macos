package auth

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// AuthService handles Google authentication via gcloud ADC
type AuthService struct {
	tokenSource oauth2.TokenSource
	authMethod  string // "gcloud"
	mu          sync.RWMutex
}

// Scopes required for IAP and Compute Engine access
var requiredScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/compute",
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	return &AuthService{}
}

// IsGcloudInstalled checks if gcloud CLI is available
func (s *AuthService) IsGcloudInstalled() bool {
	_, err := exec.LookPath("gcloud")
	return err == nil
}

// IsAuthenticated checks if user has valid ADC credentials
func (s *AuthService) IsAuthenticated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try Application Default Credentials (from gcloud)
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, requiredScopes...)
	if err == nil {
		token, err := creds.TokenSource.Token()
		if err == nil && token.Valid() {
			s.tokenSource = creds.TokenSource
			s.authMethod = "gcloud"
			return true
		}
	}

	return false
}

// GetAuthStatus returns the current authentication status
func (s *AuthService) GetAuthStatus() map[string]interface{} {
	isAuth := s.IsAuthenticated()
	gcloudInstalled := s.IsGcloudInstalled()

	s.mu.RLock()
	authMethod := s.authMethod
	s.mu.RUnlock()

	result := map[string]interface{}{
		"authenticated":   isAuth,
		"gcloudInstalled": gcloudInstalled,
		"authMethod":      authMethod,
	}

	if isAuth && authMethod == "gcloud" {
		account := s.getGcloudAccount()
		if account != "" {
			result["account"] = account
		}
	}

	return result
}

// getGcloudAccount returns the current gcloud account email
func (s *AuthService) getGcloudAccount() string {
	cmd := exec.Command("gcloud", "config", "get-value", "account")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// LoginWithGcloud runs gcloud auth application-default login
func (s *AuthService) LoginWithGcloud(ctx context.Context) error {
	if !s.IsGcloudInstalled() {
		return fmt.Errorf("gcloud CLI is not installed. Please install it from https://cloud.google.com/sdk/docs/install")
	}

	// Build the scopes argument
	scopesArg := strings.Join(requiredScopes, ",")

	// Run gcloud auth application-default login
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "application-default", "login",
		"--scopes", scopesArg,
	)

	// Run and wait for completion
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud auth failed: %w\nOutput: %s", err, string(output))
	}

	// Verify credentials are now available
	if !s.IsAuthenticated() {
		return fmt.Errorf("authentication completed but credentials not found")
	}

	s.mu.Lock()
	s.authMethod = "gcloud"
	s.mu.Unlock()

	return nil
}

// GetTokenSource returns the current token source for API calls
func (s *AuthService) GetTokenSource() oauth2.TokenSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use ADC
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, requiredScopes...)
	if err == nil {
		return creds.TokenSource
	}

	return nil
}

// GetToken returns the current token
func (s *AuthService) GetToken(ctx context.Context) (*oauth2.Token, error) {
	ts := s.GetTokenSource()
	if ts == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	return ts.Token()
}

// Logout clears credentials
func (s *AuthService) Logout() error {
	s.mu.Lock()
	s.tokenSource = nil
	s.authMethod = ""
	s.mu.Unlock()

	// Revoke ADC if gcloud is available
	if s.IsGcloudInstalled() {
		cmd := exec.Command("gcloud", "auth", "application-default", "revoke", "--quiet")
		cmd.Run()
	}

	return nil
}
