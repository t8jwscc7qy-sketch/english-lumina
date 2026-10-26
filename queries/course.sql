-- name: ListCompletedLessons :many
-- Пройденные уроки курса «Путь»: прогресс живёт в журнале событий (lesson.completed), отдельной таблицы нет.
SELECT DISTINCT (payload ->> 'lessonId')::text AS lesson_id
FROM events
WHERE user_id = $1 AND type = 'lesson.completed' AND payload ? 'lessonId';

-- name: LockUserCourse :exec
-- Блокировка завершения уроков пользователя до конца транзакции: две вкладки с разными ключами не запишут два
-- «первых прохождения» одного урока.
SELECT pg_advisory_xact_lock(hashtext('course.complete'), sqlc.arg(user_id)::int);
