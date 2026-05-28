---
title: logs
weight: 10
---

# logs

Show the contents of the log file.

```bash
git-sync logs [--lines <n>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--lines` | `50` | Number of lines to show (from the end) |

The log file location is set by `log_file` in config (default `~/.git-sync/git-sync.log`).

Log levels: `debug`, `info`, `warn`, `error` — controlled by `log_level` in config.
