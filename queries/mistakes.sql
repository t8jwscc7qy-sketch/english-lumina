-- name: InsertMistake :one
-- Новая открытая ошибка с серией 0. Вторая открытая на (user_id, sentence_id, kind) отклоняется индексом (23505).
INSERT INTO mistakes (user_id, kind, sentence_id, topics, payload, answer, given, next_due, first_wrong_at, source_exercise_id)
VALUES (sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(sentence_id), sqlc.arg(topics), sqlc.arg(payload), sqlc.arg(answer),
        sqlc.arg(given), sqlc.arg(next_due), sqlc.arg(first_wrong_at), sqlc.narg(source_exercise_id))
RETURNING id;

-- name: GetOpenMistakeForUpdate :one
-- Открытая ошибка на то же предложение и вид; строка блокируется. Нет строки (ErrNoRows) — открываем новую.
SELECT * FROM mistakes
WHERE user_id = sqlc.arg(user_id) AND sentence_id = sqlc.arg(sentence_id) AND kind = sqlc.arg(kind) AND status = 'open'
FOR UPDATE;

-- name: GetMistakeForUpdate :one
SELECT * FROM mistakes WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) FOR UPDATE;

-- name: ResetMistake :exec
-- Повторная ошибка на уже открытую: серия снова 0, новый срок и новый неверный ответ.
UPDATE mistakes
SET streak = 0, given = sqlc.arg(given), next_due = sqlc.arg(next_due), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: UpdateMistakeProgress :exec
-- Результат повтора: серия, статус, срок, время закрытия (NULL у открытой) и последний неверный ответ.
UPDATE mistakes
SET streak = sqlc.arg(streak), status = sqlc.arg(status), next_due = sqlc.arg(next_due),
    closed_at = sqlc.narg(closed_at), given = sqlc.arg(given), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: NextDueMistake :one
-- Самая старая по сроку открытая ошибка, чей срок наступил. Нет строки (ErrNoRows) — повторять нечего.
SELECT * FROM mistakes
WHERE user_id = sqlc.arg(user_id) AND status = 'open' AND next_due <= sqlc.arg(now)::timestamptz
ORDER BY next_due, id
LIMIT 1;

-- name: CountRepeatsToday :one
-- Сколько повторов отвечено с момента since (начало суток пользователя).
SELECT count(*)::bigint FROM exercises
WHERE user_id = sqlc.arg(user_id) AND mistake_id IS NOT NULL AND answered_at >= sqlc.arg(since)::timestamptz;

-- name: LastTwoExerciseRepeatFlags :many
-- Были ли повторами два последних отвеченных упражнения (сначала самое новое).
SELECT (mistake_id IS NOT NULL)::boolean AS is_repeat
FROM exercises
WHERE user_id = $1 AND answered_at IS NOT NULL
ORDER BY answered_at DESC, id DESC
LIMIT 2;

-- name: GetUnansweredRepeat :one
-- Выданная, но не отвеченная копия этой ошибки (перезагрузка страницы не плодит строки).
SELECT * FROM exercises
WHERE user_id = sqlc.arg(user_id) AND mistake_id = sqlc.arg(mistake_id) AND answered_at IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: ListOpenMistakes :many
SELECT * FROM mistakes
WHERE user_id = sqlc.arg(user_id) AND status = 'open'
ORDER BY next_due, id
LIMIT sqlc.arg(max_rows)::int;

-- name: CountMistakes :one
-- Открытые, ждущие повтора сейчас (срок наступил к now) и закрытые после week_ago.
SELECT
    count(*) FILTER (WHERE status = 'open')::bigint AS open,
    count(*) FILTER (WHERE status = 'open' AND next_due <= sqlc.arg(now)::timestamptz)::bigint AS due_now,
    count(*) FILTER (WHERE status = 'closed' AND closed_at >= sqlc.arg(week_ago)::timestamptz)::bigint AS closed_this_week
FROM mistakes
WHERE user_id = sqlc.arg(user_id);
