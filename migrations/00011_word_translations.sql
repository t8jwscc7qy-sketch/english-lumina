-- +goose Up

-- Русские переводы английских слов (словарь FreeDict/WikDict). Слово словаря задаётся парой «начальная форма + часть речи»,
-- как у упражнения «выбор формы»; значения слова — это sense_no (по порядку в словаре, считаются по лемме и части речи).
-- ru хранится без знаков ударения. definition — английское пояснение значения (может быть пустым).
CREATE TABLE word_translations (
    source_id  bigint   NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    lemma      text     NOT NULL CHECK (length(lemma) BETWEEN 1 AND 64),
    pos        text     NOT NULL CHECK (pos IN ('n', 'v', 'adj', 'adv')),
    sense_no   smallint NOT NULL CHECK (sense_no > 0),
    ru         text     NOT NULL CHECK (length(ru) BETWEEN 1 AND 64),
    definition text     CHECK (definition IS NULL OR length(definition) <= 400),
    PRIMARY KEY (source_id, lemma, pos, sense_no, ru)
);

-- Поиск переводов слова: по лемме и части речи, из последнего завершённого импорта.
CREATE INDEX word_translations_lookup ON word_translations (lemma, pos);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет переводы; их можно загрузить заново командой import freedict-eng-rus.
DROP TABLE word_translations;
