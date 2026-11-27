# Template syntax

envmerge reads dotenv-style template files (by convention named
`*.env.example`). A template is composed of statements, blank lines and
comments. The compiler expands `include` directives recursively and emits a
flat `.env` file that no longer contains any includes.

## Variables

```dotenv
APP_NAME=muonflow
APP_PORT=8080
EMPTY_VALUE=
QUOTED_VALUE="hello world"
SINGLE_QUOTED='hello world'
DATABASE_URL="mysql://root:password@localhost:3306/db"
```

Compatibility notes:

- The first `=` separates the key from the value; a value may contain further
  `=` characters.
- `#` begins a comment only at line start or when preceded by whitespace.
  `PASSWORD=abc#123` keeps the `#` in the value.
- Variable references such as `${HOST}` are **preserved verbatim** and are not
  expanded by envmerge. The consuming application decides whether to expand.
- Quoted multi-line values are supported:

  ```dotenv
  PRIVATE_KEY="-----BEGIN PRIVATE KEY-----
  MIIEvQIBADANBg...
  -----END PRIVATE KEY-----"
  ```

- envmerge is a template compiler, not a shell: `$(...)`, backticks, `source`
  and `.` are never executed. `source` / `.` lines are rejected as syntax
  errors; `KEY=$(cmd)` is kept as a literal value.

## Comments

```dotenv
# A full-line comment
HTTP_PORT=8080 # an inline comment
# include ./disabled.env.example   <- a commented include does nothing
```

Every comment line in every reachable template is copied into the generated
file, in template order:

| Written in a template                       | Where it lands in the output                    |
| ------------------------------------------- | ----------------------------------------------- |
| Section header above a variable             | Directly above that assignment                  |
| File header (above an `include`)            | Kept, before the content that follows           |
| Above an `include` directive                | Kept, directly above the included content      |
| A template with **no assignments at all**   | Kept in full (defaults catalogues survive)      |
| Commented-out defaults (`# PORT=3306`)      | Kept verbatim; `#` lines never reach the app    |
| Dangling comment at end of file             | Kept at the end of its section                  |
| Above a declaration that **loses** a conflict | Relocated above the surviving assignment, so it still documents the value in use |
| Inline (`KEY=value # note`)                 | Kept on the winning declaration's line          |

Blank lines behave the same way: the grouping of the templates is preserved, a
run of blank lines collapses to one, and the output never begins or ends with a
blank line. Because includes are expanded where they are declared, each expanded
template starts a new group separated by one blank line.

Commented-out assignments are documentation only: dotenv readers ignore `#`
lines, so a template can list defaults without activating them.

## Include

Include another template so its variables become part of this one:

```dotenv
include ./template/base.env.example
```

Supported forms:

```dotenv
include ./base.env.example
include ../common/base.env.example
include /opt/templates/base.env.example
include "./template/my config.env.example"     # quoted: path may contain spaces
include './template/my config.env.example'
```

**Relative paths are resolved against the directory of the file that declares
the `include`**, never against the process working directory. This is what makes
recursive includes behave correctly regardless of where you run the tool.

## Directives (advanced)

| Directive        | Meaning                                                       |
| ---------------- | ------------------------------------------------------------- |
| `include? PATH`  | Optional include; a missing file is silently ignored           |
| `include GLOB`   | `* ? [ ]` in a path expand to all matches (sorted, deterministic) |
| `unset KEY`      | Remove `KEY` from the merged result                            |
| `required KEY`   | Fail unless `KEY` is present and non-empty in the output       |

```dotenv
include ./template/redis.env.example
unset REDIS_PASSWORD
required DB_PASSWORD
```

## Not supported (by design)

```dotenv
source ./base.env
. ./base.env
APP_NAME=$(whoami)
```

envmerge never touches the current process environment and never calls
`os.Setenv`. It only reads template files and writes an output file.

## Output ordering & deduplication

- Every variable appears exactly once.
- A variable is emitted at the position where it was **first seen** during a
  depth-first traversal that expands each `include` at the line where it is
  declared, so the output follows the order the templates were written in.
- Overriding changes only the value, never the position, and a comment can only
  move from a losing declaration to the assignment that replaced it — so
  repeated runs are byte-identical and git diffs stay meaningful.
- A template reached through several include paths (a diamond) contributes each
  of its comment lines once.

See the README for the full precedence rules (`就近原则`) and the `--conflict`
policies.
