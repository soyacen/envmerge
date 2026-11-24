# CLI reference

`envmerge [command] [arguments] [flags]`

## Commands

### generate

Compile one or more templates into `.env` files.

```bash
envmerge generate .api.env.example                  # auto output: .api.env
envmerge generate .api.env.example -o deploy/.env    # explicit output
envmerge generate .api.env.example .admin.env.example # batch
envmerge generate --dir ./deploy/env --pattern '*.env.example'
envmerge generate --dir ./deploy/env --recursive     # include subdirs
```

The default output name strips a trailing `.example`:

| Input                 | Output              |
| --------------------- | ------------------- |
| `.api.env.example`    | `.api.env`          |
| `api.env.example`     | `api.env`           |
| `.env.example`        | `.env`              |
| `config/.env.example` | `config/.env`       |

Without `--force` an existing output is **never** overwritten (protecting
hand-edited secrets). `--dry-run` and `--stdout` print to stdout without
writing.

### validate

```bash
envmerge validate .api.env.example
envmerge validate --dir ./deploy/env
```

Reports syntax, include existence, cycles and variable validity.

### check

```bash
envmerge check .api.env.example
```

Recomputes the template in memory and compares it with the generated file.
Exits **4** on any drift or missing output — ideal for CI.

### tree

```bash
envmerge tree .api.env.example
```

```text
.api.env.example
├── template/base.env.example
└── template/db.env.example
    └── template/mysql.env.example

Templates: 4
Variables: 28
Overrides: 2
```

### explain

```bash
envmerge explain .api.env.example LOG_LEVEL
```

Shows the winning declaration, the first occurrence and the override chain.
Values of sensitive keys are redacted.

### diff

```bash
envmerge diff .api.env.example
```

Unified diff between the generated file and the current template output.

### watch

```bash
envmerge watch .api.env.example
```

Watches the entry and every dependency and regenerates (with force) on change.
Press Ctrl-C to stop.

### version

```bash
envmerge version
envmerge --version
```

## Flags

| Flag              | Default          | Applies to                       | Description                                |
| ----------------- | ---------------- | -------------------------------- | ------------------------------------------ |
| `--output, -o`    | auto-derived     | generate/check/diff/watch        | Explicit output path (single input only)   |
| `--force, -f`     | `false`          | generate                         | Overwrite an existing output               |
| `--dry-run`       | `false`          | generate                         | Preview to stdout, write nothing           |
| `--stdout`        | `false`          | generate                         | Write result to stdout                     |
| `--conflict`      | `local-wins`     | all                              | `local-wins\|last-wins\|first-wins\|error` |
| `--max-depth`     | `64`             | all                              | Maximum include recursion depth            |
| `--comments`      | `true`           | generate/check/diff/watch        | Preserve comments                          |
| `--blank-lines`   | `true`           | generate/check/diff/watch        | Preserve blank-line grouping               |
| `--dir`           | —                | generate/validate/check          | Directory to scan                          |
| `--pattern`       | `*.env.example`  | generate/validate/check          | Glob used with `--dir`                     |
| `--recursive`     | `false`          | generate/validate/check          | Recurse when scanning `--dir`              |
| `--root`          | —                | all                              | Restrict include resolution to a directory |
| `--verbose, -v`   | `false`          | all                              | Print override diagnostics                 |
| `--quiet, -q`     | `false`          | all                              | Suppress informational output              |

## Exit codes

| Code | Meaning                                                       |
| ---- | ------------------------------------------------------------- |
| `0`  | Success                                                       |
| `1`  | Program / filesystem error (e.g. write failed, output exists) |
| `2`  | CLI usage error                                               |
| `3`  | Template validation or parse error                            |
| `4`  | `check` detected drift                                        |

## Error codes

`ERR_FILE_NOT_FOUND`, `ERR_INVALID_SYNTAX`, `ERR_INVALID_INCLUDE`,
`ERR_CIRCULAR_INCLUDE`, `ERR_MAX_DEPTH`, `ERR_DUPLICATE_VARIABLE`,
`ERR_INVALID_VARIABLE`, `ERR_OUTPUT_EXISTS`, `ERR_PATH_OUTSIDE_ROOT`,
`ERR_PERMISSION_DENIED`, `ERR_WRITE_FAILED`, `ERR_OUTPUT_MISMATCH`,
`ERR_REQUIRED_MISSING`.
