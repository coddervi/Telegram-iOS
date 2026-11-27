CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(32) NOT NULL UNIQUE,
    email VARCHAR(254) UNIQUE,
    password_hash TEXT NOT NULL,
    display_name VARCHAR(64) NOT NULL,
    bio VARCHAR(160) NOT NULL DEFAULT '',
    avatar_key TEXT,
    role VARCHAR(16) NOT NULL DEFAULT 'user' CHECK (role IN ('user','moderator','admin')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_created_at_idx ON users(created_at DESC);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    device_name VARCHAR(128) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX sessions_user_idx ON sessions(user_id);

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('direct','group','channel')),
    title VARCHAR(128),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE conversation_members (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (member_role IN ('owner','admin','member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at TIMESTAMPTZ,
    PRIMARY KEY (conversation_id,user_id)
);
CREATE INDEX conversation_members_user_idx ON conversation_members(user_id);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL DEFAULT '',
    reply_to UUID REFERENCES messages(id),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX messages_conversation_time_idx ON messages(conversation_id,created_at DESC);
CREATE TABLE media_objects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id),
    object_key TEXT NOT NULL UNIQUE,
    content_type VARCHAR(128) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE wallet_accounts (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount BIGINT NOT NULL CHECK (amount <> 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    reason VARCHAR(32) NOT NULL,
    reference_id UUID,
    idempotency_key VARCHAR(128) NOT NULL,
    actor_id UUID REFERENCES users(id),
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id,idempotency_key)
);
CREATE INDEX wallet_transactions_user_time_idx ON wallet_transactions(user_id,created_at DESC);

CREATE TABLE gift_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(48) NOT NULL UNIQUE,
    title VARCHAR(96) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    image_key TEXT,
    rarity VARCHAR(16) NOT NULL CHECK (rarity IN ('common','uncommon','rare','epic','legendary')),
    price_stars BIGINT NOT NULL CHECK (price_stars >= 0),
    max_supply BIGINT CHECK (max_supply IS NULL OR max_supply > 0),
    issued_count BIGINT NOT NULL DEFAULT 0 CHECK (issued_count >= 0),
    upgrade_group VARCHAR(48),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE gift_ownership (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gift_definition_id UUID NOT NULL REFERENCES gift_definitions(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    serial_number BIGINT NOT NULL,
    purchase_key VARCHAR(128) NOT NULL,
    level INTEGER NOT NULL DEFAULT 1 CHECK (level > 0),
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(gift_definition_id,serial_number),
    UNIQUE(owner_id,purchase_key)
);
CREATE INDEX gift_ownership_owner_idx ON gift_ownership(owner_id,acquired_at DESC);
CREATE TABLE gift_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gift_ownership_id UUID NOT NULL REFERENCES gift_ownership(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    event_type VARCHAR(24) NOT NULL CHECK (event_type IN ('issued','transferred','upgraded','burned')),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE admin_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(64) NOT NULL,
    target_user_id UUID REFERENCES users(id),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
