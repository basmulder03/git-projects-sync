package git

import (
	"os"
	"path/filepath"
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

// --- SSH config file I/O ---

func tmpSSHConfig(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".ssh", "config")
}

func TestEnsureSSHEntry_CreatesFileAndDir(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc := config.AccountConfig{ID: "gh-io", Provider: "github", SSHKeyPath: "~/.ssh/test_io"}

	if err := ensureSSHEntryAt(acc, cfgPath); err != nil {
		t.Fatalf("ensureSSHEntryAt: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	assertContains(t, content, "Host github.com-gh-io")
	assertContains(t, content, "# git-sync-begin:gh-io")
	assertContains(t, content, "# git-sync-end:gh-io")
}

func TestEnsureSSHEntry_UpdatesInPlace(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc := config.AccountConfig{ID: "upd", Provider: "github", SSHKeyPath: "~/.ssh/old"}
	if err := ensureSSHEntryAt(acc, cfgPath); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	acc.SSHKeyPath = "~/.ssh/new"
	if err := ensureSSHEntryAt(acc, cfgPath); err != nil {
		t.Fatalf("update: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if strings.Contains(content, "old") {
		t.Error("old key path should be replaced")
	}
	assertContains(t, content, "new")

	// Exactly one begin marker — no duplicate entries
	if strings.Count(content, "# git-sync-begin:upd") != 1 {
		t.Error("should have exactly one begin marker after update")
	}
}

func TestEnsureSSHEntry_TwoAccountsCoexist(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc1 := config.AccountConfig{ID: "acc1", Provider: "github", SSHKeyPath: "~/.ssh/k1"}
	acc2 := config.AccountConfig{ID: "acc2", Provider: "azure_devops", SSHKeyPath: "~/.ssh/k2"}

	if err := ensureSSHEntryAt(acc1, cfgPath); err != nil {
		t.Fatalf("acc1: %v", err)
	}
	if err := ensureSSHEntryAt(acc2, cfgPath); err != nil {
		t.Fatalf("acc2: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	assertContains(t, content, "github.com-acc1")
	assertContains(t, content, "ssh.dev.azure.com-acc2")
}

func TestRemoveSSHEntry_RemovesBlock(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc := config.AccountConfig{ID: "gone", Provider: "github", SSHKeyPath: "~/.ssh/gone"}
	if err := ensureSSHEntryAt(acc, cfgPath); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := removeSSHEntryAt("gone", cfgPath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), "gone") {
		t.Error("entry should be fully removed from file")
	}
}

func TestRemoveSSHEntry_PreservesOtherAccounts(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	ensureSSHEntryAt(config.AccountConfig{ID: "keep", Provider: "github", SSHKeyPath: "~/.ssh/k"}, cfgPath)   //nolint
	ensureSSHEntryAt(config.AccountConfig{ID: "delete", Provider: "github", SSHKeyPath: "~/.ssh/d"}, cfgPath) //nolint

	if err := removeSSHEntryAt("delete", cfgPath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	assertContains(t, content, "# git-sync-begin:keep")
	if strings.Contains(content, "delete") {
		t.Error("deleted entry should not remain")
	}
}

func TestSSHConfigEntryExistsAt(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc := config.AccountConfig{ID: "exists", Provider: "github", SSHKeyPath: "~/.ssh/e"}
	ensureSSHEntryAt(acc, cfgPath) //nolint

	ok, err := sshConfigEntryExistsAt("exists", cfgPath)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !ok {
		t.Error("should exist")
	}

	ok2, _ := sshConfigEntryExistsAt("missing", cfgPath)
	if ok2 {
		t.Error("missing account should not exist")
	}
}

func TestSSHEntry_AtomicWrite_TempFileCleanedUp(t *testing.T) {
	cfgPath := tmpSSHConfig(t)
	acc := config.AccountConfig{ID: "atomic", Provider: "github", SSHKeyPath: "~/.ssh/a"}
	if err := ensureSSHEntryAt(acc, cfgPath); err != nil {
		t.Fatalf("write: %v", err)
	}

	// No .tmp file should remain after atomic write
	if _, err := os.Stat(cfgPath + ".tmp"); err == nil {
		t.Error("temp file should be cleaned up after atomic write")
	}
}

// --- helpers ---

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}
