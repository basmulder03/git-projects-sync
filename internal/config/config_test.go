package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/basmulder03/git-projects-sync/internal/config"
)

func TestExpandPath_Tilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := config.ExpandPath("~/foo/bar")
	want := filepath.Join(home, "foo", "bar")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandPath_TildeOnly(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := config.ExpandPath("~")
	if got != home {
		t.Errorf("got %q, want %q", got, home)
	}
}

func TestExpandPath_Absolute(t *testing.T) {
	got := config.ExpandPath("/absolute/path")
	if got != filepath.Clean("/absolute/path") {
		t.Errorf("got %q, want clean /absolute/path", got)
	}
}

func TestExpandPath_Relative(t *testing.T) {
	got := config.ExpandPath("relative/path")
	if got != filepath.Clean("relative/path") {
		t.Errorf("got %q, want clean relative/path", got)
	}
}

func TestLoadSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := config.DefaultConfig()
	original.General.SyncInterval = "30m"
	original.General.MaxConcurrentSyncs = 8
	original.Accounts = []config.AccountConfig{
		{ID: "gh-test", Provider: "github", Username: "user1", SSHKeyPath: "~/.ssh/test", CloneMethod: "ssh"},
	}
	original.Repositories = []config.RepoConfig{
		{AccountID: "gh-test", FullName: "user1/repo1", AutoSync: true},
	}

	if err := config.Save(original, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.General.SyncInterval != "30m" {
		t.Errorf("SyncInterval = %q, want 30m", loaded.General.SyncInterval)
	}
	if loaded.General.MaxConcurrentSyncs != 8 {
		t.Errorf("MaxConcurrentSyncs = %d, want 8", loaded.General.MaxConcurrentSyncs)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0].ID != "gh-test" {
		t.Errorf("Accounts not preserved: %+v", loaded.Accounts)
	}
	if len(loaded.Repositories) != 1 || loaded.Repositories[0].FullName != "user1/repo1" {
		t.Errorf("Repositories not preserved: %+v", loaded.Repositories)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDefaultConfig_SensibleValues(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.General.SyncInterval == "" {
		t.Error("SyncInterval should not be empty")
	}
	if cfg.General.MaxConcurrentSyncs <= 0 {
		t.Error("MaxConcurrentSyncs should be positive")
	}
	if cfg.General.WorkspaceRoot == "" {
		t.Error("WorkspaceRoot should not be empty")
	}
	if cfg.General.LogFile == "" {
		t.Error("LogFile should not be empty")
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", "config.toml")
	if err := config.Save(config.DefaultConfig(), path); err != nil {
		t.Fatalf("Save with nested path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
