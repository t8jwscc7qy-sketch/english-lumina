-- +goose Up

-- Предложения из внешних баз (Tatoeba). Автор и номер на tatoeba.org нужны для атрибуции по лицензии CC BY.
CREATE TABLE sentences (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lang       text        NOT NULL CHECK (lang IN ('en', 'ru')),
    tatoeba_id bigint      NOT NULL CHECK (tatoeba_id > 0), -- номер предложения на tatoeba.org
    text       text        NOT NULL CHECK (length(text) BETWEEN 1 AND 1000),
    author     text        NOT NULL CHECK (length(author) BETWEEN 1 AND 64),
    word_count smallint    NOT NULL CHECK (word_count > 0),
    -- Источник, впервые добавивший предложение.
    source_id  bigint      NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lang, tatoeba_id)
);
CREATE INDEX sentences_source ON sentences (source_id);

-- Перевод: английское предложение и его русский перевод. У одного английского бывает много переводов и наоборот.
-- Язык строк en_id и ru_id проверяет загрузчик.
CREATE TABLE sentence_pairs (
    en_id     bigint NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    ru_id     bigint NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    source_id bigint NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    PRIMARY KEY (en_id, ru_id)
);
CREATE INDEX sentence_pairs_ru ON sentence_pairs (ru_id);
CREATE INDEX sentence_pairs_source ON sentence_pairs (source_id);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет все загруженные предложения и пары; их можно загрузить заново командой import.
DROP TABLE sentence_pairs;
DROP TABLE sentences;
