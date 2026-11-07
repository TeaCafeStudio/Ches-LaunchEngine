# Future HTTP API JSON conventions

**Planned / Not Implemented.** This page records a documentation outline only;
it does not define a callable protocol.

When an HTTP API is implemented, its documented contract must specify content
type, request/response schemas, status codes, authentication, authorization,
redaction, and stable error codes. No current HTTP response envelope exists.

The formal CLI does implement a separate `--json` response:
`{ "success": boolean, "data": value|null, "error": object|null }`. Do not
assume that CLI envelope is already used by an HTTP API.
