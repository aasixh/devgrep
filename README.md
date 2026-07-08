<div align="center">



<strong>Search your developer workflow history instantly.</strong>

<p>
Terminal-native search for shell history, logs, markdown notes,
and indexed project workflows.
</p>

<p>
<a href="https://github.com/aasixh/devgrep">GitHub</a>
&nbsp;·&nbsp;
<a href="https://devgrep.vercel.app/">Documentation</a>
&nbsp;·&nbsp;
<a href="https://github.com/aasixh/devgrep/releases">Releases</a>
&nbsp;·&nbsp;
<a href="https://x.com/aasixh">Twitter/X</a>
</p>

<p>
<img src="https://img.shields.io/github/stars/aasixh/devgrep?style=flat-square" />
<img src="https://img.shields.io/github/license/aasixh/devgrep?style=flat-square" />
<img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go" />
</p>

</div>
devgrep is a terminal-native search engine for developer workflows. It indexes shell history, log files, and markdown notes into a local SQLite database so you can recover commands, debugging steps, and project context without digging through scattered files.

No cloud. No accounts. No telemetry. No AI layer — just fast, offline search in your terminal.

<p align="center">
  <img 
    src="assets/devgrep-demo.gif" 
    alt="devgrep demo"
    width="900"
  />
</p>


## Documentations

Comprehensive documentation, including usage guides, is available at [Devgrep Docs](https://devgrep.vercel.app/)

* [Getting Started](https://devgrep.vercel.app/docs/getting-started)
* [Commands list](https://devgrep.vercel.app/docs/commands)
* [Some examples](https://devgrep.vercel.app/docs/examples)
* [Contributing](https://devgrep.vercel.app/docs/contributing)



---

## Install

**Go**

```sh
go install github.com/aasixh/devgrep@latest
```

**Release binary**

```sh
curl -fsSL https://raw.githubusercontent.com/aasixh/devgrep/main/scripts/install.sh | sh
```

**Build from source**

```sh
git clone https://github.com/aasixh/devgrep
cd devgrep
make build
./bin/devgrep version
```

Platforms: Linux, macOS, and Windows (`amd64` / `arm64`) via manual method (docs/release.md).



## Commands

| Command | Description |
| --- | --- |
| `devgrep search [query]` | Search indexed workflows (TUI or plain) |
| `devgrep [query]` | Shorthand for `search` |
| `devgrep index [path...]` | Index history, logs, and notes |
| `devgrep index . --dry-run` | Preview files without writing to SQLite |
| `devgrep index <path> --no-watch` | Index once, do not stay in watch mode |
| `devgrep index --watch` | Restore and run persisted watchers |
| `devgrep sources` | List indexed source locations |
| `devgrep sources --tree` | Tree view of indexed paths |
| `devgrep stats` | Document counts, DB size, top searches |
| `devgrep doctor` | Local health checks |
| `devgrep version` | Version and build metadata |

**Global flags:** `--config`, `--db`, `--plain`, `--verbose`

**Search flags:** `--source history,logs,notes`, `-n` limit, `-i` force TUI, `--tail`, `--regex`, `--severity`

**Index flags:** `--source`, `--path`, `--watch`, `--no-watch`, `--dry-run`, `-y` confirm risky paths

---



## Configuration

devgrep runs with sensible defaults. No config file is required for the first run.

On first `devgrep index`, defaults are written to `~/.config/devgrep/config.yaml` if missing. Customize indexed paths, ignore rules, ranking weights, history limits, log extensions, and TUI colors.

Full reference: [docs/user-manual.md](docs/user-manual.md) · [docs/config.md](docs/config.md)

---

## Development

```sh
make build    # bin/devgrep
make test     # race detector + coverage
make lint     # golangci-lint or go vet
make bench    # includes 100k-document search benchmark
```


---



## License

[MIT](LICENSE)
