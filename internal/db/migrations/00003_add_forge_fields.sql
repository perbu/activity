-- +goose Up
ALTER TABLE repositories ADD COLUMN forge_type VARCHAR(20);
ALTER TABLE repositories ADD COLUMN forge_owner VARCHAR(255);
ALTER TABLE repositories ADD COLUMN forge_repo VARCHAR(255);

-- +goose Down
ALTER TABLE repositories DROP COLUMN forge_type;
ALTER TABLE repositories DROP COLUMN forge_owner;
ALTER TABLE repositories DROP COLUMN forge_repo;
