-- name: UpgradeUserByID :one
UPDATE users
SET is_chirpy_red = true
WHERE id = $1

returning id, created_at, updated_at, email, is_chirpy_red;