---
title: install / uninstall
weight: 2
---

# install

Install `git-sync` and register it as a login daemon.

```bash
git-sync install [flags]
```

| Flag | Description |
|------|-------------|
| `--system` | Install system-wide (requires admin/root) |
| `--no-autostart` | Skip autostart registration |
| `--no-wizard` | Skip the first-account setup wizard |

Unless `--no-wizard` is set and no accounts are configured yet, an interactive wizard runs to set up the first account.

# uninstall

Remove the autostart registration.

```bash
git-sync uninstall [--system]
```
