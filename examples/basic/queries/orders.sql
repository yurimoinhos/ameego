-- name: ListByUser :many
-- params: user_id
SELECT id, user_id, total, status, notes, placed_at, updated_at
FROM orders
WHERE user_id = $1
ORDER BY placed_at DESC;

-- name: CountByStatus :scalar
-- params: status
SELECT COUNT(*) FROM orders WHERE status = $1;
