# Private Platform Backend

Standalone backend foundation for the native iOS client. This service is independent of Telegram infrastructure and does not issue official Telegram Stars.

## Stack

- Go 1.23 HTTP API
- PostgreSQL 17 for durable application data
- Redis 7 reserved for presence, rate limits and ephemeral state
- Object storage planned for media and file attachments

## Run locally

Create a strong database password and start the services:

```sh
cd backend
export POSTGRES_PASSWORD='replace-with-a-long-random-secret'
docker compose up --build
```

The API listens on port 8080. PostgreSQL schema migrations run automatically on API startup.

## API

All API responses are JSON unless the response is `204 No Content`.

- `GET /health` — API and PostgreSQL liveness
- `GET /v1` — API version
- `POST /v1/auth/register` — create account (username 3–32 chars, password 10–72 bytes)
- `POST /v1/auth/login` — create session
- `GET /v1/auth/me` — current account (Bearer token)
- `POST /v1/auth/logout` — revoke current session
- `POST /v1/conversations` — create direct chat, group or channel
- `GET /v1/conversations` — list memberships
- `GET /v1/conversations/{conversationID}/messages` — read messages
- `POST /v1/conversations/{conversationID}/messages` — send a message
- `GET /v1/wallet` — internal Stars balance and ledger history
- `GET /v1/gifts` — active gift catalog
- `POST /v1/gifts/{giftID}/purchase` — purchase/claim a gift (requires `Idempotency-Key`)
- `POST /v1/admin/wallet/adjust` — administrator credit/debit (requires `Idempotency-Key`)
- `POST /v1/admin/gifts` — create a gift definition

Authenticated routes require `Authorization: Bearer <token>`.

## First administrator

Registration deliberately never grants administrator privileges. After creating your first account, promote it manually using a trusted database administrator connection:

```sql
UPDATE users SET role = 'admin' WHERE username = 'YOUR_USERNAME';
```

Do not expose database credentials or grant admin role from client-controlled input.

## Security principles

- Passwords are stored as bcrypt hashes.
- Session tokens are random, revocable and stored hashed server-side.
- Wallet changes are append-only ledger entries and occur in database transactions.
- Wallet operations require idempotency keys to prevent duplicate credits/debits.
- Gift supply limits and ownership issuance are enforced transactionally in PostgreSQL.
- Admin operations are authorized server-side and recorded in an audit table.
- Client input is untrusted; prices, rarity, supply limits and upgrade costs are decided by the server.

## Current implementation boundary

The current branch contains the API foundation, account/session lifecycle, basic conversation and text-message endpoints, internal wallet adjustments, gift catalog and gift purchase/issuance. Real-time WebSocket delivery, media upload/storage, group moderation, gift transfers/upgrades, push notifications, account recovery, encryption protocol and the iOS app integration are subsequent implementation stages.
