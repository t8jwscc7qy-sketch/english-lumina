-- name: LearnCandidates :many
-- Слова для «Знакомства»: есть выбранный перевод в базе знаний, нет ни карточки, ни строки состояния изучения.
-- Сначала 1000 самых частотных слов строго по частоте (любой уровень CEFR-J), затем по уровню из списка и рангу частоты
-- (нет ранга — в конце).
SELECT l.id AS lexeme_id, l.lemma, l.pos, lv.level::text AS level, st.freq_rank, c.payload
FROM lexemes l
JOIN kb_teaching_card c ON c.lexeme_id = l.id
JOIN LATERAL (SELECT min(level) AS level FROM lexeme_cefr WHERE lexeme_id = l.id) lv ON true
LEFT JOIN kb_lexeme_stat st ON st.lexeme_id = l.id
WHERE (lv.level = ANY(sqlc.arg(levels)::text[]) OR st.freq_rank <= 1000)
  AND (l.lemma ~ '^[a-z]+$' OR l.lemma = 'I') -- «I» — самое частотное слово с заглавной
  AND (l.pos IN ('noun', 'verb', 'adjective', 'adverb') OR st.freq_rank <= 1000) -- у тысячи самых частых нужны и служебные слова
  AND COALESCE(jsonb_array_length(CASE WHEN jsonb_typeof(c.payload->'translations') = 'array' THEN c.payload->'translations' END), 0) > 0
  AND NOT EXISTS (SELECT 1 FROM word_learning w WHERE w.user_id = sqlc.arg(user_id) AND w.lexeme_id = l.id)
  AND NOT EXISTS (SELECT 1 FROM cards cd WHERE cd.user_id = sqlc.arg(user_id) AND cd.lexeme_id = l.id)
  -- Лемма, у которой другая часть речи уже в учёте (go глагол и go существительное), второй раз не предлагается.
  AND NOT EXISTS (SELECT 1 FROM word_learning w2 JOIN lexemes l2 ON l2.id = w2.lexeme_id WHERE w2.user_id = sqlc.arg(user_id) AND l2.lemma = l.lemma)
  AND NOT EXISTS (SELECT 1 FROM cards c2 JOIN lexemes l3 ON l3.id = c2.lexeme_id WHERE c2.user_id = sqlc.arg(user_id) AND l3.lemma = l.lemma)
ORDER BY (st.freq_rank IS NULL OR st.freq_rank > 1000), CASE WHEN st.freq_rank <= 1000 THEN st.freq_rank END, lv.level, st.freq_rank NULLS LAST, l.id
LIMIT sqlc.arg(max_rows)::int OFFSET sqlc.arg(row_offset)::int;

-- name: LearnRelations :many
-- Связи слов (синонимы, антонимы) между словами нашего словаря.
SELECT lexeme_id, other_lexeme_id FROM kb_relation
WHERE other_lexeme_id IS NOT NULL AND lexeme_id = ANY(sqlc.arg(lexeme_ids)::bigint[]);

-- name: LearnStatusCounts :many
SELECT status, count(*) AS count FROM word_learning WHERE user_id = $1 GROUP BY status;

-- name: LearnIntroducedSince :one
-- Сколько слов введено на «Знакомстве» с момента since (начало суток пользователя).
SELECT count(*) FROM word_learning WHERE user_id = sqlc.arg(user_id) AND source = 'lesson' AND introduced_at >= sqlc.arg(since)::timestamptz;

-- name: LearnInsertIntroduced :execrows
INSERT INTO word_learning (user_id, lexeme_id, status, source, introduced_at)
VALUES (sqlc.arg(user_id), sqlc.arg(lexeme_id), 'introduced', 'lesson', sqlc.arg(introduced_at)::timestamptz)
ON CONFLICT DO NOTHING;

-- name: LearnMarkKnown :execrows
INSERT INTO word_learning (user_id, lexeme_id, status, source)
SELECT sqlc.arg(user_id)::bigint, sqlc.arg(lexeme_id)::bigint, 'known_at_start', 'placement'
WHERE NOT EXISTS (SELECT 1 FROM cards cd WHERE cd.user_id = sqlc.arg(user_id)::bigint AND cd.lexeme_id = sqlc.arg(lexeme_id)::bigint)
ON CONFLICT DO NOTHING;

-- name: LearnMarkLearned :execrows
-- Хорошее вспоминание: слово становится изученным. Нет строки — создаётся (карточка добавлена вручную); уже изучено
-- или отмечено «знаю» — без изменений (0 строк).
INSERT INTO word_learning (user_id, lexeme_id, status, source, learned_at)
VALUES (sqlc.arg(user_id), sqlc.arg(lexeme_id), 'learned', 'card', sqlc.arg(now)::timestamptz)
ON CONFLICT (user_id, lexeme_id) DO UPDATE SET status = 'learned', learned_at = EXCLUDED.learned_at
WHERE word_learning.status = 'introduced';

-- name: LearnReviewStats :one
-- Повторения карточек с момента since: всего и с оценкой «хорошо» или «легко».
SELECT count(*) AS total, count(*) FILTER (WHERE (payload->>'rating')::int >= 3) AS good
FROM events
WHERE user_id = sqlc.arg(user_id) AND type = 'card.reviewed' AND occurred_at >= sqlc.arg(since)::timestamptz;

-- name: LearnGetPreferences :one
SELECT * FROM learn_preferences WHERE user_id = $1;

-- name: LearnUpsertPreferences :one
INSERT INTO learn_preferences (user_id, new_words_mode, new_words_per_day, camp_coverage)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE SET new_words_mode = EXCLUDED.new_words_mode, new_words_per_day = EXCLUDED.new_words_per_day,
    camp_coverage = EXCLUDED.camp_coverage, updated_at = now()
RETURNING *;

-- name: LearnPlacementCandidates :many
-- Слова быстрого теста: сначала 1000 самых частотных слов по частоте, затем самые частые слова уровня; тех, что уже в
-- карточках или в состоянии изучения, нет (страницы по offset).
SELECT l.id AS lexeme_id, l.lemma, l.pos
FROM lexemes l
JOIN LATERAL (SELECT min(level) AS level FROM lexeme_cefr WHERE lexeme_id = l.id) lv ON true
LEFT JOIN kb_lexeme_stat st ON st.lexeme_id = l.id
WHERE (lv.level = ANY(sqlc.arg(levels)::text[]) OR st.freq_rank <= 1000)
  AND NOT EXISTS (SELECT 1 FROM word_learning w WHERE w.user_id = sqlc.arg(user_id) AND w.lexeme_id = l.id)
  AND NOT EXISTS (SELECT 1 FROM cards cd WHERE cd.user_id = sqlc.arg(user_id) AND cd.lexeme_id = l.id)
ORDER BY (st.freq_rank IS NULL OR st.freq_rank > 1000), CASE WHEN st.freq_rank <= 1000 THEN st.freq_rank END, lv.level, st.freq_rank NULLS LAST, l.id
LIMIT sqlc.arg(max_rows)::int OFFSET sqlc.arg(row_offset)::int;

-- name: LearnKnownWords :many
-- Известные слова для «Привала»: изучено или отмечено «знаю».
SELECT l.id AS lexeme_id, l.lemma, l.pos
FROM word_learning w
JOIN lexemes l ON l.id = w.lexeme_id
WHERE w.user_id = $1 AND w.status IN ('learned', 'known_at_start');

-- name: TextKnownWords :many
-- Слова, понятные в тексте: изучено, «знаю» и уже показанные на «Знакомстве» (у них есть перевод и фраза).
SELECT l.id AS lexeme_id, l.lemma, l.pos
FROM word_learning w
JOIN lexemes l ON l.id = w.lexeme_id
WHERE w.user_id = $1 AND w.status IN ('learned', 'known_at_start', 'introduced');
