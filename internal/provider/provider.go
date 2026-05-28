// Package provider defines the Provider interface and shared repository types.
package provider

import (
	"context"
)

// Repository holds metadata about a remote Git repository.
type Repository struct {
	Name          string
	FullName      string
	SSHURL        string
	HTTPSURL      string
	DefaultBranch string
	Description   string
	AccountID     string
}

// ListOptions controls repository listing behaviour.
type ListOptions struct {
	IncludeArchived bool
	PageSize        int
}

// Provider can list repositories for a remote Git provider account.
type Provider interface {
	Name() string
	ListRepositories(ctx context.Context, opts ListOptions) ([]*Repository, error)
}
