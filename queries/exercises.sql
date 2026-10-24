-- name: InsertExercise :one
-- mistake_id задан только у повтора ошибки; у обычного упражнения NULL.
INSERT INTO exercises (user_id, kind, sentence_id, topics, payload, answer, mistake_id)
VALUES (sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(sentence_id), sqlc.arg(topics), sqlc.arg(payload), sqlc.arg(answer), sqlc.narg(mistake_id))
RETURNING id;

-- name: GetExerciseForUpdate :one
-- Упражнение пользователя; строка блокируется до конца транзакции, чтобы два ответа не наложились.
SELECT * FROM exercises WHERE id = $1 AND user_id = $2 FOR UPDATE;

-- name: MarkExerciseAnswered :exec
-- Время передаёт вызывающий: то же значение идёт в occurred_at события.
UPDATE exercises SET answered_at = sqlc.arg(answered_at) WHERE id = sqlc.arg(id) AND answered_at IS NULL;

-- name: UsedSentences :many
-- Предложения, на которые пользователь уже ответил в теме (чтобы не повторять). Выданные, но не отвеченные
-- не считаются: обновление страницы не должно сжигать предложения.
SELECT DISTINCT sentence_id FROM exercises WHERE user_id = $1 AND answered_at IS NOT NULL AND sqlc.arg(topic)::text = ANY(topics);

-- name: ListTopicRatings :many
SELECT * FROM topic_ratings WHERE user_id = $1 ORDER BY topic;

-- name: GetTopicRatingForUpdate :one
-- Рейтинг темы; строка блокируется. Нет строки (ErrNoRows) — по теме ещё не отвечали, берутся начальные значения.
SELECT * FROM topic_ratings WHERE user_id = $1 AND topic = $2 FOR UPDATE;

-- name: EnsureTopicRating :exec
-- Создаёт рейтинг темы с начальными значениями, если его ещё нет. Вызывать перед GetTopicRatingForUpdate: так
-- блокировать есть что даже у новой темы, и два параллельных ответа не потеряют обновление друг друга.
INSERT INTO topic_ratings (user_id, topic, rating, deviation, volatility, attempts, updated_at)
VALUES ($1, $2, 1500, 350, 0.06, 0, now())
ON CONFLICT (user_id, topic) DO NOTHING;

-- name: UpsertTopicRating :exec
-- Перезаписывает rating/deviation/volatility, а attempts увеличивает на delta. Безопасно только в транзакции после
-- GetExerciseForUpdate и EnsureTopicRating + GetTopicRatingForUpdate по этой теме (блокировка строки рейтинга
-- сериализует параллельные ответы); без неё параллельный ответ потерял бы обновление.
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

-- name: ListEnglishAlternatives :many
-- Другие английские предложения с тем же показанным русским переводом (ru_id) и тем же числом слов, что у en_id:
-- для «порядка слов» они тоже верные ответы. Само предложение не возвращается. Число слов совпадает заранее,
-- чтобы лимит не съедали предложения, которые всё равно не подойдут.
SELECT DISTINCT e.id, e.text
FROM sentence_pairs p
JOIN sentences e ON e.id = p.en_id
JOIN sentences mine ON mine.id = sqlc.arg(en_id)::bigint
WHERE p.ru_id = sqlc.arg(ru_id)::bigint
  AND p.en_id <> mine.id
  AND e.lang = 'en'
  AND e.word_count = mine.word_count
  AND e.word_count BETWEEN 4 AND 10
ORDER BY e.id
LIMIT sqlc.arg(max_rows)::int;

-- name: ListWeakCardWords :many
-- Слова карточек пользователя, с которыми были трудности (переобучение или хотя бы одна ошибка): сначала с
-- наибольшим числом ошибок, затем с ближайшим сроком. Только однословные строчные глаголы и прилагательные:
-- у существительных по правилам всего две формы, упражнение «выбор формы» их не использует.
SELECT l.id AS lexeme_id, l.lemma, l.pos
FROM cards c
JOIN lexemes l ON l.id = c.lexeme_id
WHERE c.user_id = sqlc.arg(user_id)::bigint
  AND (c.state = 3 OR c.lapses > 0)
  AND l.lemma ~ '^[a-z]+$'
  AND l.pos IN ('verb', 'adjective')
ORDER BY c.lapses DESC, c.due ASC, l.id
LIMIT sqlc.arg(max_rows)::int;
