-- name: UpsertDataSource :one
-- Источник той же версии переиспользуется: повторный импорт не создаёт вторую запись.
INSERT INTO data_sources (name, version, license, url, attribution)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (name, version) DO UPDATE SET
    license     = EXCLUDED.license,
    url         = EXCLUDED.url,
    attribution = EXCLUDED.attribution
RETURNING *;

-- name: FinishDataSource :exec
UPDATE data_sources SET row_count = $2, imported_at = now() WHERE id = $1;

-- name: GetDataSource :one
SELECT * FROM data_sources WHERE name = $1 AND version = $2;

-- name: UpsertLexeme :one
-- Существующее слово не меняется (source_id остаётся у первого источника); пустой UPDATE нужен, чтобы вернуть id.
INSERT INTO lexemes (lemma, pos, source_id)
VALUES ($1, $2, $3)
ON CONFLICT (lemma, pos) DO UPDATE SET lemma = lexemes.lemma
RETURNING id;

-- name: UpsertLexemeCefr :exec
INSERT INTO lexeme_cefr (lexeme_id, source_id, level, extra)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lexeme_id, source_id) DO UPDATE SET
    level = EXCLUDED.level,
    extra = EXCLUDED.extra;
