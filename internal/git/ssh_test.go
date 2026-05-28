package git

import (
	"strings"
	"testing"

	"github.com/basmulder03/git-projects-sync/internal/config"
)

// --- replaceMarkedBlock ---

func TestReplaceMarkedBlock_AppendToEmpty(t *testing.T) {
	block := "Host example\n    HostName example.com\n"
	result := replaceMarkedBlock("", "acct1", block)

	assertContains(t, result, "# git-sync-begin:acct1")
	assertContains(t, result, "# git-sync-end:acct1")
	assertContains(t, result, block)
}

func TestReplaceMarkedBlock_AppendToExisting(t *testing.T) {
	existing := "# other config\nHost unrelated\n    HostName other.com\n"
	block := "Host new\n    HostName new.com\n"
	result := replaceMarkedBlock(existing, "acct1", block)

	assertContains(t, result, "Host unrelated")
	assertContains(t, result, "# git-sync-begin:acct1")
	assertContains(t, result, block)
}

func TestReplaceMarkedBlock_UpdateExisting(t *testing.T) {
	old := "# git-sync-begin:acct1\nHost old\n    HostName old.com\n# git-sync-end:acct1\n"
	newBlock := "Host new\n    HostName new.com\n"
	result := replaceMarkedBlock(old, "acct1", newBlock)

	if strings.Contains(result, "old.com") {
		t.Error("old block content should be replaced")
	}
	assertContains(t, result, "new.com")
	assertContains(t, result, "# git-sync-begin:acct1")
	assertContains(t, result, "# git-sync-end:acct1")
}

func TestReplaceMarkedBlock_UpdatePreservesNeighbours(t *testing.T) {
	content := "before\n# git-sync-begin:acct1\nHost old\n# git-sync-end:acct1\nafter\n"
	result := replaceMarkedBlock(content, "acct1", "Host new\n")

	assertContains(t, result, "before")
	assertContains(t, result, "after")
	assertContains(t, result, "Host new")
	if strings.Contains(result, "Host old") {
		t.Error("old host should be replaced")
	}
}

func TestReplaceMarkedBlock_RemoveBlock(t *testing.T) {
	content := "before\n# git-sync-begin:acct1\nHost foo\n# git-sync-end:acct1\nafter\n"
	result := replaceMarkedBlock(content, "acct1", "")

	if strings.Contains(result, "git-sync") {
		t.Error("markers should be removed")
	}
	if strings.Contains(result, "Host foo") {
		t.Error("block content should be removed")
	}
	assertContains(t, result, "before")
	assertContains(t, result, "after")
}

func TestReplaceMarkedBlock_RemoveNonExistent_NoOp(t *testing.T) {
	content := "some config\n"
	result := replaceMarkedBlock(content, "nonexistent", "")
	if result != content {
		t.Errorf("removing non-existent block should be a no-op, got %q", result)
	}
}

func TestReplaceMarkedBlock_MultipleAccounts(t *testing.T) {
	content := "# git-sync-begin:acct1\nHost a\n# git-sync-end:acct1\n# git-sync-begin:acct2\nHost b\n# git-sync-end:acct2\n"
	result := replaceMarkedBlock(content, "acct1", "Host updated-a\n")

	assertContains(t, result, "Host updated-a")
	assertContains(t, result, "Host b")
	if strings.Contains(result, "Host a\n") {
		t.Error("acct1 block should be replaced")
	}
}

// --- buildSSHBlock ---

func TestBuildSSHBlock_GitHub(t *testing.T) {
	acc := config.AccountConfig{ID: "gh-personal", Provider: "github", SSHKeyPath: "~/.ssh/github_personal"}
	block := buildSSHBlock(acc)

	assertContains(t, block, "Host github.com-gh-personal")
	assertContains(t, block, "HostName github.com")
	assertContains(t, block, "User git")
	assertContains(t, block, "IdentitiesOnly yes")
}

func TestBuildSSHBlock_AzureDevOps(t *testing.T) {
	acc := config.AccountConfig{ID: "azdo-work", Provider: "azure_devops", SSHKeyPath: "~/.ssh/azdo"}
	block := buildSSHBlock(acc)

	assertContains(t, block, "Host ssh.dev.azure.com-azdo-work")
	assertContains(t, block, "HostName ssh.dev.azure.com")
}

func TestBuildSSHBlock_UnknownProvider(t *testing.T) {
	acc := config.AccountConfig{ID: "other-acct", Provider: "gitea", SSHKeyPath: "~/.ssh/gitea"}
	block := buildSSHBlock(acc)

	assertContains(t, block, "Host gitea-other-acct")
	assertContains(t, block, "HostName gitea")
}

// --- SSHHostAlias ---

func TestSSHHostAlias(t *testing.T) {
	tests := []struct {
		account config.AccountConfig
		want    string
	}{
		{config.AccountConfig{ID: "p1", Provider: "github"}, "github.com-p1"},
		{config.AccountConfig{ID: "w1", Provider: "azure_devops"}, "ssh.dev.azure.com-w1"},
		{config.AccountConfig{ID: "g1", Provider: "gitea"}, "gitea-g1"},
	}
	for _, tc := range tests {
		got := SSHHostAlias(tc.account)
		if got != tc.want {
			t.Errorf("SSHHostAlias(%+v) = %q, want %q", tc.account, got, tc.want)
		}
	}
}

// --- isSSHAuthSuccess ---

func TestIsSSHAuthSuccess(t *testing.T) {
	tests := []struct {
		provider string
		output   string
		want     bool
	}{
		{"github", "Hi johndoe! You've successfully authenticated, but GitHub does not provide shell access.", true},
		{"github", "Hi org-name! You've successfully authenticated", true},
		{"github", "Permission denied (publickey).", false},
		{"github", "Host key verification failed.", false},
		{"github", "", false},
		{"azure_devops", "remote: Shell access is not supported.", true},
		{"azure_devops", "Shell access is not supported", true},
		{"azure_devops", "Permission denied (publickey).", false},
		{"unknown", "You've successfully authenticated", true},
		{"unknown", "error", false},
	}
	for _, tc := range tests {
		got := isSSHAuthSuccess(tc.provider, tc.output)
		if got != tc.want {
			t.Errorf("isSSHAuthSuccess(%q, %q) = %v, want %v", tc.provider, tc.output, got, tc.want)
		}
	}
}

// --- PublicKeyPath ---

func TestPublicKeyPath(t *testing.T) {
	got := PublicKeyPath("~/.ssh/mykey")
	if !strings.HasSuffix(got, ".pub") {
		t.Errorf("PublicKeyPath should end with .pub, got %q", got)
	}
}

// --- ProviderKeyURL ---

func TestProviderKeyURL(t *testing.T) {
	ghAcc := config.AccountConfig{ID: "p", Provider: "github"}
	if url := ProviderKeyURL(ghAcc); !strings.Contains(url, "github.com") {
		t.Errorf("GitHub URL should contain github.com, got %q", url)
	}

	azdoAcc := config.AccountConfig{ID: "w", Provider: "azure_devops", Organization: "myorg"}
	if url := ProviderKeyURL(azdoAcc); !strings.Contains(url, "myorg") {
		t.Errorf("AzDo URL should contain org name, got %q", url)
	}
}

// --- helpers ---

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}
