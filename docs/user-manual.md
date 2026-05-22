# devgrep User Manual

## grep for developer workflows

`devgrep` is a terminal-first search tool for developer memory.

It helps you find:

- commands you ran before
- debugging fixes
- deployment steps
- Docker and database workflows
- log errors
- notes written in Markdown

Everything runs locally. There is no account, no cloud service, no telemetry, and no AI wrapper.

## Why devgrep Exists

Developers often solve the same problem more than once:

1. You fix a hard issue.
2. The command or log pattern disappears into history.
3. Weeks later, you search shell history, notes, and logs by hand.

`devgrep` gives those workflow breadcrumbs one searchable index.

## First Run

Build or install `devgrep`, then create your first local index:

```sh
devgrep index
```

This indexes your shell history and configured local paths. The database is stored at:

```text
~/.local/share/devgrep/devgrep.db
```

The optional config file is stored at:

```text
~/.config/devgrep/config.yaml
```

You do not need to create a config file before using devgrep.

## Searching

The fastest syntax is direct search:

```sh
devgrep postgres timeout
```

This is shorthand for:

```sh
devgrep search "postgres timeout"
```

Both forms work. Existing commands such as `devgrep index`, `devgrep stats`, and `devgrep doctor` still behave normally.

For script-friendly output:

```sh
devgrep --plain search "docker compose"
```

Example output:

```text
[history]
docker compose up -d postgres

[last used]
3 weeks ago

[directory]
~/projects/auth-api

[score]
92
```

## Interactive TUI

When output is a terminal, search opens a full-screen interface.

The search bar is always visible:

```text
Search: postgres|
```

Useful keys:

| Key | Action |
| --- | --- |
| `/` | search |
| `j` / `k` | move selection |
| `gg` | jump to top |
| `G` | jump to bottom |
| `enter` | run a selected history command |
| `y` | copy selected result |
| `esc` / `q` | quit |

Results are labeled as:

- `[history]`
- `[log]`
- `[note]`

Log and note previews include nearby lines when the original file is available.

## Indexing Safely

Index the current project:

```sh
devgrep index .
```

Index a specific project:

```sh
devgrep index ~/projects/api
```

Avoid indexing huge directories such as `/` or your whole home directory. devgrep detects risky paths and asks for confirmation. In non-interactive scripts, it refuses unless you pass `--yes`.

```sh
devgrep index /          # refused unless confirmed
devgrep index ~          # refused unless confirmed
devgrep index / --yes    # explicit confirmation
```

devgrep ignores common heavy directories and files by default, including:

- `.git`
- `node_modules`
- `vendor`
- `dist`
- `build`
- `.cache`
- large files
- media files
- common binary/archive files

These rules are centralized and configurable.

## Dry Run

Before indexing a directory, preview what devgrep would scan:

```sh
devgrep index . --dry-run
```

Dry-run mode:

- performs no database writes
- lists files that would be indexed
- lists ignored paths
- shows estimated counts

This is the safest way to inspect a new project.

## Automatic Synchronization

When you index an explicit directory, devgrep persists that directory as a watched source:

```sh
devgrep index ~/projects/api
```

By default, devgrep continues watching that path in the foreground and updates the database when files change. It uses `fsnotify`, debounces changes, and avoids polling loops.

To index once without staying in watch mode:

```sh
devgrep index ~/projects/api --no-watch
```

To restore previously watched directories:

```sh
devgrep index --watch
```

Watch mode handles:

- new files
- modified files
- deleted files

Deleted files are removed from the index when devgrep observes the removal event.

## Sources Command

Show indexed source locations:

```sh
devgrep sources
```

Example:

```text
[history]
~/.bash_history

[notes]
~/projects/devgrep/docs

[logs]
~/projects/auth-api/logs
```

Render sources as a tree:

```sh
devgrep sources --tree
```

Example:

```text
~/projects
├── auth-api
├── backend
└── devgrep
```

The command reads SQLite metadata and removes duplicates.

## Stats

Show indexed document counts, database size, top searches, and recent index runs:

```sh
devgrep stats
```

## Doctor

Check local health:

```sh
devgrep doctor
```

Doctor checks:

- config validity
- invalid YAML
- database health
- permissions
- missing paths
- unreadable directories
- invalid config values
- shell history availability

If something is wrong, devgrep tries to print a human-readable fix.

## Configuration

Default config:

```text
~/.config/devgrep/config.yaml
```

Common settings:

- `indexed_paths`: paths used when no explicit path is passed
- `ignored_directories`: directories skipped during walks
- `ranking`: search scoring weights
- `tui`: terminal colors
- `history.limit`: shell history cap
- `logs.max_file_size_mb`: max log file size
- `indexing.max_files`: file discovery limit
- `indexing.max_file_size_mb`: max general file size
- `indexing.auto_watch`: whether explicit indexes keep watching

Start from:

```text
examples/config.yaml
```

## Common Workflows

Find a Docker command:

```sh
devgrep docker compose postgres
```

Find a deployment fix:

```sh
devgrep deploy nginx restart
```

Find recent log errors:

```sh
devgrep search --source logs "connection refused"
```

Tail matching log lines:

```sh
devgrep search --source logs --tail --regex "ERROR|WARN"
```

Preview a project before indexing:

```sh
devgrep index ~/projects/api --dry-run
```

Index once without watching:

```sh
devgrep index ~/projects/api --no-watch
```

## Troubleshooting

No results:

```text
No results found.
Try a broader query.
```

This means the database exists, but nothing matched.

No index:

```text
No index found.
Run:
devgrep index
```

This means the database has no indexed documents yet.

Unsafe path:

```text
refusing to index /: filesystem root
```

Choose a project directory or pass `--yes` only if you really intend to scan the path.

## Project Philosophy

devgrep should stay:

- lightweight
- terminal-native
- offline-first
- fast
- Unix-like
- developer-focused

It should not become a cloud service, an Electron app, a telemetry system, or an AI wrapper.
