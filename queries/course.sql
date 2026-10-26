-- name: ListCompletedLessons :many
-- Пройденные уроки курса «Путь»: прогресс живёт в журнале событий (lesson.completed), отдельной таблицы нет.
SELECT DISTINCT (payload ->> 'lessonId')::text AS lesson_id
FROM events
WHERE user_id = $1 AND type = 'lesson.completed' AND payload ? 'lessonId';
