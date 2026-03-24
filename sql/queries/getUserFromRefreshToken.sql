-- name: GetUserFromRefreshToken :one
SELECT user_id, expires_at, revoked_at FROM refresh_tokens
WHERE id = $1
AND revoked_at IS NULL
AND expires_at > NOW();