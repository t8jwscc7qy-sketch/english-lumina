-- name: UpsertSentence :one
-- Возвращает строку, которая лежит в базе под (lang, tatoeba_id): только что вставленную или уже существовавшую.
-- Вызывающий сверяет text и author: расхождение значит, что в данных один номер с разным содержимым.
WITH ins AS (
    INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id)
    VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (lang, tatoeba_id) DO NOTHING
    RETURNING id, text, author
)
SELECT ins.id::bigint AS id, ins.text::text AS text, ins.author::text AS author FROM ins
UNION ALL
SELECT s.id, s.text, s.author FROM sentences s
WHERE s.lang = $1 AND s.tatoeba_id = $2 AND NOT EXISTS (SELECT 1 FROM ins)
LIMIT 1;

-- name: UpsertSentencePair :exec
INSERT INTO sentence_pairs (en_id, ru_id, source_id)
VALUES ($1, $2, $3)
ON CONFLICT (en_id, ru_id) DO NOTHING;

-- name: ListExamples :many
-- Примеры для слова: английские предложения из 4–12 слов, где встречается любая из форм (query — выражение
-- to_tsquery('simple', ...), его строит Go из слов, без знаков синтаксиса), и самый короткий русский перевод.
-- Ближе к 8 словам лучше: «Run.» ничему не учит, абзац мешает.
SELECT e.tatoeba_id AS en_tatoeba_id, e.text AS en_text, e.author AS en_author,
       r.tatoeba_id AS ru_tatoeba_id, r.text AS ru_text, r.author AS ru_author
FROM sentences e
CROSS JOIN LATERAL (
    SELECT rs.tatoeba_id, rs.text, rs.author
    FROM sentence_pairs p
    JOIN sentences rs ON rs.id = p.ru_id
    WHERE p.en_id = e.id
    ORDER BY rs.word_count, rs.tatoeba_id
    LIMIT 1
) r
WHERE e.lang = 'en'
  AND e.word_count BETWEEN 4 AND 12
  AND to_tsvector('simple', e.text) @@ to_tsquery('simple', sqlc.arg(query)::text)
ORDER BY abs(e.word_count - 8), e.tatoeba_id
LIMIT sqlc.arg(max_rows)::int;
