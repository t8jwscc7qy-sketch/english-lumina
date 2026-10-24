-- +goose Up

-- Третий вид упражнений: «выбор формы» (form). Существующие строки не меняются.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form'));

-- +goose Down
-- ВНИМАНИЕ: откат невозможен, пока есть упражнения вида form: сначала удалите их
-- (DELETE FROM exercises WHERE kind = 'form'). События exercise.answered в журнале events остаются.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM exercises WHERE kind = 'form') THEN
        RAISE EXCEPTION 'откат 00009 невозможен: есть упражнения вида form, удалите их';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order'));
