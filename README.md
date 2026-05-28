# git-projects-sync

A lightweight Go daemon that keeps your GitHub and Azure DevOps repositories in sync across multiple accounts — safely, without ever touching dirty working trees.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-github.io-blue)](https://basmulder03.github.io/git-projects-sync/)

---

## Features

- **Multi-account** — manage GitHub and Azure DevOps accounts side by side
- **SSH isolation** — each account gets its own SSH key via `~/.ssh/config` host aliases
- **Secure secrets** — PAT tokens stored in OS keychain only (Windows Credential Manager / libsecret / macOS Keychain)
- **Safe sync policy** — skips dirty repos, fast-forward-only pulls, auto-checkout default branch when feature branch is merged
- **Background daemon** — runs on a configurable interval, registers as a login autostart item
- **One-shot sync** — run `git-sync sync` any time for an immediate pass

## Installation

**Windows** (PowerShell):

```powershell
irm https://github.com/basmulder03/git-projects-sync/releases/latest/download/install.ps1 | iex
```

**Linux / macOS**:

```bash
curl -fsSL https://github.com/basmulder03/git-projects-sync/releases/latest/download/install.sh | sh
```

Both scripts download the latest release binary, place it on your PATH, and run `git-sync install` to register the autostart daemon.

Alternatively, install via Go:

```bash
go install github.com/basmulder03/git-projects-sync/cmd/git-sync@latest
git-sync install
```

Or download a pre-built binary manually from the [Releases](https://github.com/basmulder03/git-projects-sync/releases) page.

See the [Installation guide](https://basmulder03.github.io/git-projects-sync/docs/installation/) for full details.

## Quick start

```bash
# 1. Install and create default config
git-sync install

# 2. Add a GitHub account
git-sync account add --provider github --id github-personal --username yourname --set-pat

# 3. Generate an SSH key for that account
git-sync ssh generate github-personal

# 4. Discover repositories and add them to config
git-sync discover github-personal --add

# 5. Run a one-shot sync
git-sync sync

# 6. Start the background daemon
git-sync daemon start
```

## Configuration

Config lives at `~/.git-sync/config.toml`:

```toml
[general]
  workspace_root       = "~/git-workspace"
  sync_interval        = "15m"
  log_level            = "info"
  log_file             = "~/.git-sync/git-sync.log"
  max_concurrent_syncs = 4
  delete_merged_branches = false

[[accounts]]
  id           = "github-personal"
  provider     = "github"
  username     = "yourname"
  ssh_key_path = "~/.ssh/git-sync-github-personal"
  clone_method = "ssh"

[[repositories]]
  account_id     = "github-personal"
  full_name      = "yourname/some-repo"
  local_path     = "~/git-workspace/github/yourname/some-repo"
  auto_sync      = true
  default_branch = "main"
```

See the [Configuration reference](https://basmulder03.github.io/git-projects-sync/docs/configuration/) for all options.

## Commands

| Command | Description |
|---------|-------------|
| `git-sync init` | Create default config file |
| `git-sync install` | Install binary and register autostart |
| `git-sync account add` | Add a provider account |
| `git-sync account set-pat` | Store a PAT in the OS keychain |
| `git-sync discover <id>` | List repositories for an account |
| `git-sync repo list` | List tracked repositories |
| `git-sync sync` | Run a one-shot sync |
| `git-sync status` | Show status of all tracked repos |
| `git-sync daemon start/stop/status` | Manage the background daemon |
| `git-sync ssh generate <id>` | Generate an SSH key pair |
| `git-sync ssh test <id>` | Test SSH connectivity |
| `git-sync logs` | Tail the log file |

Full command reference: [basmulder03.github.io/git-projects-sync/docs/commands/](https://basmulder03.github.io/git-projects-sync/docs/commands/)

## Building from source

```bash
git clone https://github.com/basmulder03/git-projects-sync.git
cd git-projects-sync
go build ./cmd/git-sync
```

Run tests:

```bash
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
