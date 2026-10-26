-- +goose Up

-- Состояние изучения слов: слово введено на «Знакомстве» (introduced), вспомнено хорошо (learned) или отмечено
-- «знаю» в быстром тесте (known_at_start). «Привал» берёт задания только из изученного.
CREATE TABLE word_learning (
    user_id       bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lexeme_id     bigint      NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT,
    status        text        NOT NULL CHECK (status IN ('introduced', 'learned', 'known_at_start')),
    source        text        NOT NULL CHECK (source IN ('lesson', 'placement', 'legacy_card', 'card')),
    introduced_at timestamptz,
    learned_at    timestamptz,
    PRIMARY KEY (user_id, lexeme_id),
    CHECK (status <> 'learned' OR learned_at IS NOT NULL)
);
CREATE INDEX word_learning_user_status ON word_learning (user_id, status);

-- Настройки обучения отдельно от настроек цели и уровня: режим новых слов, число в день, покрытие «Привала».
CREATE TABLE learn_preferences (
    user_id           bigint PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    new_words_mode    text        NOT NULL DEFAULT 'auto' CHECK (new_words_mode IN ('auto', 'manual')),
    new_words_per_day integer     NOT NULL DEFAULT 12 CHECK (new_words_per_day BETWEEN 1 AND 30),
    camp_coverage     real        NOT NULL DEFAULT 0.95 CHECK (camp_coverage BETWEEN 0.90 AND 0.99),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- Перенос: карточки в состоянии «повторение» (state = 2) уже вспоминаются стабильно: слово изучено.
INSERT INTO word_learning (user_id, lexeme_id, status, source, introduced_at, learned_at)
SELECT user_id, lexeme_id, 'learned', 'legacy_card', created_at, COALESCE(last_review, created_at)
FROM cards
WHERE state = 2
ON CONFLICT DO NOTHING;

-- +goose Down
-- Откат удаляет состояние изучения и настройки обучения; карточки остаются.
DROP TABLE learn_preferences;
DROP TABLE word_learning;
