// Package registry constructs Provider implementations by account type.
package registry

import (
	"fmt"

	"github.com/basmulder03/git-projects-sync/internal/auth"
	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/provider"
	"github.com/basmulder03/git-projects-sync/internal/provider/azuredevops"
	"github.com/basmulder03/git-projects-sync/internal/provider/github"
)

// NewProvider constructs the correct Provider for the given account config.
func NewProvider(account config.AccountConfig, a auth.Authenticator) (provider.Provider, error) {
	switch account.Provider {
	case "github":
		p, err := github.New(account, a)
		if err != nil {
			return nil, fmt.Errorf("provider github: %w", err)
		}
		return p, nil
	case "azure_devops":
		p, err := azuredevops.New(account, a)
		if err != nil {
			return nil, fmt.Errorf("provider azure_devops: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", account.Provider)
	}
}
