-- +goose Up

-- Поиск примеров для слова идёт по формам слова (to_tsquery('simple', ...)) среди английских предложений.
-- Индекс частичный: русские предложения в поиске не участвуют. Запрос должен использовать то же выражение
-- to_tsvector('simple', text) и условие lang = 'en', иначе индекс не подойдёт.
CREATE INDEX sentences_en_fts ON sentences USING gin (to_tsvector('simple', text)) WHERE lang = 'en';

-- +goose Down
-- Откат только убирает индекс, данные не трогает (поиск станет медленнее, но заработает полным перебором).
DROP INDEX sentences_en_fts;
