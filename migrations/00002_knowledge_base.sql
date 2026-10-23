-- +goose Up

-- Внешние базы знаний. Версия источника входит в его идентичность: новая версия — новая запись.
CREATE TABLE data_sources (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 32),
    version     text        NOT NULL CHECK (length(version) BETWEEN 1 AND 32),
    license     text        NOT NULL,
    url         text        NOT NULL,
    attribution text        NOT NULL, -- текст, который обязаны показать по лицензии
    row_count   integer     CHECK (row_count >= 0),
    imported_at timestamptz,          -- NULL, пока импорт не завершился
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name, version)
);

-- Слово с частью речи. source_id — источник, впервые добавивший слово; его уровни и частоты лежат в своих таблицах.
CREATE TABLE lexemes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lemma      text        NOT NULL CHECK (length(lemma) BETWEEN 1 AND 64), -- как в источнике, без разбора вариантов
    pos        text        NOT NULL CHECK (length(pos) BETWEEN 1 AND 32),
    source_id  bigint      NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lemma, pos)
);
CREATE INDEX lexemes_source ON lexemes (source_id);

-- Уровень CEFR слова по данным источника. Прочие колонки источника лежат в extra.
CREATE TABLE lexeme_cefr (
    lexeme_id bigint NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    source_id bigint NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    level     text   NOT NULL CHECK (level IN ('A1','A2','B1','B2','C1','C2')),
    extra     jsonb  NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(extra) = 'object'),
    PRIMARY KEY (lexeme_id, source_id)
);
CREATE INDEX lexeme_cefr_source ON lexeme_cefr (source_id);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет все импортированные слова и уровни; их можно загрузить заново командой import.
DROP TABLE lexeme_cefr;
DROP TABLE lexemes;
DROP TABLE data_sources;
