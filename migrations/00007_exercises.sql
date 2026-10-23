-- +goose Up

-- Упражнения создаются при запросе и сохраняются вместе с верным ответом (клиент ответ не видит до проверки).
CREATE TABLE exercises (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind        text        NOT NULL CHECK (kind IN ('gap')),
    sentence_id bigint      NOT NULL REFERENCES sentences (id) ON DELETE RESTRICT, -- английское предложение
    topics      text[]      NOT NULL CHECK (cardinality(topics) > 0),             -- ключи тем, по которым считается результат
    payload     jsonb       NOT NULL,                                               -- что видит клиент
    answer      jsonb       NOT NULL,                                               -- допустимые ответы
    created_at  timestamptz NOT NULL DEFAULT now(),
    answered_at timestamptz                                                         -- пусто, пока не отвечено
);
-- Какие упражнения пользователя отвечены или ждут ответа.
CREATE INDEX exercises_user_answered ON exercises (user_id, answered_at);
-- Внешний ключ на предложение (удаление предложения проверяет упражнения).
CREATE INDEX exercises_sentence ON exercises (sentence_id);

-- Рейтинг пользователя по теме (Glicko-2: рейтинг, отклонение, изменчивость).
CREATE TABLE topic_ratings (
    user_id    bigint           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    topic      text             NOT NULL CHECK (length(topic) BETWEEN 1 AND 64),
    rating     double precision NOT NULL DEFAULT 1500,
    deviation  double precision NOT NULL DEFAULT 350 CHECK (deviation > 0),
    volatility double precision NOT NULL DEFAULT 0.06 CHECK (volatility > 0),
    attempts   integer          NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    updated_at timestamptz      NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, topic)
);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет упражнения и рейтинги тем. События exercise.answered остаются в журнале events.
DROP TABLE topic_ratings;
DROP TABLE exercises;
