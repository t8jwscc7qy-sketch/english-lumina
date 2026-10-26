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
SELECT lexeme_id, text, rank, score, flags FROM kb_translation
WHERE chosen AND lang = sqlc.arg(lang)::text AND lexeme_id = ANY(sqlc.arg(lexeme_ids)::bigint[])
ORDER BY lexeme_id, rank;

-- name: KbListLemmas :many
-- Все леммы слов (как в источниках, с вариантами через «/»): по ним отбираются записи Kaikki.
SELECT DISTINCT lemma FROM lexemes ORDER BY lemma;

-- name: KbListLexemes :many
-- Слова с самым низким уровнем CEFR среди источников (пусто, если уровня нет).
SELECT l.id, l.lemma, l.pos, COALESCE(min(c.level), '')::text AS level
FROM lexemes l
LEFT JOIN lexeme_cefr c ON c.lexeme_id = l.id
GROUP BY l.id, l.lemma, l.pos
ORDER BY l.id;

-- name: KbListFrequencies :many
-- Весь список частот последнего завершённого импорта источника с таким именем.
SELECT wf.word, wf.rank, wf.count
FROM word_frequencies wf
WHERE wf.source_id = (
        SELECT s.id FROM data_sources s
        WHERE s.name = sqlc.arg(source_name) AND s.imported_at IS NOT NULL
        ORDER BY s.imported_at DESC, s.id DESC
        LIMIT 1
      );

-- name: KbListLegacyTranslations :many
-- Все переводы последнего завершённого импорта словаря.
SELECT wt.lemma, wt.pos, wt.sense_no, wt.ru
FROM word_translations wt
WHERE wt.source_id = (
        SELECT s.id FROM data_sources s
        WHERE s.name = sqlc.arg(source_name) AND s.imported_at IS NOT NULL
        ORDER BY s.imported_at DESC, s.id DESC
        LIMIT 1
      )
ORDER BY wt.lemma, wt.pos, wt.sense_no, wt.ru;

-- name: KbSourceID :one
-- Номер последнего завершённого импорта источника (нет строки — источник не загружен).
SELECT id FROM data_sources WHERE name = sqlc.arg(name) AND imported_at IS NOT NULL ORDER BY imported_at DESC, id DESC LIMIT 1;
