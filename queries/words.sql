-- name: SearchLexemes :many
-- Поиск по началу слова или по началу варианта через слэш (cafe/café). Шаблоны готовит Go (words.ParseQuery),
-- спецсимволы LIKE в них уже экранированы. Уровень слова — самый низкий из оценок источников.
SELECT l.id, l.lemma, l.pos, min(c.level)::text AS level
FROM lexemes l
JOIN lexeme_cefr c ON c.lexeme_id = l.id
WHERE lower(l.lemma) LIKE sqlc.arg(prefix)::text ESCAPE '\'
   OR lower(l.lemma) LIKE sqlc.arg(variant)::text ESCAPE '\'
GROUP BY l.id, l.lemma, l.pos
ORDER BY (lower(l.lemma) = sqlc.arg(exact)::text) DESC,
         (lower(l.lemma) LIKE sqlc.arg(prefix)::text ESCAPE '\') DESC,
         l.lemma, l.pos
LIMIT sqlc.arg(max_rows)::int;

-- name: ListLexemeLevels :many
-- Оценки найденных слов по источникам.
SELECT c.lexeme_id, s.name AS source, c.level, c.extra
FROM lexeme_cefr c
JOIN data_sources s ON s.id = c.source_id
WHERE c.lexeme_id = ANY(sqlc.arg(ids)::bigint[])
ORDER BY c.lexeme_id, s.name;
