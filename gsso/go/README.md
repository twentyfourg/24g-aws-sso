# gsso

Module for the `gsso` binary. Usage, install, and the shell hook are in the
[parent README](../README.md).

```bash
go build -o gsso ./cmd/gsso
go test ./...
```

`go test` covers the config file engine, profile names, MCP JSON/TOML edits,
and the non-interactive `add`, `switch`, and `mcp` paths. It does not drive
the interactive menus.
