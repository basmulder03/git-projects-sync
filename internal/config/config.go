// Package config handles loading, saving, and defaulting of TOML configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration structure.
type Config struct {
	General      GeneralConfig   `toml:"general"`
	Accounts     []AccountConfig `toml:"accounts"`
	Repositories []RepoConfig    `toml:"repositories"`
}

// GeneralConfig holds daemon and workspace settings.
type GeneralConfig struct {
	WorkspaceRoot        string `toml:"workspace_root"`
	SyncInterval         string `toml:"sync_interval"`
	LogLevel             string `toml:"log_level"`
	LogFile              string `toml:"log_file"`
	MaxConcurrentSyncs   int    `toml:"max_concurrent_syncs"`
	DeleteMergedBranches bool   `toml:"delete_merged_branches"`
}

// AccountConfig describes a remote provider account.
type AccountConfig struct {
	ID           string `toml:"id"`
	Provider     string `toml:"provider"`
	Username     string `toml:"username"`
	Organization string `toml:"organization"`
	SSHKeyPath   string `toml:"ssh_key_path"`
	CloneMethod  string `toml:"clone_method"`
}

// RepoConfig describes a single tracked repository.
type RepoConfig struct {
	AccountID     string `toml:"account_id"`
	FullName      string `toml:"full_name"`
	LocalPath     string `toml:"local_path"`
	AutoSync      bool   `toml:"auto_sync"`
	DefaultBranch string `toml:"default_branch"`
}

// DefaultConfigPath returns the OS-appropriate default config path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".git-sync", "config.toml")
	}
	return filepath.Join(home, ".git-sync", "config.toml")
}

// DefaultConfig returns a sensible default Config.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	workspace := filepath.Join(home, "git-workspace")
	logFile := filepath.Join(home, ".git-sync", "git-sync.log")

	return &Config{
		General: GeneralConfig{
			WorkspaceRoot:        workspace,
			SyncInterval:         "15m",
			LogLevel:             "info",
			LogFile:              logFile,
			MaxConcurrentSyncs:   4,
			DeleteMergedBranches: false,
		},
	}
}

// Load reads and parses a TOML config from path.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	return cfg, nil
}

// Save encodes cfg to TOML and writes it to path, creating parent dirs as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// DeriveLocalPath builds the local clone path for a repo under workspaceRoot.
// Layout: <workspaceRoot>/<provider>/<owner>/<repo>
func DeriveLocalPath(workspaceRoot, provider, fullName string) string {
	base := ExpandPath(workspaceRoot)
	parts := strings.Split(fullName, "/")
	segments := append([]string{base, provider}, parts...)
	return filepath.Join(segments...)
}

// ExpandPath expands a leading ~ to the user home directory and cleans the path.
func ExpandPath(p string) string {
	if !strings.HasPrefix(p, "~") {
		return filepath.Clean(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(home, p[1:]))
}
