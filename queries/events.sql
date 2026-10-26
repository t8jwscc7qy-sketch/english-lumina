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

-- name: ListActivityEvents :many
-- События занятий пользователя с момента since (для минут, серии и итога дня): ответы на упражнения,
-- оценки карточек, открытие/закрытие ошибок и пройденные уроки «Пути». correct берётся из булева поля payload.correct, без него false
-- (осмысленно только у exercise.answered). seconds — длительность урока «Пути» из payload.seconds (у остальных 0):
-- урок засчитывается своим временем, а не паузами между ответами.
SELECT type,
       occurred_at,
       COALESCE(CASE WHEN jsonb_typeof(payload -> 'correct') = 'boolean' THEN (payload ->> 'correct')::boolean END, false)::boolean AS correct,
       COALESCE(CASE WHEN type = 'lesson.completed' AND jsonb_typeof(payload -> 'seconds') = 'number' THEN LEAST(GREATEST((payload ->> 'seconds')::numeric, 0), 3600)::int END, 0)::int AS seconds
FROM events
WHERE user_id = sqlc.arg(user_id)
  AND type IN ('exercise.answered', 'card.reviewed', 'mistake.opened', 'mistake.closed', 'lesson.completed')
  AND occurred_at >= sqlc.arg(since)::timestamptz
ORDER BY occurred_at, id;
