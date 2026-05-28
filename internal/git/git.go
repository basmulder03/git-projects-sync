package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const cmdTimeout = 30 * time.Second

// Repo holds the identity of a local Git repository.
type Repo struct {
	LocalPath     string
	DefaultBranch string
	AccountID     string
}

// RepoStatus describes the current state of a local repository.
type RepoStatus struct {
	CurrentBranch   string
	IsDirty         bool
	IsDetached      bool
	LocalBranch     string
	RemoteBranch    string
	AheadCount      int
	BehindCount     int
	DefaultBranch   string
	IsDefaultBranch bool
}

// IsRepo returns true when path contains a .git directory or is inside a git repo.
func IsRepo(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// Clone clones a remote repository to localPath.
func Clone(sshOrHTTPSURL, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return run(ctx, "", "git", "clone", sshOrHTTPSURL, localPath)
}

// GetStatus returns the current status of the repository at localPath.
func GetStatus(localPath string) (*RepoStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	status := &RepoStatus{}

	// Dirty check.
	out, err := output(ctx, localPath, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	status.IsDirty = strings.TrimSpace(out) != ""

	// Current branch / detached HEAD.
	branchOut, err := output(ctx, localPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	branch := strings.TrimSpace(branchOut)
	status.CurrentBranch = branch
	status.LocalBranch = branch
	status.IsDetached = branch == "HEAD"

	// Tracking remote branch.
	remoteOut, _ := output(ctx, localPath, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	status.RemoteBranch = strings.TrimSpace(remoteOut)

	// Ahead / behind counts (only meaningful when tracking remote exists).
	if status.RemoteBranch != "" {
		behindOut, _ := output(ctx, localPath, "git", "rev-list", "--count", "HEAD..@{u}")
		aheadOut, _ := output(ctx, localPath, "git", "rev-list", "--count", "@{u}..HEAD")
		status.BehindCount, _ = strconv.Atoi(strings.TrimSpace(behindOut))
		status.AheadCount, _ = strconv.Atoi(strings.TrimSpace(aheadOut))
	}

	return status, nil
}

// Fetch fetches all remotes, updating remote tracking refs.
func Fetch(localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return run(ctx, localPath, "git", "fetch", "--all", "--prune")
}

// PullCurrentBranch performs a fast-forward-only pull on the current branch.
func PullCurrentBranch(localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return run(ctx, localPath, "git", "pull", "--ff-only")
}

// CheckoutBranch checks out the named branch.
func CheckoutBranch(localPath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return run(ctx, localPath, "git", "checkout", branch)
}

// IsBranchMergedIntoRemoteDefault reports whether origin/{branch} is merged into origin/{defaultBranch}.
func IsBranchMergedIntoRemoteDefault(localPath, branch, defaultBranch string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := output(ctx, localPath, "git", "branch", "-r", "--merged",
		fmt.Sprintf("origin/%s", defaultBranch))
	if err != nil {
		return false, fmt.Errorf("git branch --merged: %w", err)
	}

	target := "origin/" + branch
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == target {
			return true, nil
		}
	}
	return false, nil
}

// UpdateLocalBranchFromRemote fast-forwards a local branch ref from its remote counterpart
// without requiring a checkout.
func UpdateLocalBranchFromRemote(localPath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	ref := fmt.Sprintf("%s:%s", branch, branch)
	return run(ctx, localPath, "git", "fetch", "origin", ref)
}

// GetDefaultBranch detects the default branch, trying "main" then "master".
func GetDefaultBranch(localPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	// Prefer remote HEAD symbolic ref.
	out, err := output(ctx, localPath, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(out)
		if after, ok := strings.CutPrefix(ref, "origin/"); ok {
			return after, nil
		}
		return ref, nil
	}

	for _, candidate := range []string{"main", "master"} {
		checkCtx, checkCancel := context.WithTimeout(context.Background(), cmdTimeout)
		err := run(checkCtx, localPath, "git", "rev-parse", "--verify", "origin/"+candidate)
		checkCancel()
		if err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not detect default branch in %s", filepath.Base(localPath))
}

// DeleteLocalBranch deletes a local branch (safe delete, must be merged).
func DeleteLocalBranch(localPath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return run(ctx, localPath, "git", "branch", "-d", branch)
}

// --- helpers ---

func run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w\n%s", name, args, err, buf.String())
	}
	return nil
}

func output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
