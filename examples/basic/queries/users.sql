-- name: FindByEmail :one
-- params: email
SELECT id, email, name, active, created_at
FROM users
WHERE email = $1
LIMIT 1;

-- name: ListActive :many
-- params: active
SELECT id, email, name, active, created_at
FROM users
WHERE active = $1
ORDER BY created_at DESC;

-- name: DeactivateByEmail :execrows
-- params: email
UPDATE users
SET active = false
WHERE email = $1;
