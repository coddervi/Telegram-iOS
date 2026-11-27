# Private Platform Backend

Independent backend foundation for the native iOS client. This service is not connected to Telegram's infrastructure and does not issue official Telegram Stars.

## Stack

- Go HTTP API
- PostgreSQL for durable application data
- Redis for ephemeral state, rate limits and presence
- Object storage for media and file attachments

## Run the API foundation

```sh
cd backend
go run ./cmd/api
```

The service listens on `:8080` by default.

- `GET /health` — liveness endpoint
- `GET /v1` — API identity/version

## Planned modules

1. Identity: registration, sessions, device management and account recovery.
2. Messaging: direct conversations, groups, channels, message history and delivery.
3. Media: authenticated upload/download and object-storage metadata.
4. Wallet: internal Stars ledger with immutable, auditable transactions.
5. Gifts: catalog, limited editions, ownership, transfers, upgrades and collections.
6. Administration: role-based access, moderation and audited administrative actions.

## Security principles

- Passwords are stored only as strong password hashes.
- Session tokens are random, revocable and stored hashed server-side.
- Wallet changes are append-only ledger transactions; balances are derived transactionally.
- Every privileged action is authorized server-side and written to an audit log.
- Client applications never receive database credentials or signing secrets.
