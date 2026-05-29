// Package sync implements the core repository synchronisation logic.
package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/basmulder03/git-projects-sync/internal/auth"
	"github.com/basmulder03/git-projects-sync/internal/config"
	gitpkg "github.com/basmulder03/git-projects-sync/internal/git"
	"github.com/basmulder03/git-projects-sync/internal/logging"
	"github.com/basmulder03/git-projects-sync/internal/provider"
	"github.com/basmulder03/git-projects-sync/internal/registry"
)

// Syncer orchestrates syncing of all tracked repositories.
type Syncer struct {
	cfg     *config.Config
	cfgPath string
}

// New creates a Syncer backed by the given config.
func New(cfg *config.Config, cfgPath string) *Syncer {
	return &Syncer{cfg: cfg, cfgPath: cfgPath}
}

// SyncAll discovers new repos from all accounts, then syncs every tracked repository.
func (s *Syncer) SyncAll(ctx context.Context) error {
	s.discoverNewRepos(ctx)

	maxConcurrent := s.cfg.General.MaxConcurrentSyncs
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	errs := make([]error, len(s.cfg.Repositories))

	for i, repo := range s.cfg.Repositories {
		if !repo.AutoSync {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, r config.RepoConfig) {
			defer wg.Done()
			defer func() { <-sem }()
			errs[idx] = s.SyncRepo(ctx, r)
		}(i, repo)
	}

	wg.Wait()

	var firstErr error
	for _, err := range errs {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// discoverNewRepos queries each account for repositories and registers any that
// are not yet tracked. Errors per account are logged and skipped so one broken
// account does not block others.
func (s *Syncer) discoverNewRepos(ctx context.Context) {
	existing := make(map[string]bool, len(s.cfg.Repositories))
	for _, r := range s.cfg.Repositories {
		existing[r.FullName] = true
	}

	added := 0
	for _, account := range s.cfg.Accounts {
		a := auth.NewPATAuth(account.ID)
		prov, err := registry.NewProvider(account, a)
		if err != nil {
			logging.Warn("discovery: provider init failed", "account", account.ID, "err", err)
			continue
		}
		repos, err := prov.ListRepositories(ctx, provider.ListOptions{PageSize: 100})
		if err != nil {
			logging.Warn("discovery: list repos failed", "account", account.ID, "err", err)
			continue
		}
		for _, r := range repos {
			if existing[r.FullName] {
				continue
			}
			localPath := config.DeriveLocalPath(s.cfg.General.WorkspaceRoot, account.Provider, r.FullName)
			s.cfg.Repositories = append(s.cfg.Repositories, config.RepoConfig{
				AccountID:     account.ID,
				FullName:      r.FullName,
				LocalPath:     localPath,
				AutoSync:      true,
				DefaultBranch: r.DefaultBranch,
			})
			existing[r.FullName] = true
			added++
			logging.Info("discovery: new repo registered", "account", account.ID, "repo", r.FullName)
		}
	}

	if added > 0 && s.cfgPath != "" {
		if err := config.Save(s.cfg, s.cfgPath); err != nil {
			logging.Warn("discovery: save config failed", "err", err)
		}
	}
}

// buildCloneURL returns the clone URL for a repo based on its account's clone method.
func (s *Syncer) buildCloneURL(repoCfg config.RepoConfig) (string, error) {
	for _, a := range s.cfg.Accounts {
		if a.ID != repoCfg.AccountID {
			continue
		}
		if a.CloneMethod == "https" {
			switch a.Provider {
			case "github":
				return fmt.Sprintf("https://github.com/%s.git", repoCfg.FullName), nil
			case "azure_devops":
				parts := strings.SplitN(repoCfg.FullName, "/", 2)
				if len(parts) == 2 {
					return fmt.Sprintf("https://dev.azure.com/%s/_git/%s", a.Organization, parts[1]), nil
				}
			}
		}
		// Default: SSH via host alias.
		return fmt.Sprintf("git@%s:%s.git", gitpkg.SSHHostAlias(a), repoCfg.FullName), nil
	}
	return "", fmt.Errorf("account %q not found", repoCfg.AccountID)
}

// SyncRepo syncs a single repository according to the sync policy.
func (s *Syncer) SyncRepo(ctx context.Context, repoCfg config.RepoConfig) error {
	localPath := config.ExpandPath(repoCfg.LocalPath)

	if !gitpkg.IsRepo(localPath) {
		cloneURL, err := s.buildCloneURL(repoCfg)
		if err != nil {
			return fmt.Errorf("sync %s: build clone URL: %w", repoCfg.FullName, err)
		}
		logging.Info("cloning repo", "path", localPath, "url", cloneURL)
		if err := gitpkg.Clone(cloneURL, localPath); err != nil {
			return fmt.Errorf("sync %s: clone: %w", repoCfg.FullName, err)
		}
	}

	status, err := gitpkg.GetStatus(localPath)
	if err != nil {
		logging.Error("get status failed", "path", localPath, "err", err)
		return fmt.Errorf("sync %s: get status: %w", localPath, err)
	}

	if status.IsDirty {
		logging.Warn("repo is dirty, skipping", "path", localPath, "branch", status.CurrentBranch)
		return nil
	}

	if err := gitpkg.Fetch(localPath); err != nil {
		logging.Warn("fetch failed, continuing", "path", localPath, "err", err)
	}

	// Refresh status after fetch so ahead/behind counts are current.
	status, err = gitpkg.GetStatus(localPath)
	if err != nil {
		return fmt.Errorf("sync %s: get status after fetch: %w", localPath, err)
	}

	if status.IsDetached {
		logging.Warn("HEAD is detached, skipping", "path", localPath)
		return nil
	}

	defaultBranch := repoCfg.DefaultBranch
	if defaultBranch == "" {
		detected, err := gitpkg.GetDefaultBranch(localPath)
		if err != nil {
			logging.Warn("could not detect default branch, skipping sync", "path", localPath, "err", err)
			return nil
		}
		defaultBranch = detected
	}

	currentBranch := status.CurrentBranch

	if currentBranch == defaultBranch {
		if status.BehindCount > 0 {
			if err := gitpkg.PullCurrentBranch(localPath); err != nil {
				logging.Error("pull failed", "path", localPath, "branch", currentBranch, "err", err)
				return fmt.Errorf("sync %s: pull: %w", localPath, err)
			}
			logging.Info("pulled default branch", "path", localPath, "branch", currentBranch, "commits", status.BehindCount)
		} else {
			logging.Debug("default branch up to date", "path", localPath, "branch", currentBranch)
		}
		return nil
	}

	// On a non-default branch.
	if status.RemoteBranch == "" {
		logging.Info("branch has no remote, skipping branch sync", "path", localPath, "branch", currentBranch)
		// Keep default branch up to date without checkout.
		if err := gitpkg.UpdateLocalBranchFromRemote(localPath, defaultBranch); err != nil {
			logging.Warn("update default branch ref failed", "path", localPath, "branch", defaultBranch, "err", err)
		}
		return nil
	}

	merged, err := gitpkg.IsBranchMergedIntoRemoteDefault(localPath, currentBranch, defaultBranch)
	if err != nil {
		logging.Warn("merged check failed", "path", localPath, "branch", currentBranch, "err", err)
	}

	if merged {
		logging.Info("branch merged, switching to default",
			"path", localPath, "branch", currentBranch, "default", defaultBranch)

		if err := gitpkg.CheckoutBranch(localPath, defaultBranch); err != nil {
			return fmt.Errorf("sync %s: checkout default: %w", localPath, err)
		}
		if err := gitpkg.PullCurrentBranch(localPath); err != nil {
			return fmt.Errorf("sync %s: pull default: %w", localPath, err)
		}

		if s.cfg.General.DeleteMergedBranches {
			if err := gitpkg.DeleteLocalBranch(localPath, currentBranch); err != nil {
				logging.Warn("delete merged branch failed", "path", localPath, "branch", currentBranch, "err", err)
			} else {
				logging.Info("deleted merged branch", "path", localPath, "branch", currentBranch)
			}
		}
		return nil
	}

	// Branch not yet merged.
	if status.BehindCount > 0 {
		if err := gitpkg.PullCurrentBranch(localPath); err != nil {
			logging.Warn("pull feature branch failed", "path", localPath, "branch", currentBranch, "err", err)
		} else {
			logging.Info("pulled feature branch", "path", localPath, "branch", currentBranch, "commits", status.BehindCount)
		}
	}

	// Keep default branch ref current without checkout.
	if err := gitpkg.UpdateLocalBranchFromRemote(localPath, defaultBranch); err != nil {
		logging.Warn("update default branch ref failed", "path", localPath, "branch", defaultBranch, "err", err)
	}

	logging.Debug("sync complete", "path", localPath, "branch", currentBranch, "action", "none")
	return nil
}
