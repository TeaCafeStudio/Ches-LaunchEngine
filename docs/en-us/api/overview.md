# HTTP API status

**Planned / Not Implemented.** There is no HTTP router, listener, or account
endpoint in this repository. The `api` object in CLE configuration is only a
configuration placeholder; setting `api.enabled` does not start an API server.

Do not send requests to the configured address expecting a CLE service. The
current supported integration surface is the Go SDK and the CLI.
