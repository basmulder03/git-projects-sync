package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v60/github"
	"golang.org/x/time/rate"

	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/provider"
)

type mockRepo struct {
	ID            int                    `json:"id"`
	Name          string                 `json:"name"`
	FullName      string                 `json:"full_name"`
	Owner         map[string]interface{} `json:"owner"`
	CloneURL      string                 `json:"clone_url"`
	SSHURL        string                 `json:"ssh_url"`
	DefaultBranch string                 `json:"default_branch"`
	Description   string                 `json:"description"`
	Archived      bool                   `json:"archived"`
}

func newTestProvider(t *testing.T, handler http.Handler, account config.AccountConfig) *GitHubProvider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := gogithub.NewClient(nil)
	baseURL, err := url.Parse(ts.URL + "/")
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}
	client.BaseURL = baseURL
	client.UploadURL = baseURL

	return &GitHubProvider{
		client:      client,
		account:     account,
		rateLimiter: rate.NewLimiter(rate.Inf, 0),
	}
}

func TestListRepositories_ExcludesArchived(t *testing.T) {
	repos := []mockRepo{
		{ID: 1, Name: "active", FullName: "user/active", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/active.git", SSHURL: "git@github.com:user/active.git",
			DefaultBranch: "main", Archived: false},
		{ID: 2, Name: "archived", FullName: "user/archived", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/archived.git", SSHURL: "git@github.com:user/archived.git",
			DefaultBranch: "main", Archived: true},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	acc := config.AccountConfig{ID: "gh-test", Provider: "github", Username: "user", CloneMethod: "ssh"}
	p := newTestProvider(t, handler, acc)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{IncludeArchived: false})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d repos, want 1 (archived excluded)", len(got))
	}
	if got[0].FullName != "user/active" {
		t.Errorf("FullName = %q, want user/active", got[0].FullName)
	}
}

func TestListRepositories_IncludesArchived(t *testing.T) {
	repos := []mockRepo{
		{ID: 1, Name: "active", FullName: "user/active", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/active.git", SSHURL: "git@github.com:user/active.git",
			DefaultBranch: "main", Archived: false},
		{ID: 2, Name: "archived", FullName: "user/archived", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/archived.git", SSHURL: "git@github.com:user/archived.git",
			DefaultBranch: "main", Archived: true},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	acc := config.AccountConfig{ID: "gh-test", Provider: "github", Username: "user", CloneMethod: "ssh"}
	p := newTestProvider(t, handler, acc)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d repos, want 2 (archived included)", len(got))
	}
}

func TestListRepositories_SSHURLUsesHostAlias(t *testing.T) {
	repos := []mockRepo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/repo1.git", SSHURL: "git@github.com:user/repo1.git",
			DefaultBranch: "main", Archived: false},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	acc := config.AccountConfig{ID: "gh-personal", Provider: "github", Username: "user", CloneMethod: "ssh"}
	p := newTestProvider(t, handler, acc)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one repo")
	}
	// SSH URL must use the host alias so the correct identity file is chosen
	want := "git@github.com-gh-personal:user/repo1.git"
	if got[0].SSHURL != want {
		t.Errorf("SSHURL = %q, want %q", got[0].SSHURL, want)
	}
}

func TestListRepositories_HTTPSCloneMethod(t *testing.T) {
	repos := []mockRepo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/repo1.git", SSHURL: "git@github.com:user/repo1.git",
			DefaultBranch: "main", Archived: false},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	acc := config.AccountConfig{ID: "gh-test", Provider: "github", Username: "user", CloneMethod: "https"}
	p := newTestProvider(t, handler, acc)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one repo")
	}
	if !strings.HasPrefix(got[0].HTTPSURL, "https://") {
		t.Errorf("HTTPSURL should start with https://, got %q", got[0].HTTPSURL)
	}
}

func TestListRepositories_SetsAccountID(t *testing.T) {
	repos := []mockRepo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/repo1.git", SSHURL: "git@github.com:user/repo1.git",
			DefaultBranch: "main", Archived: false},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	acc := config.AccountConfig{ID: "my-account-id", Provider: "github", Username: "user", CloneMethod: "ssh"}
	p := newTestProvider(t, handler, acc)

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one repo")
	}
	if got[0].AccountID != "my-account-id" {
		t.Errorf("AccountID = %q, want my-account-id", got[0].AccountID)
	}
}

func TestListRepositories_Pagination(t *testing.T) {
	page1 := []mockRepo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/repo1.git", SSHURL: "git@github.com:user/repo1.git",
			DefaultBranch: "main", Archived: false},
	}
	page2 := []mockRepo{
		{ID: 2, Name: "repo2", FullName: "user/repo2", Owner: map[string]interface{}{"login": "user"},
			CloneURL: "https://github.com/user/repo2.git", SSHURL: "git@github.com:user/repo2.git",
			DefaultBranch: "main", Archived: false},
	}

	// tsURL is captured after the server starts so the Link header contains the full URL.
	var tsURL string
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			// go-github parses the page number from the Link header URL.
			w.Header().Set("Link", `<`+tsURL+`/user/repos?page=2>; rel="next"`)
			json.NewEncoder(w).Encode(page1)
		} else {
			json.NewEncoder(w).Encode(page2)
		}
	}))
	t.Cleanup(ts.Close)
	tsURL = ts.URL

	client := gogithub.NewClient(nil)
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL
	client.UploadURL = baseURL

	acc := config.AccountConfig{ID: "gh-test", Provider: "github", Username: "user", CloneMethod: "ssh"}
	p := &GitHubProvider{
		client:      client,
		account:     acc,
		rateLimiter: rate.NewLimiter(rate.Inf, 0),
	}

	got, err := p.ListRepositories(context.Background(), provider.ListOptions{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d repos, want 2 (across 2 pages)", len(got))
	}
}
