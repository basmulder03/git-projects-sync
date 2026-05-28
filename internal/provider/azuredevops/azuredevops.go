// Package azuredevops implements provider.Provider for Azure DevOps.
package azuredevops

import (
	"context"
	"fmt"
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
	connection  *azuredevops.Connection
	account     config.AccountConfig
	rateLimiter *rate.Limiter
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

	return &AzureDevOpsProvider{
		connection:  conn,
		account:     account,
		rateLimiter: rate.NewLimiter(rate.Every(2*time.Second), 1),
	}, nil
}

// Name returns the provider identifier.
func (a *AzureDevOpsProvider) Name() string { return "azure_devops" }

// ListRepositories returns all git repositories across all projects in the organisation.
func (a *AzureDevOpsProvider) ListRepositories(ctx context.Context, opts provider.ListOptions) ([]*provider.Repository, error) {
	if err := a.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("azuredevops rate limiter: %w", err)
	}

	apiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	coreClient, err := azdocore.NewClient(apiCtx, a.connection)
	if err != nil {
		return nil, fmt.Errorf("azuredevops core client: %w", err)
	}

	projects, err := coreClient.GetProjects(apiCtx, azdocore.GetProjectsArgs{})
	if err != nil {
		return nil, fmt.Errorf("azuredevops list projects: %w", err)
	}

	var repos []*provider.Repository
	if projects == nil {
		return repos, nil
	}

	for _, proj := range projects.Value {
		projName := *proj.Name

		if err := a.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("azuredevops rate limiter: %w", err)
		}

		gitCtx, gitCancel := context.WithTimeout(ctx, 60*time.Second)
		gitClient, err := azdogit.NewClient(gitCtx, a.connection)
		gitCancel()
		if err != nil {
			return nil, fmt.Errorf("azuredevops git client: %w", err)
		}

		if err := a.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("azuredevops rate limiter: %w", err)
		}

		repoCtx, repoCancel := context.WithTimeout(ctx, 60*time.Second)
		projectRepos, err := gitClient.GetRepositories(repoCtx, azdogit.GetRepositoriesArgs{
			Project: &projName,
		})
		repoCancel()
		if err != nil {
			return nil, fmt.Errorf("azuredevops list repos for project %s: %w", projName, err)
		}

		if projectRepos == nil {
			continue
		}

		for _, r := range *projectRepos {
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
		// strip "refs/heads/" prefix
		db := *r.DefaultBranch
		if len(db) > len("refs/heads/") {
			defaultBranch = db[len("refs/heads/"):]
		} else {
			defaultBranch = db
		}
	}

	sshURL := fmt.Sprintf("git@ssh.dev.azure.com-%s:v3/%s/%s/%s", a.account.ID, org, projectName, repoName)
	httpsURL := fmt.Sprintf("https://%s@dev.azure.com/%s/%s/_git/%s", org, org, projectName, repoName)

	description := ""
	if r.RemoteUrl != nil {
		// use description field if available; AzDo SDK does not expose description here
	}

	return &provider.Repository{
		Name:          repoName,
		FullName:      fullName,
		SSHURL:        sshURL,
		HTTPSURL:      httpsURL,
		DefaultBranch: defaultBranch,
		Description:   description,
		AccountID:     a.account.ID,
	}
}
