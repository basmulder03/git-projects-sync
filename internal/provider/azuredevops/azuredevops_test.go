package azuredevops

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	azdocore "github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"golang.org/x/time/rate"

	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/provider"
)

// makeProvider builds a provider with stubbed API functions — no HTTP involved.
func makeProvider(account config.AccountConfig, projects []azdocore.TeamProjectReference, repos map[string][]azdogit.GitRepository) *AzureDevOpsProvider {
	return &AzureDevOpsProvider{
		account:     account,
		rateLimiter: rate.NewLimiter(rate.Inf, 0),
		getProjects: func(_ context.Context) ([]azdocore.TeamProjectReference, error) {
			return projects, nil
		},
		getRepos: func(_ context.Context, project string) ([]azdogit.GitRepository, error) {
			return repos[project], nil
		},
	}
}

// helpers

func strPtr(s string) *string   { return &s }
func newUUID() *uuid.UUID       { id := uuid.New(); return &id }

func sampleProjects(names ...string) []azdocore.TeamProjectReference {
	out := make([]azdocore.TeamProjectReference, len(names))
	for i, n := range names {
		n := n
		out[i] = azdocore.TeamProjectReference{Id: newUUID(), Name: &n}
	}
	return out
}

func sampleRepo(name, defaultBranch string) azdogit.GitRepository {
	return azdogit.GitRepository{
		Id:            newUUID(),
		Name:          strPtr(name),
		DefaultBranch: strPtr("refs/heads/" + defaultBranch),
	}
}

// --- tests ---

func TestListRepositories_ReturnsAllRepos(t *testing.T) {
	acc := config.AccountConfig{
		ID:           "azdo-test",
		Provider:     "azure_devops",
		Organization: "myorg",
		CloneMethod:  "ssh",
	}

	projects := sampleProjects("proj1", "proj2")
	repos := map[string][]azdogit.GitRepository{
		"proj1": {sampleRepo("repoA", "main"), sampleRepo("repoB", "main")},
		"proj2": {sampleRepo("repoC", "develop")},
	}

	p := makeProvider(acc, projects, repos)
	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d repos, want 3", len(got))
	}
}

func TestListRepositories_FullNameFormat(t *testing.T) {
	acc := config.AccountConfig{
		ID: "w", Provider: "azure_devops", Organization: "contoso", CloneMethod: "ssh",
	}

	p := makeProvider(acc,
		sampleProjects("myproject"),
		map[string][]azdogit.GitRepository{"myproject": {sampleRepo("myrepo", "main")}},
	)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one repo")
	}
	want := "contoso/myproject/myrepo"
	if got[0].FullName != want {
		t.Errorf("FullName = %q, want %q", got[0].FullName, want)
	}
}

func TestListRepositories_SSHURLUsesHostAlias(t *testing.T) {
	acc := config.AccountConfig{
		ID: "azdo-work", Provider: "azure_devops", Organization: "fabrikam", CloneMethod: "ssh",
	}

	p := makeProvider(acc,
		sampleProjects("proj"),
		map[string][]azdogit.GitRepository{"proj": {sampleRepo("api", "main")}},
	)

	got, _ := p.ListRepositories(context.Background(), provider.ListOptions{})
	if len(got) == 0 {
		t.Fatal("expected repo")
	}
	wantSSH := "git@ssh.dev.azure.com-azdo-work:v3/fabrikam/proj/api"
	if got[0].SSHURL != wantSSH {
		t.Errorf("SSHURL = %q, want %q", got[0].SSHURL, wantSSH)
	}
}

func TestListRepositories_HTTPSURLFormat(t *testing.T) {
	acc := config.AccountConfig{
		ID: "w", Provider: "azure_devops", Organization: "contoso", CloneMethod: "https",
	}

	p := makeProvider(acc,
		sampleProjects("proj"),
		map[string][]azdogit.GitRepository{"proj": {sampleRepo("svc", "main")}},
	)

	got, _ := p.ListRepositories(context.Background(), provider.ListOptions{})
	if len(got) == 0 {
		t.Fatal("expected repo")
	}
	if !strings.HasPrefix(got[0].HTTPSURL, "https://") {
		t.Errorf("HTTPSURL should start with https://, got %q", got[0].HTTPSURL)
	}
	if !strings.Contains(got[0].HTTPSURL, "contoso") {
		t.Errorf("HTTPSURL should contain org, got %q", got[0].HTTPSURL)
	}
}

func TestListRepositories_DefaultBranchStripsRefsHeads(t *testing.T) {
	acc := config.AccountConfig{ID: "w", Provider: "azure_devops", Organization: "org"}
	p := makeProvider(acc,
		sampleProjects("p"),
		map[string][]azdogit.GitRepository{"p": {sampleRepo("r", "develop")}},
	)

	got, _ := p.ListRepositories(context.Background(), provider.ListOptions{})
	if len(got) == 0 {
		t.Fatal("expected repo")
	}
	if got[0].DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want develop", got[0].DefaultBranch)
	}
}

func TestListRepositories_SetsAccountID(t *testing.T) {
	acc := config.AccountConfig{ID: "my-azdo-id", Provider: "azure_devops", Organization: "org"}
	p := makeProvider(acc,
		sampleProjects("p"),
		map[string][]azdogit.GitRepository{"p": {sampleRepo("r", "main")}},
	)

	got, _ := p.ListRepositories(context.Background(), provider.ListOptions{})
	if len(got) == 0 {
		t.Fatal("expected repo")
	}
	if got[0].AccountID != "my-azdo-id" {
		t.Errorf("AccountID = %q, want my-azdo-id", got[0].AccountID)
	}
}

func TestListRepositories_EmptyOrg_ReturnsEmpty(t *testing.T) {
	acc := config.AccountConfig{ID: "w", Provider: "azure_devops", Organization: "org"}
	p := makeProvider(acc, []azdocore.TeamProjectReference{}, map[string][]azdogit.GitRepository{})

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d repos, want 0", len(got))
	}
}

func TestListRepositories_ProjectWithNoRepos_Skipped(t *testing.T) {
	acc := config.AccountConfig{ID: "w", Provider: "azure_devops", Organization: "org"}
	p := makeProvider(acc,
		sampleProjects("empty", "full"),
		map[string][]azdogit.GitRepository{
			"empty": {},
			"full":  {sampleRepo("r", "main")},
		},
	)

	got, _ := p.ListRepositories(context.Background(), provider.ListOptions{})
	if len(got) != 1 {
		t.Errorf("got %d repos, want 1", len(got))
	}
}

func TestName(t *testing.T) {
	p := &AzureDevOpsProvider{}
	if p.Name() != "azure_devops" {
		t.Errorf("Name() = %q, want azure_devops", p.Name())
	}
}
