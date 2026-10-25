-- name: KbInsertBuild :one
INSERT INTO kb_build (status) VALUES ('running') RETURNING id;

-- name: KbFinishBuild :exec
UPDATE kb_build SET status = sqlc.arg(status), finished_at = now(), sources = sqlc.arg(sources), report = sqlc.arg(report)
WHERE id = sqlc.arg(id);

-- name: KbLatestBuild :one
-- Последняя успешная сборка (нет строки — сборки ещё не было).
SELECT * FROM kb_build WHERE status = 'ok' ORDER BY id DESC LIMIT 1;

-- name: KbDeleteTeachingCards :exec
DELETE FROM kb_teaching_card;

-- name: KbDeleteLexemeStats :exec
DELETE FROM kb_lexeme_stat;

-- name: KbDeleteRelations :exec
DELETE FROM kb_relation;

-- name: KbDeleteForms :exec
DELETE FROM kb_form;

-- name: KbDeletePronunciations :exec
DELETE FROM kb_pronunciation;

-- name: KbDeleteTranslations :exec
DELETE FROM kb_translation;

-- name: KbDeleteSenses :exec
DELETE FROM kb_sense;

-- name: KbGetTeachingCards :many
-- Карточки слов для урока по номерам слов.
SELECT lexeme_id, payload FROM kb_teaching_card WHERE lexeme_id = ANY(sqlc.arg(lexeme_ids)::bigint[]) ORDER BY lexeme_id;

-- name: KbListChosenTranslations :many
-- Выбранные переводы слов на язык (до двух на слово; основной первым).
SELECT lexeme_id, text, rank, score FROM kb_translation
WHERE chosen AND lang = sqlc.arg(lang)::text AND lexeme_id = ANY(sqlc.arg(lexeme_ids)::bigint[])
ORDER BY lexeme_id, rank;

-- name: KbListLemmas :many
-- Все леммы слов (как в источниках, с вариантами через «/»): по ним отбираются записи Kaikki.
SELECT DISTINCT lemma FROM lexemes ORDER BY lemma;
