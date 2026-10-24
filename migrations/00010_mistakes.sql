-- +goose Up

-- Банк ошибок: неверный ответ на упражнение становится ошибкой, которая возвращается по расписанию
-- (через 1 день, затем через 3) и закрывается после двух верных повторов подряд.
CREATE TABLE mistakes (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id            bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind               text        NOT NULL CHECK (kind IN ('gap', 'order', 'form')),
    sentence_id        bigint      NOT NULL REFERENCES sentences (id) ON DELETE RESTRICT,
    topics             text[]      NOT NULL CHECK (cardinality(topics) > 0),
    payload            jsonb       NOT NULL,                                  -- копия из исходного упражнения
    answer             jsonb       NOT NULL,                                  -- копия допустимых ответов
    given              text        NOT NULL,                                  -- последний неверный ответ
    streak             smallint    NOT NULL DEFAULT 0 CHECK (streak BETWEEN 0 AND 2),
    status             text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    next_due           timestamptz NOT NULL,
    first_wrong_at     timestamptz NOT NULL,
    closed_at          timestamptz,                                           -- пусто, пока ошибка открыта
    source_exercise_id bigint,                                                -- справочно, без внешнего ключа
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mistakes_closed_at_matches_status CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);
-- Не больше одной открытой ошибки на одно предложение и вид упражнения.
CREATE UNIQUE INDEX mistakes_open_unique ON mistakes (user_id, sentence_id, kind) WHERE status = 'open';
-- Выборка повторов: открытые с наступившим сроком, самые старые первыми.
CREATE INDEX mistakes_user_status_due ON mistakes (user_id, status, next_due);
-- Внешний ключ на предложение (удаление предложения проверяет ошибки).
CREATE INDEX mistakes_sentence ON mistakes (sentence_id);

-- Повтор ошибки — новая строка exercises с этой ссылкой.
ALTER TABLE exercises ADD COLUMN mistake_id bigint REFERENCES mistakes (id) ON DELETE SET NULL;
CREATE INDEX exercises_mistake ON exercises (mistake_id) WHERE mistake_id IS NOT NULL;

-- +goose Down
-- ВНИМАНИЕ: откат удаляет банк ошибок целиком (данные теряются); у упражнений пропадает связь с ошибкой.
-- События mistake.opened и mistake.closed в журнале events остаются.
DROP INDEX exercises_mistake;
ALTER TABLE exercises DROP COLUMN mistake_id;
DROP TABLE mistakes;
