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
