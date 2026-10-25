-- +goose Up

-- Четвёртый вид упражнений: «сопоставление» (match): пять слов и пять переводов, предложения у него нет.
-- Существующие строки не меняются.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form', 'match'));
ALTER TABLE exercises ALTER COLUMN sentence_id DROP NOT NULL;

-- +goose Down
-- ВНИМАНИЕ: откат невозможен, пока есть упражнения вида match: сначала удалите их
-- (DELETE FROM exercises WHERE kind = 'match'). События exercise.answered в журнале events остаются.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM exercises WHERE kind = 'match') THEN
        RAISE EXCEPTION 'откат 00012 невозможен: есть упражнения вида match, удалите их';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE exercises ALTER COLUMN sentence_id SET NOT NULL;
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form'));
