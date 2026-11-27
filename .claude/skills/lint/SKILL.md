---
name: lint
description: Run the Go linter (golangci-lint) on this project. Use when asked to lint, or to check Go code changes before finishing.
---

Run the linter with:

```sh
make lint
```

This runs golangci-lint in Docker, so Docker must be running. A local `golangci-lint` install isn't needed. Configuration is in `.golangci.yml`.
