-- +goose Up

-- Пятый вид упражнений: «перевод» (translate): русское предложение переводят по-английски вводом.
-- Существующие строки не меняются. К настоящей базе применять только после pg_dump.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form', 'match', 'translate'));

-- +goose Down
-- ВНИМАНИЕ: откат невозможен, пока есть упражнения вида translate: сначала удалите их
-- (DELETE FROM exercises WHERE kind = 'translate'). События exercise.answered в журнале events остаются.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM exercises WHERE kind = 'translate') THEN
        RAISE EXCEPTION 'откат 00013 невозможен: есть упражнения вида translate, удалите их';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form', 'match'));
