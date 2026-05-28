---
title: repo
weight: 5
---

# repo

Manage repositories tracked in config.

## repo add

```bash
git-sync repo add --account <id> --name <owner/repo>
```

Manually add a single repository. Local path is derived from `workspace_root`.

## repo list

```bash
git-sync repo list
```

Prints a table of all tracked repositories.

## repo remove

```bash
git-sync repo remove <full-name>
```

Remove a repository from config. Does not delete local files.
