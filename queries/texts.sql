-- name: TextInsert :one
INSERT INTO texts (user_id, kind, title, body, word_count, coverage_at_build, goal)
VALUES (sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(title), sqlc.arg(body), sqlc.arg(word_count), sqlc.arg(coverage_at_build), sqlc.arg(goal))
RETURNING id;

-- name: TextGet :one
SELECT id, kind, title, body, word_count, coverage_at_build, goal, created_at
FROM texts WHERE id = sqlc.arg(id) AND (user_id = sqlc.arg(user_id) OR user_id IS NULL); -- свои склеенные и общие авторские

-- name: TextLastUnrated :one
-- Последний показанный и ещё не оценённый текст пользователя (не старше 12 часов): его отдаём повторно, пока он не
-- оценён, чтобы возврат на страницу и смена режима не теряли текст и не плодили новые.
SELECT t.id, t.kind, t.title, t.body, t.word_count, t.coverage_at_build, t.goal, t.created_at
FROM text_shown sh
JOIN texts t ON t.id = sh.text_id
WHERE sh.user_id = sqlc.arg(user_id)
  AND sh.shown_at > now() - interval '12 hours'
  AND NOT EXISTS (SELECT 1 FROM text_reads r WHERE r.text_id = sh.text_id AND r.user_id = sh.user_id)
ORDER BY sh.shown_at DESC
LIMIT 1;

-- name: TextShownInsert :exec
INSERT INTO text_shown (user_id, text_id, coverage)
VALUES (sqlc.arg(user_id), sqlc.arg(text_id), sqlc.arg(coverage))
ON CONFLICT (user_id, text_id) DO NOTHING;

-- name: TextUpsertAuthored :one
-- Авторский текст по slug: вставка или обновление (текст файла — источник истины).
INSERT INTO texts (user_id, kind, slug, band, title, body, word_count, coverage_at_build, goal)
VALUES (NULL, 'authored', sqlc.arg(slug), sqlc.arg(band), sqlc.arg(title), sqlc.arg(body), sqlc.arg(word_count), 0, 0.85)
ON CONFLICT (slug) WHERE slug IS NOT NULL DO UPDATE SET band = EXCLUDED.band, title = EXCLUDED.title, body = EXCLUDED.body, word_count = EXCLUDED.word_count
RETURNING id;

-- name: TextReadIDs :many
-- Номера текстов, которые пользователь уже оценил (прочитал).
SELECT text_id FROM text_reads WHERE user_id = $1;

-- name: TextRateInsert :execrows
-- 1 строка — оценка записана, 0 — этот текст уже оценён (повтор запроса).
INSERT INTO text_reads (user_id, text_id, mode, understood, coverage_at_read)
VALUES (sqlc.arg(user_id), sqlc.arg(text_id), sqlc.arg(mode), sqlc.arg(understood),
        COALESCE((SELECT sh.coverage FROM text_shown sh WHERE sh.user_id = sqlc.arg(user_id) AND sh.text_id = sqlc.arg(text_id)), sqlc.arg(coverage_at_read)))
ON CONFLICT DO NOTHING;

-- name: TextRecentSentenceIDs :many
-- Номера предложений последних текстов пользователя: в новом тексте они не повторяются.
SELECT DISTINCT (s ->> 'id')::bigint AS sentence_id
FROM (SELECT body FROM texts WHERE user_id = sqlc.arg(user_id) ORDER BY id DESC LIMIT sqlc.arg(max_texts)::int) t,
     jsonb_array_elements(t.body -> 'sentences') s
WHERE s ->> 'id' IS NOT NULL;

-- name: TextTargetWords :many
-- Следующие по частоте слова, которых пользователь ещё не видел (не изучено, не «знаю» и не показано на «Знакомстве»): кандидаты в целевые слова
-- текста. Нужны перевод в базе знаний и строчное написание из букв.
SELECT l.id AS lexeme_id, l.lemma, l.pos, st.freq_rank, c.payload
FROM kb_lexeme_stat st
JOIN lexemes l ON l.id = st.lexeme_id
JOIN kb_teaching_card c ON c.lexeme_id = l.id
WHERE st.freq_rank IS NOT NULL
  AND l.lemma ~ '^[a-z]+$'
  AND l.pos IN ('noun', 'verb', 'adjective', 'adverb')
  AND COALESCE(jsonb_array_length(CASE WHEN jsonb_typeof(c.payload -> 'translations') = 'array' THEN c.payload -> 'translations' END), 0) > 0
  AND NOT EXISTS (SELECT 1 FROM word_learning w WHERE w.user_id = sqlc.arg(user_id) AND w.lexeme_id = l.id AND w.status IN ('learned', 'known_at_start', 'introduced'))
ORDER BY st.freq_rank, l.id
LIMIT sqlc.arg(max_rows)::int;

-- name: TextWordHints :many
-- Подсказки (перевод, IPA) для слов текста по лемме; у леммы с несколькими частями речи первым идёт самое частое слово.
SELECT l.id AS lexeme_id, l.lemma, l.pos, c.payload
FROM lexemes l
JOIN kb_teaching_card c ON c.lexeme_id = l.id
LEFT JOIN kb_lexeme_stat st ON st.lexeme_id = l.id
WHERE l.lemma = ANY(sqlc.arg(lemmas)::text[])
ORDER BY st.freq_rank NULLS LAST, l.id;

-- name: TextSetCoverage :exec
-- Новая цель понимания текста (строка настроек обучения создаётся, если её ещё нет).
INSERT INTO learn_preferences (user_id, text_coverage)
VALUES (sqlc.arg(user_id), sqlc.arg(text_coverage))
ON CONFLICT (user_id) DO UPDATE SET text_coverage = EXCLUDED.text_coverage, updated_at = now();
