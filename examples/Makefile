ENVMERGE ?= envmerge
ENV_DIR  := ./deploy/env

.PHONY: env-generate env-regenerate env-validate env-check

env-generate:
	$(ENVMERGE) generate --dir $(ENV_DIR)

env-regenerate:
	$(ENVMERGE) generate --dir $(ENV_DIR) --force

env-validate:
	$(ENVMERGE) validate --dir $(ENV_DIR)

env-check:
	$(ENVMERGE) check --dir $(ENV_DIR)
