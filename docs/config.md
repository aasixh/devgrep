# Configuration

devgrep works without a config file. When `devgrep index` runs for the first time it writes defaults to:

```text
~/.config/devgrep/config.yaml
```

See [examples/config.yaml](../examples/config.yaml) for the complete shape.

## Indexed Paths

`indexed_paths` controls where log files and markdown notes are discovered. Shell history uses standard locations:

- `~/.bash_history`
- `~/.zsh_history`

## Ignore Rules

`ignored_directories` prevents expensive or noisy walks through build output and dependency folders.

## Ranking

Ranking weights are floating point values. They do not need to sum to 1; devgrep normalizes them internally.

## Indexing

`indexing.max_files` caps file discovery, `indexing.max_file_size_mb` skips oversized files, and `indexing.auto_watch` controls whether `devgrep index <path>` stays in watch mode after indexing.

Use `devgrep index . --dry-run` before indexing a new tree.

## Theme

The TUI accepts hex colors for accent, muted, error, warning, and success states.
