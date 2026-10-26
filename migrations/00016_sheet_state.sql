-- +goose Up
-- Состояние шпаргалок пользователя: избранное, отметка «проверено» им самим, когда открывал последний раз.
-- Сами шпаргалки в базе не хранятся (они встроены в сервер), поэтому правка текста миграций не требует.
CREATE TABLE sheet_state (
    user_id     bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sheet_id    text        NOT NULL CHECK (length(sheet_id) BETWEEN 1 AND 64),
    favorite_at timestamptz,
    reviewed_at timestamptz,
    reviewed_hash text,   -- хеш текста шпаргалки на момент проверки: правка текста сбрасывает «проверено»
    opened_at   timestamptz,
    PRIMARY KEY (user_id, sheet_id)
);

-- +goose Down
DROP TABLE sheet_state;
