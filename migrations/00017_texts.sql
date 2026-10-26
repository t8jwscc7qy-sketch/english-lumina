-- +goose Up
-- Тексты для чтения и слушания и ответы «как понял». Собранный склейкой текст сохраняется, чтобы его можно было
-- перечитать и чтобы не повторять предложения в следующих текстах. Глобальные тексты (авторские, открытые) будут без user_id.
CREATE TABLE texts (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id           bigint      REFERENCES users (id) ON DELETE CASCADE,
    kind              text        NOT NULL CHECK (kind IN ('authored', 'stitched', 'open')),
    title             text        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    body              jsonb       NOT NULL,                   -- предложения с номерами Tatoeba, переводами и словами
    word_count        smallint    NOT NULL CHECK (word_count > 0),
    coverage_at_build real        NOT NULL CHECK (coverage_at_build BETWEEN 0 AND 1),
    goal              real        NOT NULL CHECK (goal BETWEEN 0 AND 1),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX texts_user_recent ON texts (user_id, id DESC);

-- Одна оценка на текст: повтор запроса ничего не двигает.
CREATE TABLE text_reads (
    user_id            bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text_id            bigint      NOT NULL REFERENCES texts (id) ON DELETE CASCADE,
    read_at            timestamptz NOT NULL DEFAULT now(),
    mode               text        NOT NULL CHECK (mode IN ('read', 'listen')),
    understood         text        NOT NULL CHECK (understood IN ('easy', 'ok', 'hard')),
    coverage_at_read   real        NOT NULL CHECK (coverage_at_read BETWEEN 0 AND 1),
    PRIMARY KEY (user_id, text_id)
);

-- Цель понимания текста (80–90 %; real хранит дроби неточно, поэтому границы чуть шире).
ALTER TABLE learn_preferences
    ADD COLUMN text_coverage real NOT NULL DEFAULT 0.85 CHECK (text_coverage BETWEEN 0.795 AND 0.905);

-- +goose Down
ALTER TABLE learn_preferences DROP COLUMN text_coverage;
DROP TABLE text_reads;
DROP TABLE texts;
