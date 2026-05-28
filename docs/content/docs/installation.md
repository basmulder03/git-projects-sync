---
title: Installation
weight: 10
---

# Installation

## Using `go install` (recommended)

Requires Go 1.22 or later.

```bash
go install github.com/basmulder03/git-projects-sync/cmd/git-sync@latest
```

This places the `git-sync` binary in `$GOPATH/bin` (or `~/go/bin`). Make sure that directory is on your `PATH`.

To update to a newer version, run the same command again.

## Build from source

```bash
git clone https://github.com/basmulder03/git-projects-sync.git
cd git-projects-sync
go build -o git-sync ./cmd/git-sync
```

## Register autostart

Run `install` to register git-sync as a login autostart item:

```bash
git-sync install
```

Flags:

| Flag | Description |
|------|-------------|
| `--system` | Install system-wide (requires admin/root) |
| `--no-autostart` | Skip autostart registration |
| `--no-wizard` | Skip the first-account setup wizard |

On **Windows** this creates a Task Scheduler entry. On **macOS** a launchd plist is written to `~/Library/LaunchAgents/`. On **Linux** a systemd user unit is created.

## Remove autostart

```bash
git-sync uninstall
```

Add `--system` if you installed system-wide.
