# Architecture

```text
cle CLI -----------------------> pkg/sdk -> core.MinecraftEngine -> engine
cle-debug -> internal/terminal -> pkg/sdk -> core.DebugEngine ----> engine
```

The formal CLI adapter is in `internal/cli`; `cmd/cle` delegates to it and has
no dependency on `internal/terminal`. The independent `cmd/cle-debug` owns
interactive setup and imports the terminal. Both use the same SDK factory and
Vanilla engine implementation; neither contains a duplicate launcher.

`pkg/core` and `pkg/models` are shared contracts/data. `pkg/sdk` constructs the
engine. `internal/config` owns strict CLE configuration and launch-request
decoding. `internal/terminal` is a presentation/command adapter over the
public Debug Core interface. Events are provided by the existing bounded
engine event bus.

There is no HTTP API package or server yet. `api.enabled` is reserved config
and does not activate a listener. Microsoft/Yggdrasil providers are not
implemented.
