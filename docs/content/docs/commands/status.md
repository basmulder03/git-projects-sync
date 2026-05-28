---
title: status
weight: 7
---

# status

Show status of all tracked repositories.

```bash
git-sync status
```

Prints a table with icon, short path, current branch, and state:

| Icon | Meaning |
|------|---------|
| `✓` | Clean, up to date |
| `↓` | Behind remote (will pull on next sync) |
| `✗` | Dirty working tree (will be skipped on sync) |
| `-` | Not yet cloned |
| `?` | Error reading status |
