package sync_test

import (
	"context"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/basmulder03/git-projects-sync/internal/config"
	gitpkg "github.com/basmulder03/git-projects-sync/internal/git"
	syncer "github.com/basmulder03/git-projects-sync/internal/sync"
)

// findGitHTTPBackend returns the path to git-http-backend, or "" if unavailable.
func findGitHTTPBackend(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return ""
	}
	base := filepath.Join(strings.TrimSpace(string(out)), "git-http-backend")
	for _, candidate := range []string{base, base + ".exe"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// TestCloneAndSync_OverHTTP spins up a real git-http-backend server in-process,
// clones over HTTP, adds a commit on the remote, then verifies SyncAll pulls it.
func TestCloneAndSync_OverHTTP(t *testing.T) {
	backendPath := findGitHTTPBackend(t)
	if backendPath == "" {
		t.Skip("git-http-backend not found; skipping HTTP E2E clone test")
	}

	// Bare remote lives under projectRoot so git-http-backend can locate it via PATH_INFO.
	projectRoot := t.TempDir()
	const bareName = "testrepo.git"
	bareDir := filepath.Join(projectRoot, bareName)
	mustGit(t, "", "init", "--bare", bareDir)

	// Push an initial commit from a scratch source repo.
	src := t.TempDir()
	mustGit(t, "", "init", src)
	gitIdentity(t, src)
	mustGit(t, src, "remote", "add", "origin", bareDir)
	writeFile(t, filepath.Join(src, "README.md"), "hello")
	mustGit(t, src, "add", ".")
	mustGitCommit(t, src, "initial commit")
	defaultBranch := gitOutput(t, src, "rev-parse", "--abbrev-ref", "HEAD")
	mustGit(t, src, "push", "origin", "HEAD:"+defaultBranch)

	// Serve the bare repo via git-http-backend CGI (no extra deps).
	ts := httptest.NewServer(&cgi.Handler{
		Path: backendPath,
		Env: []string{
			"GIT_HTTP_EXPORT_ALL=1",
			"GIT_PROJECT_ROOT=" + projectRoot,
		},
		// Inherit PATH so git-http-backend can find git itself.
		InheritEnv: []string{"PATH", "SYSTEMROOT", "HOME"},
	})
	t.Cleanup(ts.Close)

	cloneURL := ts.URL + "/" + bareName

	// ── Step 1: Clone over HTTP ──────────────────────────────────────────────
	local := t.TempDir()
	if err := gitpkg.Clone(cloneURL, local); err != nil {
		t.Fatalf("Clone over HTTP: %v", err)
	}
	if !gitpkg.IsRepo(local) {
		t.Fatal("cloned dir is not a git repo")
	}
	data, err := os.ReadFile(filepath.Join(local, "README.md"))
	if err != nil || strings.TrimSpace(string(data)) != "hello" {
		t.Fatalf("README.md after clone = %q, want 'hello'", string(data))
	}

	// ── Step 2: Push a new commit to the remote ──────────────────────────────
	writeFile(t, filepath.Join(src, "README.md"), "updated")
	mustGit(t, src, "add", ".")
	mustGitCommit(t, src, "second commit")
	mustGit(t, src, "push", "origin", "HEAD:"+defaultBranch)

	// ── Step 3: SyncAll should pull the update ───────────────────────────────
	cfg := &config.Config{
		General: config.GeneralConfig{MaxConcurrentSyncs: 1},
		Repositories: []config.RepoConfig{
			{LocalPath: local, AutoSync: true},
		},
	}
	s := syncer.New(cfg)
	if err := s.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll after push: %v", err)
	}

	data, err = os.ReadFile(filepath.Join(local, "README.md"))
	if err != nil || strings.TrimSpace(string(data)) != "updated" {
		t.Fatalf("README.md after sync = %q, want 'updated'", string(data))
	}

	// Verify local HEAD matches remote HEAD (fully caught up).
	localSHA := headSHA(t, local)
	remoteSHA := gitOutput(t, bareDir, "rev-parse", "HEAD")
	if localSHA != remoteSHA {
		t.Errorf("local HEAD %s != remote HEAD %s after sync", localSHA[:8], remoteSHA[:8])
	}
}

// TestCloneOverHTTP_ContentIntegrity verifies file contents after cloning.
func TestCloneOverHTTP_ContentIntegrity(t *testing.T) {
	backendPath := findGitHTTPBackend(t)
	if backendPath == "" {
		t.Skip("git-http-backend not found; skipping HTTP clone content test")
	}

	projectRoot := t.TempDir()
	const bareName = "content.git"
	bareDir := filepath.Join(projectRoot, bareName)
	mustGit(t, "", "init", "--bare", bareDir)

	src := t.TempDir()
	mustGit(t, "", "init", src)
	gitIdentity(t, src)
	mustGit(t, src, "remote", "add", "origin", bareDir)

	// Multiple files to verify all transferred correctly.
	writeFile(t, filepath.Join(src, "a.txt"), "alpha")
	writeFile(t, filepath.Join(src, "b.txt"), "beta")
	mustGit(t, src, "add", ".")
	mustGitCommit(t, src, "add files")
	branch := gitOutput(t, src, "rev-parse", "--abbrev-ref", "HEAD")
	mustGit(t, src, "push", "origin", "HEAD:"+branch)

	ts := httptest.NewServer(&cgi.Handler{
		Path:       backendPath,
		Env:        []string{"GIT_HTTP_EXPORT_ALL=1", "GIT_PROJECT_ROOT=" + projectRoot},
		InheritEnv: []string{"PATH", "SYSTEMROOT", "HOME"},
	})
	t.Cleanup(ts.Close)

	local := t.TempDir()
	if err := gitpkg.Clone(ts.URL+"/"+bareName, local); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	for file, want := range map[string]string{"a.txt": "alpha", "b.txt": "beta"} {
		data, err := os.ReadFile(filepath.Join(local, file))
		if err != nil {
			t.Errorf("read %s: %v", file, err)
			continue
		}
		if got := strings.TrimSpace(string(data)); got != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
}

// TestHTTPServer_RejectsUnknownRepo verifies the server returns 4xx for unknown paths.
func TestHTTPServer_RejectsUnknownRepo(t *testing.T) {
	backendPath := findGitHTTPBackend(t)
	if backendPath == "" {
		t.Skip("git-http-backend not found")
	}

	projectRoot := t.TempDir() // empty — no repos
	ts := httptest.NewServer(&cgi.Handler{
		Path:       backendPath,
		Env:        []string{"GIT_HTTP_EXPORT_ALL=1", "GIT_PROJECT_ROOT=" + projectRoot},
		InheritEnv: []string{"PATH", "SYSTEMROOT", "HOME"},
	})
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/nonexistent.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		t.Errorf("expected 4xx for unknown repo, got %d", resp.StatusCode)
	}
}
