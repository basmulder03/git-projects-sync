---
title: sync
weight: 6
---

# sync

Run a one-shot sync of all (or one) tracked repositories.

```bash
git-sync sync [flags]
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Print what would be synced without doing it |
| `--repo <name\|path>` | Sync only this repository (full name or local path) |

## Sync policy

For each repository with `auto_sync = true`:

1. **Skip** if local path is not a git repo (not yet cloned)
2. **Skip** if working tree is dirty
3. Fetch from remote
4. If on **default branch** and behind: fast-forward pull
5. If on a **feature branch**:
   - Check if the branch has been merged into the remote default branch
   - If merged: checkout default branch and pull, optionally delete the local branch
   - If not merged and behind: pull the feature branch
   - Always update the default branch ref without checking it out

Repos are synced concurrently up to `max_concurrent_syncs`.

## Example

```bash
# Dry run
git-sync sync --dry-run

# Sync one specific repo
git-sync sync --repo yourname/some-repo
```
