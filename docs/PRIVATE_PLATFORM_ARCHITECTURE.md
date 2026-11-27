# Independent Private Platform — Architecture

## Product boundary

This project is a standalone messaging and collectibles platform. It does not authenticate against Telegram, use Telegram MTProto servers, or mint official Telegram Stars. The iOS app may retain selected open-source UI components from Telegram iOS, subject to the upstream license and required attribution, but all account, messaging, wallet and gift data belongs to this platform.

## Services

- **API** — versioned HTTPS REST endpoints; WebSocket gateway for real-time events.
- **Identity** — accounts, password hashing, sessions, device registration and recovery.
- **Messaging** — direct chats, groups, channels, membership, message delivery and read state.
- **Media** — presigned upload/download to private object storage; media authorization.
- **Wallet** — internal Stars ledger. All credits/debits are immutable entries and use database transactions plus idempotency keys.
- **Gift engine** — catalog, issuance caps, serial numbers, rarity, ownership, transfers and upgrade rules.
- **Admin** — role-based permissions, moderation and immutable audit events.

## Initial data model

- users, user_profiles, devices, sessions
- conversations, conversation_members, messages, message_receipts
- media_objects
- wallet_accounts, wallet_transactions
- gift_definitions, gift_issues, gift_ownership, gift_events
- admin_roles, admin_audit_events

## Critical invariants

1. A wallet balance cannot be edited directly. It changes only through ledger entries.
2. A transaction has a unique idempotency key to prevent duplicate credits.
3. Gift ownership changes are atomic and leave a permanent event record.
4. Limited gift issuance is enforced in the database transaction, not in the client.
5. Admin operations require server-side authorization and an audit reason.
6. Client input is untrusted; all prices, rarity, issuance limits and upgrade costs are decided by the server.

## Delivery sequence

1. API skeleton and health checks.
2. PostgreSQL migrations, configuration and integration tests.
3. Identity and session lifecycle.
4. Messaging with WebSocket delivery.
5. Media storage.
6. Internal Stars ledger.
7. Gifts and upgrades.
8. Admin API and admin UI.
9. iOS integration and end-to-end testing.
