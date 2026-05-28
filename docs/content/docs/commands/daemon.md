---
title: daemon
weight: 8
---

# daemon

Manage the background sync daemon.

## daemon start

```bash
git-sync daemon start
```

Starts the daemon process. It runs a full sync immediately, then repeats on the `sync_interval` from config. The process blocks; the autostart mechanism keeps it running across reboots.

## daemon stop

```bash
git-sync daemon stop
```

Sends a stop signal to the running daemon.

## daemon status

```bash
git-sync daemon status
```

Prints whether the daemon is currently running.
