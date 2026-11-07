# Contributing

- Preserve the shared Core/SDK boundary; do not make Core depend on the
  terminal or duplicate launch/download/process managers.
- Keep docs and examples aligned with actual JSON tags, flags, and registered
  commands. Clearly mark planned functionality as not implemented.
- Never add credentials to ordinary JSON config, tests, logs, or examples.
- Add focused tests for behavior and maintain cross-platform compilation.

Before submitting changes:

```sh
gofmt -w cmd internal pkg
go test ./...
go vet ./...
```

For executable or platform-specific changes, also run both matrix builds from
[`build.md`](build.md). Race tests are recommended where CGO and a C compiler
are available.
