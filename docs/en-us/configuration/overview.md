# Configuration overview

CLE uses two different JSON documents:

1. A versioned **CLE configuration** read by `-config` or `cle-debug
   --config`.
2. A per-launch **launch request** read by `-launch` or the Debug Terminal's
   `launch --config`.

They are not interchangeable. The CLE config is validated against
[`configs/cle.schema.json`](../../../configs/cle.schema.json) and runtime checks.
Launch requests are strictly decoded from `pkg/models.LaunchRequest`; unknown
fields are rejected. See [the launch request reference](launch-config.md).

Example minimal CLE config:

```json
{"version":1}
```

Validate it with `cle -check-config cle.json`. Defaults are supplied by
`internal/config.Default`; see [schema and defaults](schema.md).
