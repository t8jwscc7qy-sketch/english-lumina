-- +goose Up
-- Авторские тексты лежат в `texts` без владельца (user_id пуст) и находятся по slug; что показано пользователю, пишется в
-- text_shown (там же покрытие на момент показа), чтобы не отдавать прочитанное и повторно выдавать неоценённый текст.
ALTER TABLE texts ADD COLUMN slug text;
ALTER TABLE texts ADD COLUMN band smallint;
CREATE UNIQUE INDEX texts_slug ON texts (slug) WHERE slug IS NOT NULL;
ALTER TABLE texts ADD CONSTRAINT texts_authored_has_slug CHECK ((kind = 'authored') = (slug IS NOT NULL));

CREATE TABLE text_shown (
    user_id  bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text_id  bigint      NOT NULL REFERENCES texts (id) ON DELETE CASCADE,
    shown_at timestamptz NOT NULL DEFAULT now(),
    coverage real        NOT NULL CHECK (coverage BETWEEN 0 AND 1),
    PRIMARY KEY (user_id, text_id)
);
CREATE INDEX text_shown_recent ON text_shown (user_id, shown_at DESC);
CREATE INDEX text_shown_text ON text_shown (text_id);

-- Уже собранные (склеенные) тексты считаются показанными своему владельцу.
INSERT INTO text_shown (user_id, text_id, shown_at, coverage)
SELECT user_id, id, created_at, coverage_at_build FROM texts WHERE user_id IS NOT NULL;

-- +goose Down
DROP TABLE text_shown;
ALTER TABLE texts DROP CONSTRAINT texts_authored_has_slug;
DROP INDEX texts_slug;
ALTER TABLE texts DROP COLUMN band;
ALTER TABLE texts DROP COLUMN slug;
