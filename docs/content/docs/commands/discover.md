---
title: discover
weight: 4
---

# discover

List repositories accessible to an account. Optionally add them to the config.

```bash
git-sync discover <account-id> [flags]
```

| Flag | Description |
|------|-------------|
| `--add` | Add all discovered repos to config (skips ones already tracked) |
| `--include-archived` | Include archived repositories |

Example — preview without adding:

```bash
git-sync discover github-personal
```

Example — import all repos:

```bash
git-sync discover github-personal --add
```

When `--add` is used, each new repo is added to `[[repositories]]` with `auto_sync = true`. The local path is derived as `<workspace_root>/<provider>/<owner>/<repo>`.

Requires a valid PAT in the keychain (`git-sync account set-pat <id>`).
