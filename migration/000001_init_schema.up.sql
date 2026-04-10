CREATE TABLE subscriptions (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    repo TEXT NOT NULL,
    confirmed BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_token TEXT NOT NULL UNIQUE,
    unsubscribe_token TEXT NOT NULL UNIQUE,
    last_seen_tag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (email, repo)
);

CREATE INDEX idx_subscriptions_email ON subscriptions (email);
CREATE INDEX idx_subscriptions_repo ON subscriptions (repo);
CREATE INDEX idx_subscriptions_confirmed ON subscriptions (confirmed);
