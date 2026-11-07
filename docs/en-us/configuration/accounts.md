# Account profile storage

Phase 4.1.1 includes only the offline provider. Debug Terminal commands are
`account add offline <name> [uuid]`, `account list`, `account inspect <id>`,
`account profile <id>`, `account validate <id>`, `account switch <id>`, and
`account remove <id>`.

Profiles are stored in JSON under the OS user configuration directory at
`ChesLaunchEngine/accounts.json`. Override it with `account.storePath` in CLE
configuration. On POSIX, the directory/file permissions are restricted to the
user. On Windows, the user-profile ACL is inherited.

This store contains non-sensitive offline profile metadata only. It is not an
encrypted credential vault. There are no online tokens or passwords to store
in this phase. Non-empty `account.credentialStore` and online
`account.defaultProvider` values are rejected as unsupported.

Offline UUIDs use Minecraft's `OfflinePlayer:<name>` UUID algorithm. A supplied
UUID must be canonical 8-4-4-4-12 hexadecimal. Offline identities cannot
authenticate to online-mode servers. Refresh and logout are unsupported
because offline profiles have no remote authentication session.
