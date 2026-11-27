CREATE TABLE registration_code_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number_id UUID REFERENCES platform_numbers(id) ON DELETE SET NULL,
    number_snapshot VARCHAR(16) NOT NULL,
    username VARCHAR(32) NOT NULL DEFAULT '',
    event_type VARCHAR(32) NOT NULL CHECK (event_type IN (
        'requested','delivered','delivery_failed','wrong_code',
        'expired','locked','verified'
    )),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX registration_code_events_created_idx
    ON registration_code_events(created_at DESC);
CREATE INDEX registration_code_events_number_idx
    ON registration_code_events(number_snapshot, created_at DESC);
