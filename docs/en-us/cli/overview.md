# Formal CLI

`cle` handles one-shot engine tasks. Its current flags are `-version`,
`-check-config <file>`, `-config <file>`, `-launch <request.json>`, and
`--json`. Both single- and double-dash forms are accepted by Go's flag parser.
Run `cle -h` for usage.

The CLI does not host the interactive Debug Terminal. The legacy `cle debug`
form prints a migration message; use the independently built `cle-debug`.
Commands not present in the implementation (for example `cle account list`
or `cle java list`) are not formal CLI commands.

See [the command reference](commands.md) and
[JSON output](json-output.md).
