// Package auth defines authentication interfaces and implementations.
package auth

import (
	"context"
	"fmt"

	"github.com/basmulder03/git-projects-sync/internal/keychain"
)

// Authenticator provides an auth token for a provider.
type Authenticator interface {
	Token(ctx context.Context) (string, error)
	Method() string
}

// PATAuthenticator retrieves a Personal Access Token from the OS keychain.
type PATAuthenticator struct {
	accountID string
}

// NewPATAuth creates a PATAuthenticator for the given account ID.
func NewPATAuth(accountID string) *PATAuthenticator {
	return &PATAuthenticator{accountID: accountID}
}

// Method returns the authentication method name.
func (p *PATAuthenticator) Method() string { return "pat" }

// Token retrieves the PAT from the OS keychain.
func (p *PATAuthenticator) Token(_ context.Context) (string, error) {
	token, err := keychain.GetPAT(p.accountID)
	if err != nil {
		return "", fmt.Errorf("auth token for %s: %w", p.accountID, err)
	}
	return token, nil
}
