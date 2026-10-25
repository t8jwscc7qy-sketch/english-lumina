-- +goose Up

-- Пятый вид упражнений: «перевод» (translate): русское предложение переводят по-английски вводом.
-- У банка ошибок своя проверка вида, её тоже расширяем. Существующие строки не меняются.
-- К настоящей базе применять только после pg_dump.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form', 'match', 'translate'));
ALTER TABLE mistakes DROP CONSTRAINT mistakes_kind_check;
ALTER TABLE mistakes ADD CONSTRAINT mistakes_kind_check CHECK (kind IN ('gap', 'order', 'form', 'translate'));

-- +goose Down
-- ВНИМАНИЕ: откат невозможен, пока есть упражнения или ошибки вида translate: сначала удалите их
-- (DELETE FROM exercises WHERE kind = 'translate'; DELETE FROM mistakes WHERE kind = 'translate').
-- События в журнале events остаются.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM exercises WHERE kind = 'translate') OR EXISTS (SELECT 1 FROM mistakes WHERE kind = 'translate') THEN
        RAISE EXCEPTION 'откат 00013 невозможен: есть упражнения или ошибки вида translate, удалите их';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE mistakes DROP CONSTRAINT mistakes_kind_check;
ALTER TABLE mistakes ADD CONSTRAINT mistakes_kind_check CHECK (kind IN ('gap', 'order', 'form'));
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('gap', 'order', 'form', 'match'));
