# envmerge

A small, fast, dependency-light **dotenv template compiler**. It lets `.env.example`
files reference each other with an `include` directive and compiles them into
flattened, deterministic `.env` files — so a monorepo with many services can
share infrastructure configuration instead of copying it into every file.

English | [中文](README_zh.md)

```dotenv
# .api.env.example
include ./template/base.env.example
include ./template/mysql.env.example
include ./template/redis.env.example

# HTTP Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8080

# Override a shared default
LOG_LEVEL=debug
```

```bash
envmerge generate .api.env.example   # -> .api.env (no includes, ready to use)
```

## Why

- **Composable** — reference shared templates recursively (templates can include
  templates).
- **Documented** — every comment in every template survives into the generated
  file: file headers, section notes, comment-only "defaults catalogue" files,
  comments above an `include`, and dangling notes at end of file. Nothing to
  switch on, nothing to lose.
- **Private-first** — a service's own variables always win over included ones.
- **Deterministic** — the same inputs always produce byte-identical output.
- **Safe** — never executes shell commands, never overwrites an existing `.env`
  without `--force`, and never logs secret values.
- **Single binary** — compiled with the Go standard library plus one CLI
  framework; no runtime dependencies.

## Install

Pre-built binaries are published on the [releases](../../releases) page. To build
from source (requires Go 1.27+):

```bash
go build -o envmerge ./cmd/envmerge
# or, install the current tagged version:
go install github.com/soyacen/envmerge/cmd/envmerge@latest
```

## Commands

| Command          | Purpose                                                     |
| ---------------- | ----------------------------------------------------------- |
| `generate`       | Compile one or more templates into `.env` files              |
| `validate`       | Check syntax, include existence and cycle-freeness           |
| `check`          | Fail (exit 4) when a generated file is out of date (CI)      |
| `tree`           | Print the dependency tree with counts                        |
| `explain`        | Show where a variable's final value came from                |
| `diff`           | Show the difference between a template and its output        |
| `watch`          | Re-generate on any change to the template graph              |

Full flag reference lives in [`docs/cli.md`](docs/cli.md); the template grammar
lives in [`docs/syntax.md`](docs/syntax.md).

## Merge model (就近原则 / proximity)

Duplicates are resolved by **closeness to the entry template**, then by
declaration order:

1. A variable declared in a file wins over the same variable pulled in from an
   `include` (private-first).
2. Among includes at the same depth, the later one wins.
3. Within one file, the later line wins.

Each variable appears exactly once, at the position where it was **first seen**;
`include` is expanded where it is declared, so the generated file keeps the
section order the templates were written in. Comments stay with the lines they
document: the comment run written above a declaration that loses a conflict is
relocated directly above the surviving assignment (its origin is reported by
`explain` and `--verbose`), so no comment is ever left dangling or dropped.

Four policies are available via `--conflict`: `local-wins` (default, the rule
above), `last-wins`, `first-wins`, and `error`.

## CI/CD

```yaml
- run: envmerge check --dir ./deploy/env        # exit 4 if any .env drifted
- run: envmerge generate --dir ./deploy/env --force
```

## SDK

The engine is importable as a library:

```go
import "github.com/soyacen/envmerge/pkg/envmerge"

engine := envmerge.New(envmerge.Options{
    ConflictPolicy: envmerge.LocalWins, // comments and blank lines always kept
})
res, err := engine.Generate(ctx, ".api.env.example", "")
```

## License

MIT — see [LICENSE](LICENSE).
