---
title: Configuration
weight: 20
---

# Configuration

Config lives at `~/.git-sync/config.toml` by default. Override with `--config <path>` on any command.

Run `git-sync init` to create the file with defaults.

## Full example

```toml
[general]
  workspace_root         = "~/git-workspace"
  sync_interval          = "15m"
  log_level              = "info"
  log_file               = "~/.git-sync/git-sync.log"
  max_concurrent_syncs   = 4
  delete_merged_branches = false

[[accounts]]
  id           = "github-personal"
  provider     = "github"
  username     = "yourname"
  ssh_key_path = "~/.ssh/git-sync-github-personal"
  clone_method = "ssh"

[[accounts]]
  id           = "azdo-work"
  provider     = "azure_devops"
  organization = "myorg"
  ssh_key_path = "~/.ssh/git-sync-azdo-work"
  clone_method = "ssh"

[[repositories]]
  account_id     = "github-personal"
  full_name      = "yourname/some-repo"
  local_path     = "~/git-workspace/github/yourname/some-repo"
  auto_sync      = true
  default_branch = "main"
```

## `[general]` options

| Key | Default | Description |
|-----|---------|-------------|
| `workspace_root` | `~/git-workspace` | Root directory where repos are cloned |
| `sync_interval` | `15m` | How often the daemon syncs. Go duration string (e.g. `5m`, `1h`) |
| `log_level` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `log_file` | `~/.git-sync/git-sync.log` | Log output path. Paths starting with `~` are expanded |
| `max_concurrent_syncs` | `4` | Maximum number of repos synced in parallel |
| `delete_merged_branches` | `false` | Delete local branch after auto-checkout to default when merged |

## `[[accounts]]` options

| Key | Required | Description |
|-----|----------|-------------|
| `id` | yes | Unique identifier for this account (e.g. `github-personal`) |
| `provider` | yes | `github` or `azure_devops` |
| `username` | GitHub only | GitHub username |
| `organization` | AzDo only | Azure DevOps organisation name |
| `ssh_key_path` | yes (for SSH) | Path to the private SSH key, `~` is expanded |
| `clone_method` | `ssh` | `ssh` or `https` |

PAT tokens are **not** stored in config. Use `git-sync account set-pat <id>` to store them in the OS keychain.

## `[[repositories]]` options

| Key | Required | Description |
|-----|----------|-------------|
| `account_id` | yes | Must match an `[[accounts]]` `id` |
| `full_name` | yes | `owner/repo` for GitHub, `project/repo` for Azure DevOps |
| `local_path` | yes | Absolute local path, `~` is expanded |
| `auto_sync` | `true` | Include in daemon/`sync` runs |
| `default_branch` | auto | Default branch name. Auto-detected if empty |

Repositories are usually populated via `git-sync discover <id> --add`.

## SSH host aliases

git-sync writes `~/.ssh/config` entries in the form:

```
Host github.com-github-personal
  HostName github.com
  User git
  IdentityFile ~/.ssh/git-sync-github-personal
  IdentitiesOnly yes
```

This lets multiple GitHub accounts coexist cleanly. The alias is `<provider-host>-<account-id>`.
