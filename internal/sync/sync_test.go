package sync_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/basmulder03/git-projects-sync/internal/config"
	syncer "github.com/basmulder03/git-projects-sync/internal/sync"
)

// testRepo wraps a local clone and its bare remote for sync integration tests.
type testRepo struct {
	t      *testing.T
	local  string
	remote string
}

// setupRepo creates a bare remote with one initial commit and a local clone.
func setupRepo(t *testing.T) *testRepo {
	t.Helper()

	// Populate a non-bare source repo, then mirror it as the bare remote.
	src := t.TempDir()
	mustGit(t, "", "init", src)
	gitIdentity(t, src)
	writeFile(t, filepath.Join(src, "README.md"), "init")
	mustGit(t, src, "add", ".")
	mustGitCommit(t, src, "initial")

	remote := t.TempDir()
	mustGit(t, "", "clone", "--bare", src, remote)

	local := t.TempDir()
	mustGit(t, "", "clone", remote, local)
	gitIdentity(t, local)

	return &testRepo{t: t, local: local, remote: remote}
}

// defaultBranch returns the default branch name using origin/HEAD — stable regardless
// of which branch is currently checked out.
func (r *testRepo) defaultBranch() string {
	r.t.Helper()
	out, err := exec.Command("git", "-C", r.local, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		return strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/")
	}
	for _, candidate := range []string{"master", "main"} {
		if exec.Command("git", "-C", r.local, "rev-parse", "--verify", "refs/remotes/origin/"+candidate).Run() == nil {
			return candidate
		}
	}
	r.t.Fatal("cannot determine default branch")
	return ""
}

// addRemoteCommit commits a new file to the given branch on the remote via a temp clone.
func (r *testRepo) addRemoteCommit(branch, filename, content string) {
	r.t.Helper()
	tmp := r.t.TempDir()
	mustGit(r.t, "", "clone", r.remote, tmp)
	gitIdentity(r.t, tmp)
	mustGit(r.t, tmp, "checkout", branch)
	writeFile(r.t, filepath.Join(tmp, filename), content)
	mustGit(r.t, tmp, "add", ".")
	mustGitCommit(r.t, tmp, "remote: "+filename)
	mustGit(r.t, tmp, "push", "origin", branch)
}

// createFeatureBranch creates a local feature branch with one commit and pushes it.
func (r *testRepo) createFeatureBranch(branch string) {
	r.t.Helper()
	mustGit(r.t, r.local, "checkout", "-b", branch)
	writeFile(r.t, filepath.Join(r.local, strings.ReplaceAll(branch, "/", "-")+".txt"), "feature")
	mustGit(r.t, r.local, "add", ".")
	mustGitCommit(r.t, r.local, "add "+branch)
	mustGit(r.t, r.local, "push", "-u", "origin", branch)
}

// mergeFeatureIntoDefault merges the feature branch into the default branch on the remote.
func (r *testRepo) mergeFeatureIntoDefault(featureBranch string) {
	r.t.Helper()
	def := r.defaultBranch()
	tmp := r.t.TempDir()
	mustGit(r.t, "", "clone", r.remote, tmp)
	gitIdentity(r.t, tmp)
	mustGit(r.t, tmp, "fetch", "--all")
	mustGit(r.t, tmp, "checkout", def)
	mustGit(r.t, tmp, "merge", "--no-ff", "origin/"+featureBranch, "-m", "merge "+featureBranch)
	mustGit(r.t, tmp, "push", "origin", def)
}

// makeLocalDirty writes an uncommitted file to the local repo.
func (r *testRepo) makeLocalDirty() {
	r.t.Helper()
	writeFile(r.t, filepath.Join(r.local, "dirty.txt"), "uncommitted")
}

func runSync(t *testing.T, r *testRepo, deleteMerged bool) error {
	t.Helper()
	cfg := &config.Config{
		General: config.GeneralConfig{
			MaxConcurrentSyncs:   1,
			DeleteMergedBranches: deleteMerged,
		},
	}
	s := syncer.New(cfg)
	return s.SyncRepo(context.Background(), config.RepoConfig{
		AccountID: "test",
		FullName:  "test/repo",
		LocalPath: r.local,
		AutoSync:  true,
	})
}

// --- sync policy tests ---

func TestSyncRepo_DirtyRepo_Skipped(t *testing.T) {
	r := setupRepo(t)
	r.addRemoteCommit(r.defaultBranch(), "remote.txt", "new")
	r.makeLocalDirty()

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) != before {
		t.Error("dirty repo should not be updated")
	}
}

func TestSyncRepo_MainBehindRemote_Pulled(t *testing.T) {
	r := setupRepo(t)
	r.addRemoteCommit(r.defaultBranch(), "remote.txt", "new")

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) == before {
		t.Error("local main should be updated after pull")
	}
}

func TestSyncRepo_MainUpToDate_NoOp(t *testing.T) {
	r := setupRepo(t)

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) != before {
		t.Error("up-to-date repo should not change")
	}
}

func TestSyncRepo_MergedFeatureBranch_SwitchesToDefault(t *testing.T) {
	r := setupRepo(t)
	def := r.defaultBranch()

	r.createFeatureBranch("feature/foo")
	mustGit(t, r.local, "fetch", "--all")
	r.mergeFeatureIntoDefault("feature/foo")

	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if currentBranch(t, r.local) != def {
		t.Errorf("should switch to %q, got %q", def, currentBranch(t, r.local))
	}
}

func TestSyncRepo_MergedFeatureBranch_DefaultBranchUpdated(t *testing.T) {
	r := setupRepo(t)

	// Add a remote commit to default branch before creating the feature branch
	r.addRemoteCommit(r.defaultBranch(), "base.txt", "base")
	mustGit(t, r.local, "pull")

	r.createFeatureBranch("feature/bar")
	mustGit(t, r.local, "fetch", "--all")
	r.mergeFeatureIntoDefault("feature/bar")

	remoteSHA := gitOutput(t, r.remote, "rev-parse", "HEAD")
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	// After switching to default and pulling, local HEAD should match remote
	if headSHA(t, r.local) != remoteSHA {
		t.Error("local default branch should be at same SHA as remote after sync")
	}
}

func TestSyncRepo_MergedFeatureBranch_DeletesWhenConfigured(t *testing.T) {
	r := setupRepo(t)

	r.createFeatureBranch("feature/todelete")
	mustGit(t, r.local, "fetch", "--all")
	r.mergeFeatureIntoDefault("feature/todelete")

	if err := runSync(t, r, true); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}

	// Branch should be gone
	cmd := exec.Command("git", "-C", r.local, "rev-parse", "--verify", "feature/todelete")
	if cmd.Run() == nil {
		t.Error("merged branch should be deleted when DeleteMergedBranches=true")
	}
}

func TestSyncRepo_MergedFeatureBranch_KeepsWhenNotConfigured(t *testing.T) {
	r := setupRepo(t)

	r.createFeatureBranch("feature/tokeep")
	mustGit(t, r.local, "fetch", "--all")
	r.mergeFeatureIntoDefault("feature/tokeep")

	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}

	// Branch should still exist
	cmd := exec.Command("git", "-C", r.local, "rev-parse", "--verify", "feature/tokeep")
	if cmd.Run() != nil {
		t.Error("merged branch should be kept when DeleteMergedBranches=false")
	}
}

func TestSyncRepo_FeatureBranchBehindRemote_Pulled(t *testing.T) {
	r := setupRepo(t)

	r.createFeatureBranch("feature/behind")
	r.addRemoteCommit("feature/behind", "more.txt", "extra")

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) == before {
		t.Error("feature branch should be updated")
	}
	if currentBranch(t, r.local) != "feature/behind" {
		t.Error("should remain on feature branch")
	}
}

func TestSyncRepo_FeatureBranchUpToDate_NoOp(t *testing.T) {
	r := setupRepo(t)
	r.createFeatureBranch("feature/current")

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) != before {
		t.Error("up-to-date feature branch should not change")
	}
	if currentBranch(t, r.local) != "feature/current" {
		t.Error("should remain on feature branch")
	}
}

func TestSyncRepo_UnpublishedBranch_NotTouched(t *testing.T) {
	r := setupRepo(t)

	// Create local branch without pushing
	mustGit(t, r.local, "checkout", "-b", "local-only")
	writeFile(t, filepath.Join(r.local, "local.txt"), "local only")
	mustGit(t, r.local, "add", ".")
	mustGitCommit(t, r.local, "local only commit")

	before := headSHA(t, r.local)
	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}
	if headSHA(t, r.local) != before {
		t.Error("unpublished branch should not be modified")
	}
	if currentBranch(t, r.local) != "local-only" {
		t.Error("should remain on local-only branch")
	}
}

func TestSyncRepo_UnpublishedBranch_DefaultBranchUpdated(t *testing.T) {
	r := setupRepo(t)
	def := r.defaultBranch()

	// Add remote commit to default before switching to unpublished branch
	r.addRemoteCommit(def, "remote.txt", "remote work")

	mustGit(t, r.local, "checkout", "-b", "local-only")

	if err := runSync(t, r, false); err != nil {
		t.Fatalf("SyncRepo: %v", err)
	}

	// local-only branch should not move, but local default branch ref should advance
	localDefaultSHA := gitOutput(t, r.local, "rev-parse", def)
	remoteSHA := gitOutput(t, r.remote, "rev-parse", def)
	if localDefaultSHA != remoteSHA {
		t.Errorf("local %s ref should be updated without checkout: local=%s remote=%s",
			def, localDefaultSHA[:8], remoteSHA[:8])
	}
}

func TestSyncRepo_NotCloned_AttemptClone(t *testing.T) {
	// Sync now attempts to clone missing repos; without a matching account it errors.
	cfg := &config.Config{General: config.GeneralConfig{MaxConcurrentSyncs: 1}}
	s := syncer.New(cfg)
	err := s.SyncRepo(context.Background(), config.RepoConfig{
		AccountID: "test",
		FullName:  "test/repo",
		LocalPath: filepath.Join(t.TempDir(), "does-not-exist"),
		AutoSync:  true,
	})
	if err == nil {
		t.Error("expected error when account not found for clone, got nil")
	}
}

func TestSyncAll_SkipsAutoSyncFalse(t *testing.T) {
	r := setupRepo(t)
	r.addRemoteCommit(r.defaultBranch(), "remote.txt", "new")

	before := headSHA(t, r.local)
	cfg := &config.Config{
		General: config.GeneralConfig{MaxConcurrentSyncs: 1},
		Repositories: []config.RepoConfig{
			{AccountID: "test", FullName: "test/repo", LocalPath: r.local, AutoSync: false},
		},
	}
	s := syncer.New(cfg)
	if err := s.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	if headSHA(t, r.local) != before {
		t.Error("repo with AutoSync=false should not be updated")
	}
}

// --- test helpers ---

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func mustGitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit %q: %v\n%s", msg, err, out)
	}
}

func gitIdentity(t *testing.T, dir string) {
	t.Helper()
	mustGit(t, dir, "config", "user.name", "Test")
	mustGit(t, dir, "config", "user.email", "test@test.com")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return gitOutput(t, dir, "rev-parse", "HEAD")
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
