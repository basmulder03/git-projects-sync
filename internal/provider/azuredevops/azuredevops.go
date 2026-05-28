// Package azuredevops implements provider.Provider for Azure DevOps.
package azuredevops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	azdocore "github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"golang.org/x/time/rate"

	"github.com/basmulder03/git-projects-sync/internal/auth"
	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/provider"
)

// AzureDevOpsProvider lists repositories from an Azure DevOps organisation.
type AzureDevOpsProvider struct {
	account     config.AccountConfig
	rateLimiter *rate.Limiter
	// getProjects and getRepos are injectable for testing.
	getProjects func(ctx context.Context) ([]azdocore.TeamProjectReference, error)
	getRepos    func(ctx context.Context, project string) ([]azdogit.GitRepository, error)
}

// New creates an AzureDevOpsProvider authenticated with the given Authenticator.
func New(account config.AccountConfig, a auth.Authenticator) (*AzureDevOpsProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := a.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: get token: %w", err)
	}

	orgURL := fmt.Sprintf("https://dev.azure.com/%s", account.Organization)
	conn := azuredevops.NewPatConnection(orgURL, token)

	p := &AzureDevOpsProvider{
		account:     account,
		rateLimiter: rate.NewLimiter(rate.Every(2*time.Second), 1),
	}

	p.getProjects = func(ctx context.Context) ([]azdocore.TeamProjectReference, error) {
		apiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		coreClient, err := azdocore.NewClient(apiCtx, conn)
		if err != nil {
			return nil, fmt.Errorf("core client: %w", err)
		}
		result, err := coreClient.GetProjects(apiCtx, azdocore.GetProjectsArgs{})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, nil
		}
		return result.Value, nil
	}

	p.getRepos = func(ctx context.Context, project string) ([]azdogit.GitRepository, error) {
		apiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		gitClient, err := azdogit.NewClient(apiCtx, conn)
		if err != nil {
			return nil, fmt.Errorf("git client: %w", err)
		}
		result, err := gitClient.GetRepositories(apiCtx, azdogit.GetRepositoriesArgs{Project: &project})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, nil
		}
		return *result, nil
	}

	return p, nil
}

// Name returns the provider identifier.
func (a *AzureDevOpsProvider) Name() string { return "azure_devops" }

// ListRepositories returns all git repositories across all projects in the organisation.
func (a *AzureDevOpsProvider) ListRepositories(ctx context.Context, opts provider.ListOptions) ([]*provider.Repository, error) {
	if err := a.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("azuredevops rate limiter: %w", err)
	}

	projects, err := a.getProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("azuredevops list projects: %w", err)
	}

	var repos []*provider.Repository
	for _, proj := range projects {
		projName := *proj.Name

		if err := a.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("azuredevops rate limiter: %w", err)
		}

		projectRepos, err := a.getRepos(ctx, projName)
		if err != nil {
			return nil, fmt.Errorf("azuredevops list repos for project %s: %w", projName, err)
		}

		for _, r := range projectRepos {
			repos = append(repos, a.mapRepo(r, projName))
		}
	}

	return repos, nil
}

func (a *AzureDevOpsProvider) mapRepo(r azdogit.GitRepository, projectName string) *provider.Repository {
	org := a.account.Organization
	repoName := *r.Name
	fullName := fmt.Sprintf("%s/%s/%s", org, projectName, repoName)

	defaultBranch := ""
	if r.DefaultBranch != nil {
		db := *r.DefaultBranch
		defaultBranch = strings.TrimPrefix(db, "refs/heads/")
	}

	sshURL := fmt.Sprintf("git@ssh.dev.azure.com-%s:v3/%s/%s/%s", a.account.ID, org, projectName, repoName)
	httpsURL := fmt.Sprintf("https://%s@dev.azure.com/%s/%s/_git/%s", org, org, projectName, repoName)

	return &provider.Repository{
		Name:          repoName,
		FullName:      fullName,
		SSHURL:        sshURL,
		HTTPSURL:      httpsURL,
		DefaultBranch: defaultBranch,
		AccountID:     a.account.ID,
	}
}
