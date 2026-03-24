-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    id, 
    created_at, 
    updated_at, 
    user_id, 
    expires_at, 
    revoked_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    NOW() + interval '60 days',
    NULL
)

returning id;
