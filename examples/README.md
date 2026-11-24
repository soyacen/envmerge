# envmerge — worked examples

This directory mirrors the recommended MuonFlow layout from the design spec
(§15): shared infrastructure configuration lives in `deploy/env/template/`, and
each service keeps only its private overrides in a small `.env.example` that
`include`s the shared pieces.

```text
examples/
├── deploy/env/
│   ├── template/
│   │   ├── base.env.example
│   │   ├── logging.env.example
│   │   ├── mysql.env.example
│   │   ├── redis.env.example
│   │   ├── kafka.env.example
│   │   ├── temporal.env.example
│   │   ├── nacos.env.example
│   │   ├── otel.env.example
│   │   ├── storage.env.example
│   │   └── firebase.env.example
│   ├── .api.env.example
│   ├── .admin-api.env.example
│   └── .temporal-worker.env.example
└── Makefile
```

## Try it

```bash
# from the repository root
make examples                     # compiles every service template

# or directly
envmerge generate examples/deploy/env/.api.env.example --stdout
envmerge tree examples/deploy/env/.api.env.example
```

## What to notice

- Each service `include`s only the infrastructure it actually needs — the API
  pulls in storage/firebase, the Temporal worker pulls in temporal instead.
- A service's own `SERVICE_NAME` and `LOG_LEVEL` override the shared defaults
  (proximity / `就近原则`).
- Real secrets stay empty in templates; production injects them via Kubernetes
  Secrets or a vault, not via committed files.

## Makefile targets

```bash
make env-generate    # create .env files (never overwrites)
make env-regenerate  # force-refresh .env files
make env-validate    # check the graph
make env-check       # CI drift check (exit 4 on drift)
```
