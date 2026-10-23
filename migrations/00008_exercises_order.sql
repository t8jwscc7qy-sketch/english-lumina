-- +goose Up

-- Второй вид упражнений: «порядок слов» (order). Существующие строки не меняются.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order'));

-- +goose Down
-- ВНИМАНИЕ: откат невозможен, пока есть упражнения вида order: сначала удалите их
-- (DELETE FROM exercises WHERE kind = 'order'). События exercise.answered в журнале events остаются.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM exercises WHERE kind = 'order') THEN
        RAISE EXCEPTION 'откат 00008 невозможен: есть упражнения вида order, удалите их';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap'));
