-- +goose Up

-- Карточки слов с состоянием FSRS (интервальные повторения). Одна карточка на пользователя и слово.
-- Поля состояния повторяют карточку библиотеки go-fsrs/v4; каждое повторение дополнительно пишется
-- в журнал events, а эта таблица хранит только текущее состояние.
CREATE TABLE cards (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lexeme_id       bigint           NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT,
    due             timestamptz      NOT NULL,                       -- когда карточку пора повторить
    stability       double precision NOT NULL CHECK (stability >= 0),
    difficulty      double precision NOT NULL CHECK (difficulty >= 0),
    scheduled_days  integer          NOT NULL CHECK (scheduled_days >= 0),
    reps            integer          NOT NULL CHECK (reps >= 0),
    lapses          integer          NOT NULL CHECK (lapses >= 0),
    state           smallint         NOT NULL CHECK (state BETWEEN 0 AND 3), -- 0 новая, 1 обучение, 2 повторение, 3 переобучение
    last_review     timestamptz,                                     -- пусто, пока карточку не повторяли
    remaining_steps integer          NOT NULL CHECK (remaining_steps >= 0),
    created_at      timestamptz      NOT NULL DEFAULT now(),
    updated_at      timestamptz      NOT NULL DEFAULT now(),
    UNIQUE (user_id, lexeme_id)
);
-- Очередь: просроченные карточки пользователя по возрастанию срока.
CREATE INDEX cards_user_due ON cards (user_id, due);
-- Внешний ключ на слово (удаление слова проверяет карточки).
CREATE INDEX cards_lexeme ON cards (lexeme_id);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет прогресс повторений. События card.added / card.reviewed остаются в журнале events.
DROP TABLE cards;
