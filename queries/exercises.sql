-- name: InsertExercise :one
INSERT INTO exercises (user_id, kind, sentence_id, topics, payload, answer)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetExerciseForUpdate :one
-- Упражнение пользователя; строка блокируется до конца транзакции, чтобы два ответа не наложились.
SELECT * FROM exercises WHERE id = $1 AND user_id = $2 FOR UPDATE;

-- name: MarkExerciseAnswered :exec
UPDATE exercises SET answered_at = now() WHERE id = $1 AND answered_at IS NULL;

-- name: UsedSentences :many
-- Предложения, уже использованные пользователем в теме (чтобы не повторять).
SELECT DISTINCT sentence_id FROM exercises WHERE user_id = $1 AND sqlc.arg(topic)::text = ANY(topics);

-- name: ListTopicRatings :many
SELECT * FROM topic_ratings WHERE user_id = $1 ORDER BY topic;

-- name: GetTopicRatingForUpdate :one
-- Рейтинг темы; строка блокируется. Нет строки (ErrNoRows) — по теме ещё не отвечали, берутся начальные значения.
SELECT * FROM topic_ratings WHERE user_id = $1 AND topic = $2 FOR UPDATE;

-- name: UpsertTopicRating :exec
INSERT INTO topic_ratings (user_id, topic, rating, deviation, volatility, attempts, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(topic), sqlc.arg(rating), sqlc.arg(deviation), sqlc.arg(volatility), sqlc.arg(attempts_delta)::int, now())
ON CONFLICT (user_id, topic) DO UPDATE SET
    rating     = EXCLUDED.rating,
    deviation  = EXCLUDED.deviation,
    volatility = EXCLUDED.volatility,
    attempts   = topic_ratings.attempts + EXCLUDED.attempts,
    updated_at = now();

-- name: SampleSentences :many
-- Кандидаты для упражнения: английские предложения из 4–12 слов с самым коротким переводом, начиная с from_id
-- (Go берёт случайный from_id и при нехватке повторяет с нуля). Идёт по первичному ключу, без сортировки всей таблицы.
SELECT e.id, e.tatoeba_id, e.text, e.author, e.word_count,
       r.id AS ru_id, r.text AS ru_text
FROM sentences e
CROSS JOIN LATERAL (
    SELECT rs.id, rs.text
    FROM sentence_pairs p
    JOIN sentences rs ON rs.id = p.ru_id
    WHERE p.en_id = e.id
    ORDER BY rs.word_count, rs.tatoeba_id
    LIMIT 1
) r
WHERE e.id >= sqlc.arg(from_id)::bigint
  AND e.lang = 'en'
  AND e.word_count BETWEEN 4 AND 12
ORDER BY e.id
LIMIT sqlc.arg(max_rows)::int;

-- name: SentenceLevels :many
-- По словам (нижний регистр): уровень слова словаря с таким написанием (самый низкий из оценок источников,
-- как в поиске; пусто, если слова нет в словаре) и место в частотном списке (пусто, если нет в списке).
WITH w AS (
    SELECT DISTINCT unnest(sqlc.arg(words)::text[]) AS word
), lv AS (
    SELECT lower(l.lemma) AS word, min(c.level)::text AS level
    FROM lexemes l
    JOIN lexeme_cefr c ON c.lexeme_id = l.id
    WHERE lower(l.lemma) = ANY(sqlc.arg(words)::text[])
    GROUP BY lower(l.lemma)
), fr AS (
    SELECT wf.word, wf.rank
    FROM word_frequencies wf
    WHERE wf.source_id = (
            SELECT s.id FROM data_sources s
            WHERE s.name = 'frequencywords' AND s.imported_at IS NOT NULL
            ORDER BY s.imported_at DESC, s.id DESC
            LIMIT 1
          )
      AND wf.word = ANY(sqlc.arg(words)::text[])
)
SELECT w.word::text AS word, lv.level, fr.rank
FROM w
LEFT JOIN lv ON lv.word = w.word
LEFT JOIN fr ON fr.word = w.word
ORDER BY w.word;

-- name: SentenceIDRange :one
-- Границы номеров предложений: Go берёт случайный from_id внутри них для SampleSentences. Пусто — (0, 0).
SELECT COALESCE(min(id), 0)::bigint AS min_id, COALESCE(max(id), 0)::bigint AS max_id FROM sentences;

-- name: RandomLexemesByLevel :many
-- Случайные однословные слова уровня (уровень слова — самый низкий из оценок источников, как в поиске).
-- Слов в списках около 10 тысяч, сортировка по random() на таком объёме дешёвая.
SELECT l.id, l.lemma, l.pos
FROM lexemes l
JOIN lexeme_cefr c ON c.lexeme_id = l.id
WHERE l.lemma ~ '^[a-z]+$'
GROUP BY l.id, l.lemma, l.pos
HAVING min(c.level) = sqlc.arg(level)::text
ORDER BY random()
LIMIT sqlc.arg(max_rows)::int;
