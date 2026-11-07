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

`pkg/accounts` contains the Offline and Yggdrasil providers and account
manager. Yggdrasil session tokens are kept in the OS credential store, apart
from the account profile JSON. The CLI and terminal pass stored account IDs
through Core; the manager resolves the account and provider session before
version installation and process creation. Same-account refresh, validation,
launch-session resolution, logout, and removal are serialized.

Yggdrasil JVM arguments include a SHA-256-pinned Authlib Injector 1.2.8 agent
from the official distribution endpoint. This is required because the
Minecraft client's session requests are independent of CLE's login request.
Mock-server and argument-placement tests cover the integration, but real
authentication, live Minecraft launch, and Windows/macOS/Linux credential-store
runtime behavior have not been verified. Microsoft authentication is not
implemented.

There is no HTTP API package or server yet. `api.enabled` is reserved config
and does not activate a listener.
