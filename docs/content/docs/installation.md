---
title: Installation
weight: 10
---

# Installation

## Using `go install`

Requires Go 1.22 or later.

```bash
go install github.com/basmulder03/git-projects-sync/cmd/git-sync@latest
```

This places the `git-sync` binary in `$GOPATH/bin` (or `~/go/bin`).

## Pre-built binaries

Download the latest release binary for your OS from the [Releases](https://github.com/basmulder03/git-projects-sync/releases) page, then move it onto your `PATH`.

## Build from source

```bash
git clone https://github.com/basmulder03/git-projects-sync.git
cd git-projects-sync
go build -o git-sync ./cmd/git-sync
```

## Register autostart

Run `install` to copy the binary to its final location and register it as a login autostart item:

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
