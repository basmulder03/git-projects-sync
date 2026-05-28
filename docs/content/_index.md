---
title: git-projects-sync
type: docs
---

# git-projects-sync

A lightweight Go daemon that keeps your GitHub and Azure DevOps repositories in sync across multiple accounts — safely, without ever touching dirty working trees.

## What it does

`git-sync` runs as a background daemon on a configurable interval. For each tracked repository it:

1. Skips the repo if the working tree is dirty
2. Fetches from remote
3. Fast-forward pulls if behind on the default branch
4. Detects when a feature branch has been merged and auto-checks out the default branch
5. Updates the default branch ref in the background even when you're on a feature branch

PAT tokens are stored exclusively in the OS keychain — never in the config file. Each account uses an isolated SSH key via `~/.ssh/config` host aliases so multiple GitHub accounts can coexist cleanly.

## Quick start

```bash
# Install binary and register autostart
git-sync install

# Add your first account (wizard will run automatically)
git-sync account add --provider github --id github-personal --username yourname --set-pat

# Generate SSH key
git-sync ssh generate github-personal

# Discover and track repos
git-sync discover github-personal --add

# One-shot sync
git-sync sync
```

## Navigation

- [Installation]({{< relref "docs/installation" >}}) — install, autostart, build from source
- [Configuration]({{< relref "docs/configuration" >}}) — all config options explained
- [Commands]({{< relref "docs/commands" >}}) — full command reference
