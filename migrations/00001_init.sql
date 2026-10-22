-- +goose Up

-- Единственный пользователь сервиса.
CREATE TABLE users (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login               text        NOT NULL UNIQUE CHECK (login ~ '^[a-z0-9_.-]{3,32}$'),
    password_hash       text        NOT NULL CHECK (password_hash LIKE '$argon2id$%'), -- PHC-строка
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
-- Не больше одной строки: индекс по константному выражению. Вторая вставка -> 23505.
CREATE UNIQUE INDEX users_single_row ON users ((true));

-- Сессии входа. В БД только SHA-256 от токена из cookie.
CREATE TABLE sessions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    user_agent   text CHECK (length(user_agent) <= 512),
    CHECK (expires_at > created_at)
);
-- «Выйти везде» / список активных сессий.
CREATE INDEX sessions_user_active ON sessions (user_id) WHERE revoked_at IS NULL;

-- Настройки обучения: одна строка на пользователя. Нет строки = онбординг не пройден.
CREATE TABLE settings (
    user_id           bigint PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    current_level     text    NOT NULL CHECK (current_level IN ('A1','A2','B1','B2','C1')),
    target_level      text    NOT NULL CHECK (target_level  IN ('A1','A2','B1','B2','C1')),
    target_date       date,                       -- календарный день, не момент времени
    daily_min_minutes integer NOT NULL DEFAULT 15 CHECK (daily_min_minutes BETWEEN 5 AND 240),
    timezone          text    NOT NULL DEFAULT 'UTC' CHECK (timezone <> ''), -- IANA, проверяется в Go
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    -- Цель не ниже текущего уровня; коды упорядочены лексикографически.
    CHECK (target_level COLLATE "C" >= current_level COLLATE "C")
);

-- Журнал событий: только добавление. Источник для серий, вех, аналитики, пересчёта Glicko/FSRS.
CREATE TABLE events (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    type            text        NOT NULL
                    CHECK (length(type) <= 64 AND type ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$'),
    schema_version  integer     NOT NULL CHECK (schema_version >= 1),
    occurred_at     timestamptz NOT NULL,                 -- когда произошло
    recorded_at     timestamptz NOT NULL DEFAULT now(),   -- когда записано
    payload         jsonb       NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    idempotency_key text CHECK (length(idempotency_key) BETWEEN 1 AND 128)
);
-- Повтор того же ответа (двойной клик, ретрай) не создаст второе событие.
CREATE UNIQUE INDEX events_idempotency ON events (user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
-- Серии и аналитика по времени; фильтр по типу события.
CREATE INDEX events_user_occurred      ON events (user_id, occurred_at);
CREATE INDEX events_user_type_occurred ON events (user_id, type, occurred_at);

-- +goose StatementBegin
CREATE FUNCTION events_forbid_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'events только для добавления: % запрещён', TG_OP;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER events_no_update_delete
    BEFORE UPDATE OR DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_forbid_change();
CREATE TRIGGER events_no_truncate
    BEFORE TRUNCATE ON events
    FOR EACH STATEMENT EXECUTE FUNCTION events_forbid_change();

-- +goose Down
-- ВНИМАНИЕ: откат удаляет ВСЕ данные (пользователя, сессии, настройки, журнал).
-- DROP TABLE не вызывает строковые триггеры, поэтому откат проходит.
DROP TABLE events;
DROP FUNCTION events_forbid_change();
DROP TABLE settings;
DROP TABLE sessions;
DROP TABLE users;
