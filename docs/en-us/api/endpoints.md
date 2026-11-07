# Planned HTTP endpoints

**Planned / Not Implemented.** None of the following routes are currently
registered or callable:

- `GET /api/v1/accounts`
- `POST /api/v1/accounts`
- `GET /api/v1/accounts/{id}`
- `POST /api/v1/accounts/{id}/validate`
- `POST /api/v1/accounts/{id}/refresh`
- `POST /api/v1/accounts/{id}/logout`
- `DELETE /api/v1/accounts/{id}`
- `POST /api/v1/auth/microsoft/start`
- `GET /api/v1/auth/microsoft/callback`

Before implementation, define caller authentication/authorization, OAuth
callback policy, request and response schemas, HTTP status/error mappings, and
how secrets are excluded from responses and logs. No example `curl` call is
provided because there is no server to receive it.
