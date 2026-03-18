-- +goose Up
CREATE TABLE users (
    id UUID,
    created_at TIMESTAMP NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    email TEXT NOT NULL UNIQUE
);

-- +goose Down
Drop TABLE users

