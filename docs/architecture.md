# Architecture

devgrep is a local-first CLI built around four small layers:

1. Source indexers parse developer artifacts into normalized documents.
2. SQLite storage persists documents, incremental offsets, and search stats.
3. The search engine retrieves candidates, applies fuzzy matching, and ranks results.
4. Cobra commands and the Bubble Tea TUI expose Unix-friendly workflows.

## Sources

The phase 1 sources are shell history, `.log` files, and markdown notes. Each source implements the pluggable `Indexer` interface in `internal/indexer`, so future sources can add Docker logs, Git history, stacktraces, and CI logs without changing command code.

File discovery uses centralized ignore logic in `internal/utils`, shared by normal indexing, dry-run mode, and watch re-indexing. The policy skips configured directories, huge files, media files, binary artifacts, and archives before source-specific parsers run.

## Storage

SQLite lives at `~/.local/share/devgrep/devgrep.db` by default. The store uses WAL mode, a busy timeout, prepared statements, and source state records for incremental indexing.

The main tables are:

- `documents`: indexed commands, log lines, and notes
- `source_states`: file offsets and modification metadata
- `search_stats`: local query frequency
- `index_runs`: indexing performance history
- `schema_migrations`: applied schema versions
- `watched_paths`: persisted directories restored by watch mode

The `sources` command reads indexed document metadata from SQLite and groups source locations without duplicating paths.

## Ranking

Ranking blends:

- fuzzy match quality
- recency
- usage frequency
- exact phrase matches
- command length
- directory relevance

The weights are configurable in `~/.config/devgrep/config.yaml`.

## Watch Mode

Explicit indexing paths are persisted as watched paths. Foreground watch mode uses `fsnotify` with debounced updates, reuses the same indexers, and removes documents for deleted files.

## Privacy

devgrep has no telemetry, no background network calls, no accounts, and no cloud API. All indexing and search happen on local files.
