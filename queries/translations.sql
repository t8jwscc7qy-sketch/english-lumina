-- name: UpsertWordTranslation :exec
INSERT INTO word_translations (source_id, lemma, pos, sense_no, ru, definition)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source_id, lemma, pos, sense_no, ru) DO UPDATE SET definition = EXCLUDED.definition;
