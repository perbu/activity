-- +goose Up
CREATE TABLE page_views (
    id         BIGSERIAL    PRIMARY KEY,
    user_hash  VARCHAR(64),                              -- SHA-256 hex of user email; NULL for anonymous
    path       TEXT         NOT NULL,
    viewed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_page_views_viewed_at ON page_views(viewed_at);
CREATE INDEX idx_page_views_user_hash ON page_views(user_hash) WHERE user_hash IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS page_views;
