CREATE TABLE platform_numbers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number VARCHAR(16) NOT NULL UNIQUE,
    telegram_chat_id BIGINT NOT NULL UNIQUE,
    linked_user_id UUID UNIQUE REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE registration_codes (
    number_id UUID PRIMARY KEY REFERENCES platform_numbers(id) ON DELETE CASCADE,
    username VARCHAR(32) NOT NULL,
    display_name VARCHAR(64) NOT NULL,
    password_hash TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0)
);
