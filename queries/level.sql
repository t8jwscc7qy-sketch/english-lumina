-- name: ListLevelAnswers :many
-- Верность последних ответов на упражнения темы (например level:B1) с момента since; сначала новые.
-- Тема ищется в payload.topics через jsonb_exists; без булева payload.correct ответ считается неверным.
SELECT COALESCE(CASE WHEN jsonb_typeof(payload -> 'correct') = 'boolean' THEN (payload ->> 'correct')::boolean END, false)::boolean AS correct
FROM events
WHERE user_id = sqlc.arg(user_id)
  AND type = 'exercise.answered'
  AND occurred_at >= sqlc.arg(since)::timestamptz
  AND jsonb_exists(payload -> 'topics', sqlc.arg(topic)::text)
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: LastLevelSuggestionDismissal :one
-- Когда пользователь в последний раз ответил «Пока нет» на предложение сменить уровень (нет строки — не отвечал).
SELECT occurred_at FROM events
WHERE user_id = $1 AND type = 'level.suggestion.dismissed'
ORDER BY occurred_at DESC, id DESC
LIMIT 1;
