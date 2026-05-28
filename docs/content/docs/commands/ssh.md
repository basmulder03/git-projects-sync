---
title: ssh
weight: 9
---

# ssh

Manage SSH keys for provider accounts.

## ssh generate

```bash
git-sync ssh generate <account-id> [--force]
```

Generates an `ed25519` SSH key pair at the path configured for the account (`ssh_key_path`). Updates `~/.ssh/config` with a host alias entry. Prints the public key and the provider URL where it should be added.

| Flag | Description |
|------|-------------|
| `--force` | Overwrite an existing key |

## ssh show-pubkey

```bash
git-sync ssh show-pubkey <account-id>
```

Prints the public key and the provider URL for adding it.

## ssh test

```bash
git-sync ssh test <account-id>
```

Opens an SSH connection to the provider using the account's host alias and reports whether authentication succeeded.

## ssh status

```bash
git-sync ssh status
```

Prints a table showing, for each account, whether the key file exists and whether a `~/.ssh/config` entry is present.

---

### SSH host alias format

```
Host <provider>-<account-id>
  HostName <provider-hostname>
  User git
  IdentityFile <ssh_key_path>
  IdentitiesOnly yes
```

Example for `github-personal`:

```
Host github.com-github-personal
  HostName github.com
  User git
  IdentityFile ~/.ssh/git-sync-github-personal
  IdentitiesOnly yes
```
