---
title: Commands
weight: 30
bookFlatSection: true
---

# Commands

| Command | Description |
|---------|-------------|
| [`init`]({{< relref "init" >}}) | Create default config file |
| [`install`]({{< relref "install" >}}) | Install binary and register autostart |
| [`account`]({{< relref "account" >}}) | Manage provider accounts |
| [`discover`]({{< relref "discover" >}}) | List/import repositories for an account |
| [`repo`]({{< relref "repo" >}}) | Manage tracked repositories |
| [`sync`]({{< relref "sync" >}}) | Run a one-shot sync |
| [`status`]({{< relref "status" >}}) | Show status of all tracked repos |
| [`daemon`]({{< relref "daemon" >}}) | Manage the background sync daemon |
| [`ssh`]({{< relref "ssh" >}}) | Manage SSH keys |
| [`logs`]({{< relref "logs" >}}) | Show log file contents |

All commands accept `--config <path>` to override the default config location (`~/.git-sync/config.toml`).
