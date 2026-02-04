-- +goose Up
ALTER TABLE repositories ADD COLUMN external BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE repositories DROP COLUMN external;
