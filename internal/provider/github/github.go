// Package github implements provider.Provider for GitHub.
package github

import (
	"context"
	"fmt"
	"time"

	gogithub "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	"github.com/basmulder03/git-projects-sync/internal/auth"
	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/provider"
)

// GitHubProvider lists GitHub repositories using the GitHub REST API.
type GitHubProvider struct {
	client      *gogithub.Client
	account     config.AccountConfig
	rateLimiter *rate.Limiter
}

// New creates a GitHubProvider authenticated with the given Authenticator.
func New(account config.AccountConfig, a auth.Authenticator) (*GitHubProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := a.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: get token: %w", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	client := gogithub.NewClient(httpClient)

	return &GitHubProvider{
		client:      client,
		account:     account,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 1),
	}, nil
}

// Name returns the provider identifier.
func (g *GitHubProvider) Name() string { return "github" }

// ListRepositories returns all repositories accessible to the authenticated user.
func (g *GitHubProvider) ListRepositories(ctx context.Context, opts provider.ListOptions) ([]*provider.Repository, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	var repos []*provider.Repository
	listOpts := &gogithub.RepositoryListByAuthenticatedUserOptions{
		ListOptions: gogithub.ListOptions{PerPage: pageSize},
	}

	for {
		if err := g.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("github rate limiter: %w", err)
		}

		apiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		ghRepos, resp, err := g.client.Repositories.ListByAuthenticatedUser(apiCtx, listOpts)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("github list repos: %w", err)
		}

		for _, r := range ghRepos {
			if !opts.IncludeArchived && r.GetArchived() {
				continue
			}
			repos = append(repos, g.mapRepo(r))
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return repos, nil
}

func (g *GitHubProvider) mapRepo(r *gogithub.Repository) *provider.Repository {
	owner := ""
	if r.Owner != nil {
		owner = r.GetOwner().GetLogin()
	}
	repoName := r.GetName()
	fullName := r.GetFullName()

	var sshURL, httpsURL string
	if g.account.CloneMethod == "ssh" {
		// Use SSH host alias so the correct identity file is selected.
		sshURL = fmt.Sprintf("git@github.com-%s:%s.git", g.account.ID, fullName)
	} else {
		httpsURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repoName)
	}
	if sshURL == "" {
		sshURL = r.GetSSHURL()
	}
	if httpsURL == "" {
		httpsURL = r.GetCloneURL()
	}

	return &provider.Repository{
		Name:          repoName,
		FullName:      fullName,
		SSHURL:        sshURL,
		HTTPSURL:      httpsURL,
		DefaultBranch: r.GetDefaultBranch(),
		Description:   r.GetDescription(),
		AccountID:     g.account.ID,
	}
}
