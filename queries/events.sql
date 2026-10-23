-- name: AppendEvent :one
INSERT INTO events (user_id, type, schema_version, occurred_at, payload, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id;

-- name: ListEventsAfter :many
SELECT * FROM events
WHERE user_id = $1 AND id > $2
ORDER BY id
LIMIT $3;

-- name: GetEventByKey :one
-- Событие по ключу идемпотентности (ErrNoRows, если такого ещё нет).
SELECT * FROM events WHERE user_id = $1 AND idempotency_key = $2;
