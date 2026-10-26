-- +goose Up

-- База знаний kb_*: производные данные из внешних источников (переводы, произношение, формы, синонимы).
-- Собирается командой kb-build и пересобирается целиком; данные пользователя на эти таблицы не ссылаются
-- (только на lexemes.id), поэтому их можно удалить и собрать заново. Резервная копия: pg_dump -t 'kb_*'.
CREATE TABLE kb_build (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    started_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    status         text        NOT NULL CHECK (status IN ('running', 'ok', 'failed')),
    schema_version integer     NOT NULL DEFAULT 1,
    sources        jsonb       NOT NULL DEFAULT '{}', -- источники, их версии и суммы
    report         jsonb       NOT NULL DEFAULT '{}'  -- показатели отчёта качества
);

CREATE TABLE kb_sense (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lexeme_id bigint  NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    sense_no  integer NOT NULL,
    gloss_en  text    NOT NULL,
    tags      text[]  NOT NULL DEFAULT '{}',
    flags     text[]  NOT NULL DEFAULT '{}', -- archaic, obsolete, dated, vulgar, offensive, slang, rare, informal
    source_id bigint  NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    UNIQUE (lexeme_id, sense_no)
);

CREATE TABLE kb_translation (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lexeme_id  bigint  NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    sense_no   integer,                         -- номер значения (kb_sense.sense_no); пусто, если значение неизвестно
    lang       text    NOT NULL CHECK (lang <> ''),
    text       text    NOT NULL CHECK (text <> ''),
    score      real    NOT NULL,
    rank       integer NOT NULL CHECK (rank > 0),
    chosen     boolean NOT NULL DEFAULT false, -- до двух выбранных на слово и язык; у выбранных rank 1 или 2, у остальных дальше по оценке
    flags      text[]  NOT NULL DEFAULT '{}',
    source_ids bigint[] NOT NULL DEFAULT '{}',
    CHECK (NOT chosen OR rank BETWEEN 1 AND 2),
    FOREIGN KEY (lexeme_id, sense_no) REFERENCES kb_sense (lexeme_id, sense_no) ON DELETE CASCADE
);
CREATE UNIQUE INDEX kb_translation_chosen ON kb_translation (lexeme_id, lang, rank) WHERE chosen;
CREATE INDEX kb_translation_lexeme ON kb_translation (lexeme_id) WHERE chosen;
-- Под составной внешний ключ на kb_sense (каскадное удаление значений без полного просмотра).
CREATE INDEX kb_translation_sense ON kb_translation (lexeme_id, sense_no);

CREATE TABLE kb_pronunciation (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lexeme_id  bigint  NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    accent     text    NOT NULL CHECK (accent IN ('us', 'uk', '')),
    ipa        text    NOT NULL,
    audio_ref  text    NOT NULL DEFAULT '', -- имя файла Wikimedia Commons или пусто
    is_primary boolean NOT NULL DEFAULT false,
    source_id  bigint  NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT
);
CREATE INDEX kb_pronunciation_lexeme ON kb_pronunciation (lexeme_id);

CREATE TABLE kb_form (
    form      text   NOT NULL,
    lexeme_id bigint NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    tag       text   NOT NULL DEFAULT '',
    PRIMARY KEY (form, lexeme_id, tag)
);
CREATE INDEX kb_form_lexeme ON kb_form (lexeme_id);

CREATE TABLE kb_relation (
    lexeme_id        bigint NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    kind             text   NOT NULL CHECK (kind IN ('synonym', 'antonym')),
    other_lemma      text   NOT NULL,
    other_lexeme_id  bigint REFERENCES lexemes (id) ON DELETE SET NULL,
    PRIMARY KEY (lexeme_id, kind, other_lemma)
);
CREATE INDEX kb_relation_other ON kb_relation (other_lexeme_id) WHERE other_lexeme_id IS NOT NULL;

CREATE TABLE kb_lexeme_stat (
    lexeme_id  bigint PRIMARY KEY REFERENCES lexemes (id) ON DELETE CASCADE,
    freq_count bigint  NOT NULL DEFAULT 0, -- сумма по формам слова
    freq_rank  integer                      -- место по сумме (1 — самое частое); пусто, если слова нет в списке частот
);

-- Готовая «карточка слова для урока»: читается одним запросом по ключу.
CREATE TABLE kb_teaching_card (
    lexeme_id bigint PRIMARY KEY REFERENCES lexemes (id) ON DELETE CASCADE,
    payload   jsonb       NOT NULL,
    built_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Таблицы kb_* производные: удаляются целиком и пересобираются командой kb-build.
DROP TABLE kb_teaching_card;
DROP TABLE kb_lexeme_stat;
DROP TABLE kb_relation;
DROP TABLE kb_form;
DROP TABLE kb_pronunciation;
DROP TABLE kb_translation;
DROP TABLE kb_sense;
DROP TABLE kb_build;
