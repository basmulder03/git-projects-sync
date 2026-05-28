---
title: account
weight: 3
---

# account

Manage provider accounts stored in config.

## account add

```bash
git-sync account add --provider <github|azure_devops> --id <id> [flags]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--provider` | yes | `github` or `azure_devops` |
| `--id` | yes | Unique account ID (e.g. `github-personal`) |
| `--username` | GitHub | GitHub username |
| `--org` | Azure DevOps | Organisation name |
| `--ssh-key` | | Path to SSH private key |
| `--clone-method` | | `ssh` (default) or `https` |
| `--set-pat` | | Prompt for a PAT and store it in the OS keychain |

Example:

```bash
git-sync account add \
  --provider github \
  --id github-personal \
  --username yourname \
  --ssh-key ~/.ssh/git-sync-github-personal \
  --set-pat
```

## account list

```bash
git-sync account list
```

Prints a table of configured accounts.

## account remove

```bash
git-sync account remove <id>
```

Removes the account from config, clears its SSH config entry, and deletes its PAT from the keychain.

## account set-pat

```bash
git-sync account set-pat <id>
```

Prompts for a PAT and stores it in the OS keychain. The PAT is used for repository discovery only — not for cloning.
